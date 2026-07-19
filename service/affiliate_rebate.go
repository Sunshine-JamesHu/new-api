package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type RegistrationAffiliate struct {
	InviterID int
	Group     string
}

func ResolveRegistrationAffiliate(affCode string) RegistrationAffiliate {
	inviterID, err := model.GetUserIdByAffCode(affCode)
	if err != nil {
		return RegistrationAffiliate{}
	}
	inviter, err := model.GetUserById(inviterID, false)
	if err != nil || inviter.Status != common.UserStatusEnabled {
		return RegistrationAffiliate{}
	}

	result := RegistrationAffiliate{InviterID: inviterID}
	if ratio_setting.ContainsGroupRatio(affCode) {
		result.Group = affCode
	}
	return result
}

func ConsumeAffiliateRebateMaturity(userId int, quota int) {
	model.ConsumeAffiliateRebateMaturity(userId, quota)
}
