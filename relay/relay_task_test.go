package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
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

func testPriceDataWithOtherRatios(ratios map[string]float64) types.PriceData {
	priceData := types.PriceData{}
	for key, ratio := range ratios {
		priceData.AddOtherRatio(key, ratio)
	}
	return priceData
}

func TestRelayTaskSubmitJimengUsesMappedReqKeyAndMetadataPrompt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"alias-jimeng":0.001}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
	})

	withRelayBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"alias-jimeng":"per_second"}`,
	})

	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/jimeng/", r.URL.Path)
		require.Equal(t, "CVSync2AsyncSubmitTask", r.URL.Query().Get("Action"))
		require.Equal(t, "Bearer sk-jimeng", r.Header.Get("Authorization"))
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":10000,"message":"Success","request_id":"req_1","data":{"task_id":"task_upstream"}}`))
	}))
	t.Cleanup(upstream.Close)

	body := `{"model":"alias-jimeng","prompt":"outer ignored","duration":5,"metadata":{"prompt":"real prompt","seed":-1,"aspect_ratio":"16:9","frames":999}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("model_mapping", `{"alias-jimeng":"jimeng_t2v_v30"}`)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeJimeng)
	common.SetContextKey(c, constant.ContextKeyChannelId, 999)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-jimeng")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "alias-jimeng")
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserQuota, 1000000)
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "jimeng-billing-token")

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	require.NoError(t, err)
	info.Billing = testBillingSession{}
	result, taskErr := RelayTaskSubmit(c, info)
	require.Nil(t, taskErr)
	require.NotNil(t, result)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &payload))
	require.Equal(t, "jimeng_t2v_v30", payload["req_key"])
	require.Equal(t, "real prompt", payload["prompt"])
	require.Equal(t, float64(121), payload["frames"])
	require.Equal(t, float64(-1), payload["seed"])
	require.Equal(t, "16:9", payload["aspect_ratio"])
	require.NotContains(t, string(upstreamBody), "outer ignored")
	require.NotContains(t, string(upstreamBody), "alias-jimeng")
	require.Contains(t, string(result.TaskData), `"req_key":"jimeng_t2v_v30"`)
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
		PriceData: testPriceDataWithOtherRatios(map[string]float64{
			"seconds":          5,
			"resolution-1080P": 1,
			"audio":            1.2,
		}),
	}

	applyConfiguredPerSecondMultipliers(info)

	otherRatios := info.PriceData.OtherRatios()
	require.Equal(t, 5.0, otherRatios["seconds"])
	require.Equal(t, 1.777778, otherRatios["resolution-1080P"])
	require.Equal(t, 2.0, otherRatios["audio"])

	ratioInfo := &relaycommon.RelayInfo{
		OriginModelName: "ratio-model",
		PriceData:       testPriceDataWithOtherRatios(map[string]float64{"resolution-1080P": 1}),
	}

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

func TestRelayTaskSubmitNewApiVideoUsesLocalRequestRatiosForQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"happyhorse-1.0-t2v":0.001}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
	})

	withRelayBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
	})

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/video/generations", r.URL.Path)
		require.Equal(t, "Bearer sk-newapi", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-New-Api-Other-Ratios", `{"seconds":9,"resolution-1080P":2}`)
		data, err := common.Marshal(dto.TaskResponse[any]{
			Code: "success",
			Data: map[string]any{
				"task_id":  "task_upstream",
				"status":   string(model.TaskStatusSubmitted),
				"progress": "10%",
			},
		})
		require.NoError(t, err)
		_, _ = w.Write(data)
	}))
	t.Cleanup(upstream.Close)

	body := `{"model":"happyhorse-1.0-t2v","prompt":"horse","duration":5,"resolution":"720P"}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeNewApiVideo)
	common.SetContextKey(c, constant.ContextKeyChannelId, 997)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-newapi")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "happyhorse-1.0-t2v")
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserQuota, 1000000)
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "newapi-video-billing-token")

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	require.NoError(t, err)
	info.Billing = testBillingSession{}
	result, taskErr := RelayTaskSubmit(c, info)
	require.Nil(t, taskErr)
	require.NotNil(t, result)

	require.Equal(t, 2500, result.Quota)
	require.Equal(t, map[string]float64{
		"seconds":         5,
		"resolution-720P": 1,
	}, info.PriceData.OtherRatios())
	require.JSONEq(t, `{"seconds":5,"resolution-720P":1}`, recorder.Header().Get("X-New-Api-Other-Ratios"))
}
