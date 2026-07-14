package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type testBillingSession struct{}

func (testBillingSession) Settle(int) error         { return nil }
func (testBillingSession) Refund(*gin.Context)      {}
func (testBillingSession) NeedsRefund() bool        { return false }
func (testBillingSession) GetPreConsumedQuota() int { return 0 }
func (testBillingSession) Reserve(int) error        { return nil }

func withRelayBillingConfig(t *testing.T, values map[string]string) {
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

func TestApplyConfiguredPerSecondMultipliersOverridesRequestFactors(t *testing.T) {
	withRelayBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"video-model":"per_second","ratio-model":"ratio"}`,
		"billing_setting.per_second_multipliers": `{
			"video-model":{
				"resolution-1080P":1.777778,
				"audio":2
			}
		}`,
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "video-model",
		PriceData:       types.PriceData{},
	}
	require.True(t, info.PriceData.ReplaceOtherRatios(map[string]float64{
		"seconds":          5,
		"resolution-1080P": 1,
		"audio":            1.2,
	}))

	applyConfiguredPerSecondMultipliers(info)

	require.Equal(t, 5.0, info.PriceData.OtherRatios()["seconds"])
	require.Equal(t, 1.777778, info.PriceData.OtherRatios()["resolution-1080P"])
	require.Equal(t, 2.0, info.PriceData.OtherRatios()["audio"])

	ratioInfo := &relaycommon.RelayInfo{
		OriginModelName: "ratio-model",
		PriceData:       types.PriceData{},
	}
	require.True(t, ratioInfo.PriceData.ReplaceOtherRatios(map[string]float64{"resolution-1080P": 1}))

	applyConfiguredPerSecondMultipliers(ratioInfo)

	require.Equal(t, 1.0, ratioInfo.PriceData.OtherRatios()["resolution-1080P"])
}

func TestRelayTaskSubmitHappyHorsePerSecondBillingAppliesToQuotaAndHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"happyhorse-1.0-t2v":0.001}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
	})

	withRelayBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
		"billing_setting.per_second_multipliers": `{
			"happyhorse-1.0-t2v":{"resolution-1080P":2}
		}`,
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/services/aigc/video-generation/video-synthesis", r.URL.Path)
		require.Equal(t, "Bearer sk-happyhorse", r.Header.Get("Authorization"))
		require.Equal(t, "enable", r.Header.Get("X-DashScope-Async"))
		require.Equal(t, "enable", r.Header.Get("X-DashScope-OssResourceResolve"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req_1","output":{"task_id":"task_upstream","task_status":"PENDING"}}`))
	}))
	t.Cleanup(upstream.Close)

	body := `{"model":"happyhorse-1.0-t2v","prompt":"horse","duration":6,"size":"1080p"}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeHappyHorse)
	common.SetContextKey(c, constant.ContextKeyChannelId, 998)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-happyhorse")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "happyhorse-1.0-t2v")
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserQuota, 1000000)
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "happyhorse-billing-token")

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	require.NoError(t, err)
	info.Billing = testBillingSession{}
	result, taskErr := RelayTaskSubmit(c, info)
	require.Nil(t, taskErr)
	require.NotNil(t, result)

	require.Equal(t, 6000, result.Quota)
	require.Equal(t, map[string]float64{
		"seconds":          6,
		"resolution-1080P": 2,
	}, info.PriceData.OtherRatios())

	var headerRatios map[string]float64
	require.NoError(t, common.Unmarshal([]byte(recorder.Header().Get("X-New-Api-Other-Ratios")), &headerRatios))
	require.Equal(t, info.PriceData.OtherRatios(), headerRatios)

	var openAIVideo dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &openAIVideo))
	require.Equal(t, "happyhorse-1.0-t2v", openAIVideo.Model)
	require.Equal(t, dto.VideoStatusQueued, openAIVideo.Status)
}
