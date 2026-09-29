package model

import (
	"errors"
	"fmt"
	"math"

	"gorm.io/gorm"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/common/redis"
	"one-api/common/utils"
)

// Used only inside the durable source transaction (redemption/order). Balance,
// promotion and enabled top-up audit log succeed together, bypassing batch RAM.
func creditRechargeQuotaTx(tx *gorm.DB, userID, quota int, ip, content string) (bool, error) {
	if userID <= 0 || quota <= 0 {
		return false, errors.New("invalid recharge quota")
	}
	maxInt := int(^uint(0) >> 1)
	result := tx.Model(&User{}).Where("id = ? AND quota <= ?", userID, maxInt-quota).Update("quota", gorm.Expr("quota + ?", quota))
	if err := quotaWriteResult(result, "recharge user is missing or quota would overflow"); err != nil {
		return false, err
	}
	changed, err := checkAndUpgradeUserGroupTx(tx, userID, 0)
	if err != nil {
		return false, err
	}
	var user User
	if err := tx.First(&user, userID).Error; err != nil {
		return false, err
	}
	log := Log{UserId: userID, Username: user.Username, Quota: quota, CreatedAt: utils.GetTimestamp(), Type: LogTypeTopup, SourceIp: ip, Content: content}
	return changed, tx.Create(&log).Error
}

// extraQuota is retained for the legacy caller; transactional recharge has
// already credited the balance and therefore always passes zero.
func checkAndUpgradeUserGroupTx(tx *gorm.DB, userID, extraQuota int) (bool, error) {
	var user User
	if err := tx.First(&user, userID).Error; err != nil {
		return false, err
	}
	maxInt := int(^uint(0) >> 1)
	minInt := -maxInt - 1
	total := user.Quota
	for _, value := range []int{user.UsedQuota, extraQuota} {
		if value > 0 && total > maxInt-value || value < 0 && total < minInt-value {
			return false, errors.New("cumulative recharge would overflow")
		}
		total += value
	}
	if config.QuotaPerUnit <= 0 || math.IsNaN(config.QuotaPerUnit) || math.IsInf(config.QuotaPerUnit, 0) {
		return false, errors.New("invalid recharge unit")
	}
	var groups []*UserGroup
	if err := tx.Where("promotion = ? AND enable = ?", true, true).Find(&groups).Error; err != nil {
		return false, err
	}
	var target *UserGroup
	for _, group := range groups {
		minQuota, maxQuota := float64(group.Min)*config.QuotaPerUnit, float64(group.Max)*config.QuotaPerUnit
		if float64(total) >= minQuota && (group.Max == 0 || float64(total) < maxQuota) && (target == nil || group.Min > target.Min) {
			target = group
		}
	}
	if target == nil || target.Symbol == user.Group {
		return false, nil
	}
	return true, tx.Model(&user).Update("group", target.Symbol).Error
}

func invalidateRechargeGroupCache(userID int) {
	if config.RedisEnabled {
		if err := redis.RedisDel(fmt.Sprintf(UserGroupCacheKey, userID)); err != nil {
			logger.SysError("failed to invalidate recharged user group cache")
		}
	}
}
