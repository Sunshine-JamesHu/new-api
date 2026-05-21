package ali

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func aliRelayInfo(originModel, upstreamModel string, mapped bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: originModel,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     mapped,
			UpstreamModelName: upstreamModel,
		},
	}
}

func aliUnmappedRelayInfo() *relaycommon.RelayInfo {
	return aliRelayInfo("", "", false)
}

func TestConvertHappyHorseTextToVideoDefaults(t *testing.T) {
	req, err := (&TaskAdaptor{}).convertToAliRequestV2(aliUnmappedRelayInfo(), relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.0-t2v",
		Prompt: "horse",
	})
	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.0-t2v", req.Model)
	require.Equal(t, 5, req.Parameters.Duration)
	require.Equal(t, "1080P", req.Parameters.Resolution)
	require.Equal(t, "16:9", req.Parameters.Ratio)
	require.Empty(t, req.Parameters.Size)
	require.Empty(t, req.Input.Media)
}

func TestConvertHappyHorseMediaModels(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		images    []string
		wantTypes []string
	}{
		{
			name:      "image to video",
			model:     "happyhorse-1.0-i2v",
			images:    []string{"https://example.com/a.png"},
			wantTypes: []string{"first_frame"},
		},
		{
			name:      "reference to video",
			model:     "happyhorse-1.0-r2v",
			images:    []string{"https://example.com/a.png", "https://example.com/b.png"},
			wantTypes: []string{"reference_image", "reference_image"},
		},
		{
			name:      "video edit",
			model:     "happyhorse-1.0-video-edit",
			images:    []string{"https://example.com/in.mp4", "https://example.com/ref.png"},
			wantTypes: []string{"video", "reference_image"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := (&TaskAdaptor{}).convertToAliRequestV2(aliUnmappedRelayInfo(), relaycommon.TaskSubmitReq{
				Model:  tt.model,
				Prompt: "horse",
				Images: tt.images,
			})
			require.NoError(t, err)
			require.Len(t, req.Input.Media, len(tt.wantTypes))
			for i, wantType := range tt.wantTypes {
				require.Equal(t, wantType, req.Input.Media[i].Type)
				require.Equal(t, tt.images[i], req.Input.Media[i].URL)
				require.Empty(t, req.Input.Media[i].ImageURL)
				require.Empty(t, req.Input.Media[i].VideoURL)
			}
		})
	}
}

