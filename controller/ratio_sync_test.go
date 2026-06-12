package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withRatioSyncBillingConfig(t *testing.T, values map[string]string) {
	t.Helper()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(values))
}

func TestNormalizeSyncValueCanonicalizesPerSecondMultiplierAliases(t *testing.T) {
	value := normalizeSyncValue(billing_setting.PerSecondMultipliersField, map[string]any{
		"resolution-480p": 0.5,
		"resolution-720":  1,
		"zero":            0,
	})

	require.Equal(t, map[string]float64{
		"resolution-480P": 0.5,
		"resolution-720P": 1,
	}, value)
}

func TestBuildDifferencesIgnoresPerSecondMultiplierAliasOnlyChanges(t *testing.T) {
	localData := map[string]any{
		billing_setting.PerSecondMultipliersField: map[string]any{
			"video-model": map[string]any{
				"resolution-720P":  1,
				"resolution-1080P": 1.777778,
			},
		},
	}
	upstreams := []struct {
		name string
		data map[string]any
	}{
		{
			name: "upstream",
			data: map[string]any{
				billing_setting.PerSecondMultipliersField: map[string]any{
					"video-model": map[string]any{
						"resolution-720":   1,
						"resolution-1080p": 1.777778,
					},
				},
			},
		},
	}

	require.Empty(t, buildDifferences(localData, upstreams))
}

func TestFetchUpstreamRatiosNormalizesPerSecondMultipliersFromPricingAPI(t *testing.T) {
	withRatioSyncBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"video-model":"per_second"}`,
		"billing_setting.per_second_multipliers": `{
			"video-model":{"resolution-480P":0.5,"resolution-720P":1,"resolution-1080P":1.777778}
		}`,
	})
	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"video-model":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/pricing", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"data": [{
				"model_name": "video-model",
				"quota_type": 1,
				"model_price": 1,
				"billing_mode": "per_second",
				"per_second_multipliers": {
					"resolution-480": 0.5,
					"resolution-720p": 1,
					"resolution-1080p": 1.777778
				}
			}]
		}`))
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ratio/fetch", strings.NewReader(`{
		"upstreams": [{"name":"upstream","base_url":"`+upstream.URL+`"}],
		"timeout": 5
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	FetchUpstreamRatios(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Differences map[string]any `json:"differences"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Empty(t, body.Data.Differences)
}
