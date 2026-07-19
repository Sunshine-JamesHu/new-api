package service

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withAffiliateRebateSetting(t *testing.T, enabled bool, rate float64) {
	t.Helper()
	original := *operation_setting.GetPaymentSetting()
	operation_setting.GetPaymentSetting().AffiliateRebateEnabled = enabled
	operation_setting.GetPaymentSetting().AffiliateRebateRate = rate
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	operation_setting.GetPaymentSetting().ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		*operation_setting.GetPaymentSetting() = original
	})
}

func TestResolveRegistrationAffiliate(t *testing.T) {
	truncate(t)
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"cockpit":1,"private":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
	})

	require.NoError(t, model.DB.Create(&model.User{
		Id:       1201,
		Username: "cockpit_inviter",
		AffCode:  "cockpit",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1202,
		Username: "standard_inviter",
		AffCode:  "standard",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1203,
		Username: "disabled_inviter",
		AffCode:  "private",
		Status:   common.UserStatusDisabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1204,
		Username: "case_sensitive_inviter",
		AffCode:  "Cockpit",
		Status:   common.UserStatusEnabled,
	}).Error)

	tests := []struct {
		name    string
		affCode string
		want    RegistrationAffiliate
	}{
		{
			name:    "valid invite code with matching group",
			affCode: "cockpit",
			want: RegistrationAffiliate{
				InviterID: 1201,
				Group:     "cockpit",
			},
		},
		{
			name:    "valid invite code without matching group",
			affCode: "standard",
			want: RegistrationAffiliate{
				InviterID: 1202,
			},
		},
		{
			name:    "disabled inviter cannot assign matching group",
			affCode: "private",
			want:    RegistrationAffiliate{},
		},
		{
			name:    "group matching is case sensitive",
			affCode: "Cockpit",
			want: RegistrationAffiliate{
				InviterID: 1204,
			},
		},
		{
			name:    "matching group without inviter",
			affCode: "missing",
			want:    RegistrationAffiliate{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, ResolveRegistrationAffiliate(test.affCode))
		})
	}
}

func TestAffiliateRebateMaturesAfterInviteeConsumesTopUpQuota(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1001,
		Username: "inviter_rebate",
		AffCode:  "AFF1001",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1002,
		Username:  "invitee_rebate",
		AffCode:   "AFF1002",
		Status:    common.UserStatusEnabled,
		Quota:     10000,
		InviterId: 1001,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2001,
		UserId:          invitee.Id,
		Amount:          10000,
		Money:           1,
		TradeNo:         "rebate-mature",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 10000))

	ConsumeAffiliateRebateMaturity(invitee.Id, 9999)
	var pending model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&pending).Error)
	assert.Equal(t, model.AffiliateRebateStatusPending, pending.Status)
	assert.Equal(t, 1, pending.RemainingQuota)
	assertUserAffiliateQuota(t, 1001, 0, 0)

	ConsumeAffiliateRebateMaturity(invitee.Id, 1)
	var settled model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&settled).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, settled.Status)
	assert.Equal(t, 0, settled.RemainingQuota)
	assert.Equal(t, 1000, settled.RebateQuota)
	assertUserAffiliateQuota(t, 1001, 1000, 1000)
}

func TestAffiliateRebateBillingSessionExactSettleMatures(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1051,
		Username: "inviter_exact_settle",
		AffCode:  "AFF1051",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1052,
		Username:  "invitee_exact_settle",
		AffCode:   "AFF1052",
		Status:    common.UserStatusEnabled,
		Quota:     3000,
		InviterId: 1051,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2051,
		UserId:          invitee.Id,
		Amount:          3000,
		Money:           1,
		TradeNo:         "rebate-exact-settle",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 3000))

	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: invitee.Id},
		funding:          &WalletFunding{userId: invitee.Id},
		preConsumedQuota: 3000,
	}
	require.NoError(t, session.Settle(3000))

	var rebate model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&rebate).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, rebate.Status)
	assert.Equal(t, 0, rebate.RemainingQuota)
	assertUserAffiliateQuota(t, 1051, 300, 300)
}