func TestConvertAliKlingMediaAndMetadata(t *testing.T) {
	req, err := (&TaskAdaptor{}).convertToAliRequestV2(aliRelayInfo(
		"mapped-kling",
		"kling/kling-v3-omni-video-generation",
		true,
	), relaycommon.TaskSubmitReq{
		Model:  "mapped-kling",
		Prompt: "scene",
		Images: []string{"https://example.com/start.png", "https://example.com/end.png"},
		Size:   "720p",
		Metadata: map[string]interface{}{
			"duration":     10,
			"audio":        true,
			"multi_prompt": []interface{}{"a", "b"},
			"element_list": []interface{}{map[string]interface{}{"prompt": "cat"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "kling/kling-v3-omni-video-generation", req.Model)
	require.Equal(t, "std", req.Parameters.Mode)
	require.Equal(t, 10, req.Parameters.Duration)
	require.NotNil(t, req.Parameters.Audio)
	require.True(t, *req.Parameters.Audio)
	require.Len(t, req.Input.Media, 2)
	require.Equal(t, "first_frame", req.Input.Media[0].Type)
	require.Equal(t, "last_frame", req.Input.Media[1].Type)
	require.NotNil(t, req.Input.MultiPrompt)
	require.NotNil(t, req.Input.ElementList)
}

func TestConvertAliDurationStringSources(t *testing.T) {
	req, err := (&TaskAdaptor{}).convertToAliRequestV2(aliUnmappedRelayInfo(), relaycommon.TaskSubmitReq{
		Model:   "happyhorse-1.0-t2v",
		Prompt:  "horse",
		Seconds: "7",
	})
	require.NoError(t, err)
	require.Equal(t, 7, req.Parameters.Duration)

	req, err = (&TaskAdaptor{}).convertToAliRequestV2(aliUnmappedRelayInfo(), relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.0-t2v",
		Prompt: "horse",
		Metadata: map[string]interface{}{
			"duration": "8",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 8, req.Parameters.Duration)
}

func TestBuildAliBailianRequestBodyUsesMappedModelForLogic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{
		"model":"alias-video",
		"prompt":"horse",
		"images":["https://example.com/start.png"],
		"size":"720p"
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	info := aliRelayInfo("alias-video", "happyhorse-1.0-i2v", true)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	body, err := (&TaskAdaptor{ChannelType: constant.ChannelTypeAliBailian}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Contains(t, string(data), `"model":"happyhorse-1.0-i2v"`)
	require.Contains(t, string(data), `"type":"first_frame"`)
	require.Contains(t, string(data), `"resolution":"720P"`)
}

func TestAliBailianHeaderEnablesOssResolve(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis", nil)
	err := (&TaskAdaptor{apiKey: "sk-test", ChannelType: constant.ChannelTypeAliBailian}).BuildRequestHeader(nil, req, aliUnmappedRelayInfo())
	require.NoError(t, err)
	require.Equal(t, "enable", req.Header.Get("X-DashScope-Async"))
	require.Equal(t, "enable", req.Header.Get("X-DashScope-OssResourceResolve"))
}

func TestAliLegacyHeaderDoesNotEnableOssResolve(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis", nil)
	err := (&TaskAdaptor{apiKey: "sk-test", ChannelType: constant.ChannelTypeAli}).BuildRequestHeader(nil, req, aliUnmappedRelayInfo())
	require.NoError(t, err)
	require.Equal(t, "enable", req.Header.Get("X-DashScope-Async"))
	require.Empty(t, req.Header.Get("X-DashScope-OssResourceResolve"))
}

func TestAliBailianChannelMetadata(t *testing.T) {
	adaptor := &TaskAdaptor{ChannelType: constant.ChannelTypeAliBailian}
	require.Equal(t, BailianMediaChannelName, adaptor.GetChannelName())
	require.Contains(t, adaptor.GetModelList(), "kling/kling-v3-video-generation")
	require.Contains(t, adaptor.GetModelList(), "happyhorse-1.0-video-edit")
	require.NotContains(t, adaptor.GetModelList(), "qwen-plus")

	legacyAdaptor := &TaskAdaptor{ChannelType: constant.ChannelTypeAli}
	require.Equal(t, ChannelName, legacyAdaptor.GetChannelName())
	require.Contains(t, legacyAdaptor.GetModelList(), "wan2.5-i2v-preview")
	require.NotContains(t, legacyAdaptor.GetModelList(), "kling/kling-v3-video-generation")
	require.NotContains(t, legacyAdaptor.GetModelList(), "happyhorse-1.0-video-edit")
}

func TestProcessAliOtherRatios(t *testing.T) {
	audio := true
	ratios, err := ProcessAliOtherRatios(&aliVideoRequestV2{
		Model: "kling/kling-v3-video-generation",
		Input: aliVideoInputV2{},
		Parameters: &aliVideoParametersV2{
			Mode:  "pro",
			Audio: &audio,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 2.8/1.4, ratios["resolution-1080P"])
	require.Equal(t, 5.0/2.8, ratios["audio"])

	ratios, err = ProcessAliOtherRatios(&aliVideoRequestV2{
		Model: "happyhorse-1.0-t2v",
		Parameters: &aliVideoParametersV2{
			Resolution: "1080p",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1.6/0.9, ratios["resolution-1080P"])
}

func TestProcessAliOtherRatiosEmitsResolutionKeyWithoutDefaultMultiplier(t *testing.T) {
	ratios, err := ProcessAliOtherRatios(&aliVideoRequestV2{
		Model: "unknown-video-model",
		Parameters: &aliVideoParametersV2{
			Resolution: "720p",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, ratios["resolution-720P"])
}

func TestAliEstimateBillingAppliesConfiguredPerSecondMultiplier(t *testing.T) {
	withAliBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
		"billing_setting.per_second_multipliers": `{
			"happyhorse-1.0-t2v":{"resolution-1080P":2.25}
		}`,
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{
		"model":"happyhorse-1.0-t2v",
		"prompt":"horse",
		"size":"1080p",
		"duration":6
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	info := aliRelayInfo("happyhorse-1.0-t2v", "", false)
	adaptor := &TaskAdaptor{ChannelType: constant.ChannelTypeAliBailian}
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	ratios := adaptor.EstimateBilling(c, info)
	require.Equal(t, float64(6), ratios["seconds"])
	require.Equal(t, 2.25, ratios["resolution-1080P"])
}

func withAliBillingConfig(t *testing.T, values map[string]string) {
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
	_ = billing_setting.GetBillingMode("")
}
