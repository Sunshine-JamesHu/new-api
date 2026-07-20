package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type UserInvitee struct {
	Id          int            `json:"id"`
	DisplayName string         `json:"display_name"`
	CreatedAt   int64          `json:"created_at"`
	UsedQuota   int            `json:"used_quota"`
	Status      int            `json:"status"`
	DeletedAt   gorm.DeletedAt `json:"-"`
	Deleted     bool           `json:"deleted" gorm:"-"`
}

func GetUserInvitees(inviterId int, pageInfo *common.PageInfo) (invitees []*UserInvitee, total int64, err error) {
	if inviterId == 0 {
		return nil, 0, errors.New("invalid inviter id")
	}

	query := DB.Unscoped().Model(&User{}).Where("inviter_id = ?", inviterId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = query.
		Select("id", "display_name", "created_at", "used_quota", "status", "deleted_at").
		Order("created_at DESC").
		Order("id DESC").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&invitees).Error
	if err != nil {
		return nil, 0, err
	}

	for _, invitee := range invitees {
		invitee.Deleted = invitee.DeletedAt.Valid
	}
	return invitees, total, nil
}