func TestAffiliateRebateFallbackExactSettleMatures(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1053,
		Username: "inviter_fallback_exact",
		AffCode:  "AFF1053",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1054,
		Username:  "invitee_fallback_exact",
		AffCode:   "AFF1054",
		Status:    common.UserStatusEnabled,
		Quota:     4000,
		InviterId: 1053,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2053,
		UserId:          invitee.Id,
		Amount:          4000,
		Money:           1,
		TradeNo:         "rebate-fallback-exact",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 4000))

	relayInfo := &relaycommon.RelayInfo{
		UserId:                invitee.Id,
		FinalPreConsumedQuota: 4000,
		BillingSource:         BillingSourceWallet,
	}
	require.NoError(t, SettleBilling(nil, relayInfo, 4000))

	var rebate model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&rebate).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, rebate.Status)
	assert.Equal(t, 0, rebate.RemainingQuota)
	assertUserAffiliateQuota(t, 1053, 400, 400)
}

func TestDirectInvitationRewardAndTopUpRebateCoexist(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 20)
	originalNewUserQuota := common.QuotaForNewUser
	originalInviteeQuota := common.QuotaForInvitee
	originalInviterQuota := common.QuotaForInviter
	common.QuotaForNewUser = 0
	common.QuotaForInvitee = 0
	common.QuotaForInviter = 333
	t.Cleanup(func() {
		common.QuotaForNewUser = originalNewUserQuota
		common.QuotaForInvitee = originalInviteeQuota
		common.QuotaForInviter = originalInviterQuota
	})

	require.NoError(t, model.DB.Create(&model.User{
		Id:       1101,
		Username: "inviter_coexist",
		AffCode:  "AFF1101",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1102,
		Username:  "invitee_coexist",
		Password:  "password123",
		AffCode:   "AFF1102",
		Status:    common.UserStatusEnabled,
		InviterId: 1101,
	}
	require.NoError(t, invitee.Insert(1101))
	assertUserAffiliateQuota(t, 1101, 333, 333)

	topUp := &model.TopUp{
		Id:              2101,
		UserId:          invitee.Id,
		Amount:          1000,
		Money:           1,
		TradeNo:         "rebate-coexist",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 1000))
	assertUserAffiliateQuota(t, 1101, 333, 333)

	pendingQuota, err := model.SumUserPendingAffiliateRebateQuota(1101)
	require.NoError(t, err)
	assert.Equal(t, int64(200), pendingQuota)

	ConsumeAffiliateRebateMaturity(invitee.Id, 1000)

	assertUserAffiliateQuota(t, 1101, 533, 533)
	pendingQuota, err = model.SumUserPendingAffiliateRebateQuota(1101)
	require.NoError(t, err)
	assert.Equal(t, int64(0), pendingQuota)
}

func TestAffiliateRebateWaitsForExistingBalanceBeforeTopUpQuota(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1003,
		Username: "inviter_existing_balance",
		AffCode:  "AFF1003",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1004,
		Username:  "invitee_existing_balance",
		AffCode:   "AFF1004",
		Status:    common.UserStatusEnabled,
		Quota:     15000,
		InviterId: 1003,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2003,
		UserId:          invitee.Id,
		Amount:          10000,
		Money:           1,
		TradeNo:         "rebate-existing-balance",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 10000))

	ConsumeAffiliateRebateMaturity(invitee.Id, 5000)
	var pending model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&pending).Error)
	assert.Equal(t, model.AffiliateRebateStatusPending, pending.Status)
	assert.Equal(t, 10000, pending.RemainingQuota)
	assertUserAffiliateQuota(t, 1003, 0, 0)

	ConsumeAffiliateRebateMaturity(invitee.Id, 9999)
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&pending).Error)
	assert.Equal(t, model.AffiliateRebateStatusPending, pending.Status)
	assert.Equal(t, 1, pending.RemainingQuota)
	assertUserAffiliateQuota(t, 1003, 0, 0)

	ConsumeAffiliateRebateMaturity(invitee.Id, 1)
	var settled model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&settled).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, settled.Status)
	assert.Equal(t, 0, settled.RemainingQuota)
	assertUserAffiliateQuota(t, 1003, 1000, 1000)
}

