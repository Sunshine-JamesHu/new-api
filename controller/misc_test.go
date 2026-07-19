package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatusExposesEffectiveAffiliateRebateConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := *operation_setting.GetPaymentSetting()
	t.Cleanup(func() {
		*operation_setting.GetPaymentSetting() = original
	})

	tests := []struct {
		name        string
		enabled     bool
		compliant   bool
		rate        float64
		wantEnabled bool
		wantRate    float64
	}{
		{
			name:        "enabled and compliant",
			enabled:     true,
			compliant:   true,
			rate:        12.5,
			wantEnabled: true,
			wantRate:    12.5,
		},
		{
			name:      "disabled",
			rate:      12.5,
			compliant: true,
			wantRate:  12.5,
		},
		{
			name:     "compliance not confirmed",
			enabled:  true,
			rate:     12.5,
			wantRate: 12.5,
		},
		{
			name:        "rate is clamped",
			enabled:     true,
			compliant:   true,
			rate:        150,
			wantEnabled: true,
			wantRate:    100,
		},
		{
			name:      "negative rate is clamped",
			enabled:   true,
			compliant: true,
			rate:      -5,
			wantRate:  0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paymentSetting := operation_setting.GetPaymentSetting()
			paymentSetting.AffiliateRebateEnabled = test.enabled
			paymentSetting.AffiliateRebateRate = test.rate
			paymentSetting.ComplianceConfirmed = test.compliant
			paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			GetStatus(context)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Data struct {
					AffiliateRebateEnabled bool    `json:"affiliate_rebate_enabled"`
					AffiliateRebateRate    float64 `json:"affiliate_rebate_rate"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, test.wantEnabled, response.Data.AffiliateRebateEnabled)
			assert.Equal(t, test.wantRate, response.Data.AffiliateRebateRate)
		})
	}
}
