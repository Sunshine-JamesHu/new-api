package service

import (
	"github.com/QuantumNous/new-api/model"
)

func ConsumeAffiliateRebateMaturity(userId int, quota int) {
	model.ConsumeAffiliateRebateMaturity(userId, quota)
}