func TestAffiliateRebateMultipleTopUpsMatureByEachTopUpBalance(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1005,
		Username: "inviter_multi",
		AffCode:  "AFF1005",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1006,
		Username:  "invitee_multi",
		AffCode:   "AFF1006",
		Status:    common.UserStatusEnabled,
		Quota:     5000,
		InviterId: 1005,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	firstTopUp := &model.TopUp{
		Id:              2005,
		UserId:          invitee.Id,
		Amount:          5000,
		TradeNo:         "rebate-multi-first",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(firstTopUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, firstTopUp, 5000))

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", invitee.Id).Update("quota", 12000).Error)
	secondTopUp := &model.TopUp{
		Id:              2006,
		UserId:          invitee.Id,
		Amount:          7000,
		TradeNo:         "rebate-multi-second",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(secondTopUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, secondTopUp, 7000))

	ConsumeAffiliateRebateMaturity(invitee.Id, 5000)

	var first model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", firstTopUp.Id).First(&first).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, first.Status)
	assert.Equal(t, 0, first.RemainingQuota)
	var second model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", secondTopUp.Id).First(&second).Error)
	assert.Equal(t, model.AffiliateRebateStatusPending, second.Status)
	assert.Equal(t, 7000, second.RemainingQuota)
	assertUserAffiliateQuota(t, 1005, 500, 500)

	ConsumeAffiliateRebateMaturity(invitee.Id, 7000)
	require.NoError(t, model.DB.Where("top_up_id = ?", secondTopUp.Id).First(&second).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, second.Status)
	assert.Equal(t, 0, second.RemainingQuota)
	assertUserAffiliateQuota(t, 1005, 1200, 1200)
}

func TestAffiliateRebateDisabledDoesNotCreateRebate(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, false, 20)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1011,
		Username: "inviter_disabled",
		AffCode:  "AFF1011",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1012,
		Username:  "invitee_disabled",
		AffCode:   "AFF1012",
		Status:    common.UserStatusEnabled,
		Quota:     5000,
		InviterId: 1011,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2011,
		UserId:          invitee.Id,
		Amount:          5000,
		TradeNo:         "rebate-disabled",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 5000))

	var count int64
	require.NoError(t, model.DB.Model(&model.AffiliateRebate{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestAffiliateRebateExistingPendingStillMaturesAfterFeatureDisabled(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 20)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1015,
		Username: "inviter_existing_pending",
		AffCode:  "AFF1015",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1016,
		Username:  "invitee_existing_pending",
		AffCode:   "AFF1016",
		Status:    common.UserStatusEnabled,
		InviterId: 1015,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2015,
		UserId:          invitee.Id,
		Amount:          5000,
		TradeNo:         "rebate-existing-pending",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 5000))

	operation_setting.GetPaymentSetting().AffiliateRebateEnabled = false
	ConsumeAffiliateRebateMaturity(invitee.Id, 5000)

	var rebate model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&rebate).Error)
	assert.Equal(t, model.AffiliateRebateStatusSettled, rebate.Status)
	assert.Equal(t, 0, rebate.RemainingQuota)
	assertUserAffiliateQuota(t, 1015, 1000, 1000)
}

func TestAffiliateRebateCreateIsIdempotentForSameTopUp(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 12.5)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1021,
		Username: "inviter_idempotent",
		AffCode:  "AFF1021",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1022,
		Username:  "invitee_idempotent",
		AffCode:   "AFF1022",
		Status:    common.UserStatusEnabled,
		InviterId: 1021,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2021,
		UserId:          invitee.Id,
		Amount:          999,
		TradeNo:         "rebate-idempotent",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 999))
	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 999))

	var rebates []model.AffiliateRebate
	require.NoError(t, model.DB.Find(&rebates).Error)
	require.Len(t, rebates, 1)
	assert.Equal(t, 124, rebates[0].RebateQuota)
	assert.Equal(t, 12.5, rebates[0].RatePercent)
}

