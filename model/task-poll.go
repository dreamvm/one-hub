package model

import (
	"errors"

	"gorm.io/gorm"
)

// Poll updates only mutable provider fields. The local identity and original
// receipt are re-read under a row lock; stale polls cannot reopen a terminal row.
func (task *Task) UpdateFromPoll(failed bool) error {
	if task.ID <= 0 {
		return errors.New("invalid task identity")
	}
	requested := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Even an unchanged UPDATE locks the row on MySQL; do not use RowsAffected
		// to infer existence because MySQL reports only changed rows by default.
		if err := tx.Model(&Task{}).Where("id = ?", task.ID).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; err != nil {
			return err
		}
		var stored Task
		if err := tx.First(&stored, task.ID).Error; err != nil {
			return err
		}
		if stored.UserId != task.UserId || stored.TokenID != task.TokenID || stored.ChannelId != task.ChannelId || stored.TaskID != task.TaskID || stored.Platform != task.Platform || stored.ReservationID != task.ReservationID {
			return errors.New("task identity changed")
		}
		if stored.Progress == 100 {
			return nil
		}
		fields := map[string]any{"status": task.Status, "fail_reason": task.FailReason, "progress": task.Progress, "submit_time": task.SubmitTime, "start_time": task.StartTime, "finish_time": task.FinishTime, "data": task.Data}
		if failed {
			status, err := requestTaskCompensation(tx, stored.ReservationID, stored.UserId, stored.TokenID, stored.ChannelId)
			if err != nil {
				return err
			}
			fields["refund_status"], fields["progress"], fields["status"] = status, 100, TaskStatusFailure
			requested = status == TaskRefundRequested
		}
		return tx.Model(&stored).Updates(fields).Error
	})
	if err != nil {
		return err
	}
	if requested {
		return FinalizeQuotaCompensation(task.ReservationID)
	}
	return nil
}

func (task *Midjourney) UpdateFromPoll(failed bool) error {
	if task.Id <= 0 {
		return errors.New("invalid midjourney identity")
	}
	requested := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Midjourney{}).Where("id = ?", task.Id).UpdateColumn("code", gorm.Expr("code")).Error; err != nil {
			return err
		}
		var stored Midjourney
		if err := tx.First(&stored, task.Id).Error; err != nil {
			return err
		}
		if stored.UserId != task.UserId || stored.TokenID != task.TokenID || stored.ChannelId != task.ChannelId || stored.MjId != task.MjId || stored.ReservationID != task.ReservationID {
			return errors.New("midjourney identity changed")
		}
		if stored.Progress == "100%" {
			return nil
		}
		fields := map[string]any{"code": task.Code, "progress": task.Progress, "prompt_en": task.PromptEn, "state": task.State, "submit_time": task.SubmitTime, "start_time": task.StartTime, "finish_time": task.FinishTime, "image_url": task.ImageUrl, "status": task.Status, "fail_reason": task.FailReason, "properties": task.Properties, "buttons": task.Buttons}
		if failed {
			status, err := requestTaskCompensation(tx, stored.ReservationID, stored.UserId, stored.TokenID, stored.ChannelId)
			if err != nil {
				return err
			}
			fields["refund_status"], fields["progress"], fields["status"] = status, "100%", "FAILURE"
			requested = status == TaskRefundRequested
		}
		return tx.Model(&stored).Updates(fields).Error
	})
	if err != nil {
		return err
	}
	if requested {
		return FinalizeQuotaCompensation(task.ReservationID)
	}
	return nil
}
