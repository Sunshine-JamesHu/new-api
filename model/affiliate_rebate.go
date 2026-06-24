package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AffiliateRebateStatusPending = "pending"
	AffiliateRebateStatusSettled = "settled"
)

type AffiliateRebate struct {
	Id              int     `json:"id"`
	TopUpId         int     `json:"topup_id" gorm:"uniqueIndex"`
	TradeNo         string  `json:"trade_no" gorm:"type:varchar(255);index"`
	InviterId       int     `json:"inviter_id" gorm:"index"`
	InviteeId       int     `json:"invitee_id" gorm:"index"`
	TopUpQuota      int     `json:"topup_quota" gorm:"default:0"`
	RemainingQuota  int     `json:"remaining_quota" gorm:"default:0;index"`
	RebateQuota     int     `json:"rebate_quota" gorm:"default:0"`
	RatePercent     float64 `json:"rate_percent"`
	PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	Status          string  `json:"status" gorm:"type:varchar(20);default:'pending';index"`
	CreatedAt       int64   `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	SettledAt       int64   `json:"settled_at" gorm:"default:0;column:settled_at"`
}

func GetUserAffiliateRebates(inviterId int, pageInfo *common.PageInfo) (rebates []*AffiliateRebate, total int64, err error) {
	if inviterId == 0 {
		return nil, 0, errors.New("invalid inviter id")
	}
	tx := DB.Model(&AffiliateRebate{}).Where("inviter_id = ?", inviterId)
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&rebates).Error
	return rebates, total, err
}

func SumUserPendingAffiliateRebateQuota(inviterId int) (int64, error) {
	if inviterId == 0 {
		return 0, nil
	}
	var total int64
	err := DB.Model(&AffiliateRebate{}).
		Where("inviter_id = ? AND status = ?", inviterId, AffiliateRebateStatusPending).
		Select("COALESCE(SUM(rebate_quota), 0)").
		Scan(&total).Error
	return total, err
}

func CreatePendingAffiliateRebateForTopUp(tx *gorm.DB, topUp *TopUp, topUpQuota int) error {
	if !operation_setting.IsAffiliateRebateEnabled() || topUp == nil || topUpQuota <= 0 {
		return nil
	}
	rate := operation_setting.GetAffiliateRebateRate()
	rebateDecimal := decimal.NewFromInt(int64(topUpQuota)).
		Mul(decimal.NewFromFloat(rate)).
		Div(decimal.NewFromInt(100))
	rebateQuota := int(math.Floor(rebateDecimal.InexactFloat64()))
	if rebateQuota <= 0 {
		return nil
	}
	return createAffiliateRebateForTopUp(tx, topUp, topUpQuota, rebateQuota, rate)
}

func createAffiliateRebateForTopUp(tx *gorm.DB, topUp *TopUp, topUpQuota int, rebateQuota int, ratePercent float64) error {
	if tx == nil {
		tx = DB
	}
	if topUp == nil || topUp.Id == 0 {
		return errors.New("topup is required")
	}
	if topUp.UserId == 0 || topUpQuota <= 0 || rebateQuota <= 0 {
		return nil
	}

	var invitee User
	if err := tx.Select("id", "quota", "inviter_id").Where("id = ?", topUp.UserId).First(&invitee).Error; err != nil {
		return err
	}
	if invitee.InviterId == 0 || invitee.InviterId == invitee.Id {
		return nil
	}
	remainingQuota := invitee.Quota
	if remainingQuota < topUpQuota {
		remainingQuota = topUpQuota
	}

	rebate := AffiliateRebate{
		TopUpId:         topUp.Id,
		TradeNo:         topUp.TradeNo,
		InviterId:       invitee.InviterId,
		InviteeId:       invitee.Id,
		TopUpQuota:      topUpQuota,
		RemainingQuota:  remainingQuota,
		RebateQuota:     rebateQuota,
		RatePercent:     ratePercent,
		PaymentProvider: topUp.PaymentProvider,
		Status:          AffiliateRebateStatusPending,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "top_up_id"}},
		DoNothing: true,
	}).Create(&rebate).Error
}

func ConsumeAffiliateRebateMaturity(userId int, quota int) {
	if userId == 0 || quota <= 0 {
		return
	}
	if err := consumeAffiliateRebateMaturity(DB, userId, quota); err != nil {
		common.SysLog(fmt.Sprintf("failed to mature affiliate rebates: user_id=%d quota=%d err=%v", userId, quota, err))
	}
}

func consumeAffiliateRebateMaturity(db *gorm.DB, userId int, quota int) error {
	if db == nil {
		db = DB
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var rebates []AffiliateRebate
		if err := tx.Where("invitee_id = ? AND status = ? AND remaining_quota > ?", userId, AffiliateRebateStatusPending, 0).
			Order("id asc").
			Find(&rebates).Error; err != nil {
			return err
		}
		for _, rebate := range rebates {
			nextRemaining := rebate.RemainingQuota - quota
			if nextRemaining < 0 {
				nextRemaining = 0
			}
			updates := map[string]interface{}{"remaining_quota": nextRemaining}
			if nextRemaining == 0 {
				updates["status"] = AffiliateRebateStatusSettled
				updates["settled_at"] = common.GetTimestamp()
			}
			result := tx.Model(&AffiliateRebate{}).Where("id = ? AND status = ?", rebate.Id, AffiliateRebateStatusPending).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			if nextRemaining == 0 {
				if err := tx.Model(&User{}).Where("id = ?", rebate.InviterId).Updates(map[string]interface{}{
					"aff_quota":   gorm.Expr("aff_quota + ?", rebate.RebateQuota),
					"aff_history": gorm.Expr("aff_history + ?", rebate.RebateQuota),
				}).Error; err != nil {
					return err
				}
				var inviter User
				if err := tx.Select("username").Where("id = ?", rebate.InviterId).First(&inviter).Error; err != nil {
					return err
				}
				if err := tx.Create(&Log{
					UserId:    rebate.InviterId,
					Username:  inviter.Username,
					CreatedAt: common.GetTimestamp(),
					Type:      LogTypeSystem,
					Content:   fmt.Sprintf("邀请返利成熟，获得 %s", logger.LogQuota(rebate.RebateQuota)),
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
