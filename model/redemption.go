package model

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/utils"
)

type Redemption struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id"`
	Key          string `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int    `json:"status" gorm:"default:1"`
	Name         string `json:"name" gorm:"index"`
	Quota        int    `json:"quota" gorm:"default:100"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	RedeemedBy   int    `json:"redeemed_by" gorm:"index;default:0"`
	RedeemedTime int64  `json:"redeemed_time" gorm:"bigint"`
	Count        int    `json:"count" gorm:"-:all"` // only for api request
}

var allowedRedemptionslOrderFields = map[string]bool{
	"id":            true,
	"name":          true,
	"status":        true,
	"quota":         true,
	"created_time":  true,
	"redeemed_time": true,
}

func GetRedemptionsList(params *GenericParams) (*DataResult[Redemption], error) {
	var redemptions []*Redemption
	db := DB
	if params.Keyword != "" {
		db = db.Where("id = ? or name LIKE ?", utils.String2Int(params.Keyword), params.Keyword+"%")
	}

	return PaginateAndOrder[Redemption](db, &params.PaginationParams, &redemptions, allowedRedemptionslOrderFields)
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int, ip string) (quota int, err error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId <= 0 {
		return 0, errors.New("无效的 user id")
	}
	var redemption Redemption
	changedGroup := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		// The conditional write is the cross-database lock and single-use claim.
		result := tx.Model(&Redemption{}).Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: key}).Where("status = ? AND quota > 0 AND (redeemed_time = 0 OR redeemed_time IS NULL) AND (redeemed_by = 0 OR redeemed_by IS NULL)", config.RedemptionCodeStatusEnabled).Updates(map[string]any{"status": config.RedemptionCodeStatusUsed, "redeemed_time": utils.GetTimestamp(), "redeemed_by": userId})
		if err := quotaWriteResult(result, "无效或已使用的兑换码"); err != nil {
			return err
		}
		if err := tx.Where(clause.Eq{Column: clause.Column{Name: "key"}, Value: key}).First(&redemption).Error; err != nil {
			return err
		}
		var creditErr error
		changedGroup, creditErr = creditRechargeQuotaTx(tx, userId, redemption.Quota, ip, fmt.Sprintf("通过兑换码充值 %s", common.LogQuota(redemption.Quota)))
		return creditErr
	})
	if err != nil {
		return 0, errors.New("兑换失败，" + err.Error())
	}
	if changedGroup {
		invalidateRechargeGroupCache(userId)
	}
	return redemption.Quota, nil
}

func (redemption *Redemption) Insert() error {
	if redemption.Quota <= 0 {
		return errors.New("兑换额度必须大于零")
	}
	if redemption.Status != 0 && redemption.Status != config.RedemptionCodeStatusEnabled && redemption.Status != config.RedemptionCodeStatusDisabled {
		return errors.New("无效的兑换码状态")
	}
	redemption.RedeemedTime, redemption.RedeemedBy = 0, 0
	return DB.Create(redemption).Error
}

func (redemption *Redemption) SelectUpdate() error { return redemption.updateUnused(true) }

func (redemption *Redemption) Update() error { return redemption.updateUnused(false) }

func (redemption *Redemption) updateUnused(statusOnly bool) error {
	if redemption.Id <= 0 || (redemption.Status != config.RedemptionCodeStatusEnabled && redemption.Status != config.RedemptionCodeStatusDisabled) {
		return errors.New("无效的兑换码状态")
	}
	if !statusOnly && redemption.Quota <= 0 {
		return errors.New("兑换额度必须大于零")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Redemption{}).Where("id = ?", redemption.Id).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
			return err
		}
		var stored Redemption
		if err := tx.First(&stored, redemption.Id).Error; err != nil {
			return err
		}
		if stored.Status == config.RedemptionCodeStatusUsed || stored.RedeemedTime != 0 || stored.RedeemedBy != 0 {
			return errors.New("已使用的兑换码不可修改")
		}
		fields := map[string]any{"status": redemption.Status}
		if !statusOnly {
			fields["name"], fields["quota"] = redemption.Name, redemption.Quota
		}
		return tx.Model(&stored).Updates(fields).Error
	})
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

type RedemptionStatistics struct {
	Count  int64 `json:"count"`
	Quota  int64 `json:"quota"`
	Status int   `json:"status"`
}

func GetStatisticsRedemption() (redemptionStatistics []*RedemptionStatistics, err error) {
	err = DB.Model(&Redemption{}).Select("status", "count(*) as count", "sum(quota) as quota").Where("status != ?", 2).Group("status").Scan(&redemptionStatistics).Error
	return redemptionStatistics, err
}

type RedemptionStatisticsGroup struct {
	Date      string `json:"date"`
	Quota     int64  `json:"quota"`
	UserCount int64  `json:"user_count"`
}

func GetStatisticsRedemptionByPeriod(startTimestamp, endTimestamp int64) (redemptionStatistics []*RedemptionStatisticsGroup, err error) {
	groupSelect := getTimestampGroupsSelect("redeemed_time", "day", "date")

	err = DB.Raw(`
		SELECT `+groupSelect+`,
		sum(quota) as quota,
		count(distinct user_id) as user_count
		FROM redemptions
		WHERE status=3
		AND redeemed_time BETWEEN ? AND ?
		GROUP BY date
		ORDER BY date
	`, startTimestamp, endTimestamp).Scan(&redemptionStatistics).Error

	return redemptionStatistics, err
}