func TestAffiliateRebateRateAboveHundredIsClamped(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 150)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1031,
		Username: "inviter_clamped",
		AffCode:  "AFF1031",
		Status:   common.UserStatusEnabled,
	}).Error)
	invitee := &model.User{
		Id:        1032,
		Username:  "invitee_clamped",
		AffCode:   "AFF1032",
		Status:    common.UserStatusEnabled,
		InviterId: 1031,
	}
	require.NoError(t, model.DB.Create(invitee).Error)
	topUp := &model.TopUp{
		Id:              2031,
		UserId:          invitee.Id,
		Amount:          500,
		TradeNo:         "rebate-clamped",
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(topUp).Error)

	require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, 500))

	var rebate model.AffiliateRebate
	require.NoError(t, model.DB.Where("top_up_id = ?", topUp.Id).First(&rebate).Error)
	assert.Equal(t, 500, rebate.RebateQuota)
	assert.Equal(t, float64(100), rebate.RatePercent)
}

func TestAffiliateRebateOptionsAreSeededInOptionMap(t *testing.T) {
	original := *operation_setting.GetPaymentSetting()
	operation_setting.GetPaymentSetting().AffiliateRebateEnabled = true
	operation_setting.GetPaymentSetting().AffiliateRebateRate = 12.5
	t.Cleanup(func() {
		*operation_setting.GetPaymentSetting() = original
		model.InitOptionMap()
	})

	model.InitOptionMap()

	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["payment_setting.affiliate_rebate_enabled"]
	rate := common.OptionMap["payment_setting.affiliate_rebate_rate"]
	common.OptionMapRWMutex.RUnlock()
	assert.Equal(t, strconv.FormatBool(true), enabled)
	assert.Equal(t, "12.5", rate)
}

func TestGetUserAffiliateRebatesScopesAndSumsPendingQuota(t *testing.T) {
	truncate(t)
	withAffiliateRebateSetting(t, true, 10)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1041,
		Username: "inviter_list",
		AffCode:  "AFF1041",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1042,
		Username: "other_inviter_list",
		AffCode:  "AFF1042",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        1043,
		Username:  "invitee_list",
		AffCode:   "AFF1043",
		Status:    common.UserStatusEnabled,
		InviterId: 1041,
		Quota:     1000,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        1044,
		Username:  "other_invitee_list",
		AffCode:   "AFF1044",
		Status:    common.UserStatusEnabled,
		InviterId: 1042,
		Quota:     2000,
	}).Error)
	topUps := []*model.TopUp{
		{Id: 2041, UserId: 1043, Amount: 1000, TradeNo: "rebate-list-one", PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix()},
		{Id: 2042, UserId: 1043, Amount: 2000, TradeNo: "rebate-list-two", PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix()},
		{Id: 2043, UserId: 1044, Amount: 2000, TradeNo: "rebate-list-other", PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix()},
	}
	for _, topUp := range topUps {
		require.NoError(t, model.DB.Create(topUp).Error)
		require.NoError(t, model.CreatePendingAffiliateRebateForTopUp(model.DB, topUp, int(topUp.Amount)))
	}

	rebates, total, err := model.GetUserAffiliateRebates(1041, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, rebates, 2)
	assert.Equal(t, 1041, rebates[0].InviterId)
	assert.Equal(t, 1041, rebates[1].InviterId)

	pendingQuota, err := model.SumUserPendingAffiliateRebateQuota(1041)
	require.NoError(t, err)
	assert.Equal(t, int64(300), pendingQuota)
}

func assertUserAffiliateQuota(t *testing.T, userId int, wantAffQuota int, wantHistory int) {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("aff_quota", "aff_history").Where("id = ?", userId).First(&user).Error)
	assert.Equal(t, wantAffQuota, user.AffQuota)
	assert.Equal(t, wantHistory, user.AffHistoryQuota)
}
