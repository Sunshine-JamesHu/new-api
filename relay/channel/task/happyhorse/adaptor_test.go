package happyhorse

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func happyHorseRelayInfo(originModel, upstreamModel string, mapped bool) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: originModel,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			IsModelMapped:     mapped,
			UpstreamModelName: upstreamModel,
		},
	}
}

func happyHorseContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestConvertTextToVideoDefaults(t *testing.T) {
	req, err := convertToRequest(happyHorseRelayInfo("", "", false), relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.0-t2v",
		Prompt: "horse",
	})
	require.NoError(t, err)
	require.Equal(t, "happyhorse-1.0-t2v", req.Model)
	require.Equal(t, "horse", req.Input["prompt"])
	require.Equal(t, 5, req.Parameters["duration"])
	require.Equal(t, "1080P", req.Parameters["resolution"])
	require.Equal(t, "16:9", req.Parameters["ratio"])
	require.NotContains(t, req.Input, "media")
}

func TestConvertMediaModels(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		images    []string
		wantTypes []string
	}{
		{name: "image to video", model: "happyhorse-1.0-i2v", images: []string{"https://example.com/a.png"}, wantTypes: []string{"first_frame"}},
		{name: "reference to video", model: "happyhorse-1.0-r2v", images: []string{"https://example.com/a.png", "https://example.com/b.png"}, wantTypes: []string{"reference_image", "reference_image"}},
		{name: "video edit", model: "happyhorse-1.0-video-edit", images: []string{"https://example.com/in.mp4", "https://example.com/ref.png"}, wantTypes: []string{"video", "reference_image"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := convertToRequest(happyHorseRelayInfo("", "", false), relaycommon.TaskSubmitReq{
				Model:  tt.model,
				Prompt: "horse",
				Images: tt.images,
			})
			require.NoError(t, err)
			media, ok := req.Input["media"].([]map[string]any)
			require.True(t, ok)
			require.Len(t, media, len(tt.wantTypes))
			for i, wantType := range tt.wantTypes {
				require.Equal(t, wantType, media[i]["type"])
				require.Equal(t, tt.images[i], media[i]["url"])
			}
		})
	}
}

func TestPreservesExplicitInputAndParametersIncludingZeroValues(t *testing.T) {
	req, err := convertToRequest(happyHorseRelayInfo("", "", false), relaycommon.TaskSubmitReq{
		Model:  "happyhorse-1.0-r2v",
		Prompt: "outer prompt",
		Images: []string{"https://example.com/compat.png"},
		Input: map[string]any{
			"prompt": "inner prompt",
			"media": []any{
				map[string]any{"type": "reference_image", "url": "https://example.com/ref.png"},
			},
			"custom_empty_list": []any{},
		},
		Parameters: map[string]any{
			"duration":  8,
			"watermark": false,
			"seed":      0,
			"custom":    "kept",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "inner prompt", req.Input["prompt"])
	require.Equal(t, []any{}, req.Input["custom_empty_list"])
	require.Equal(t, 8, req.Parameters["duration"])
	require.Equal(t, false, req.Parameters["watermark"])
	require.Equal(t, 0, req.Parameters["seed"])
	require.Equal(t, "kept", req.Parameters["custom"])
}

func TestBuildRequestBodyUsesMappedModelForLogic(t *testing.T) {
	c := happyHorseContext(`{
		"model":"alias-video",
		"prompt":"horse",
		"images":["https://example.com/start.png"],
		"size":"720p"
	}`)
	info := happyHorseRelayInfo("alias-video", "happyhorse-1.0-i2v", true)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Contains(t, string(data), `"model":"happyhorse-1.0-i2v"`)
	require.Contains(t, string(data), `"type":"first_frame"`)
	require.Contains(t, string(data), `"resolution":"720P"`)
}

func TestHeaderEnablesOssResolve(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis", nil)
	err := (&TaskAdaptor{apiKey: "sk-test"}).BuildRequestHeader(nil, req, happyHorseRelayInfo("", "", false))
	require.NoError(t, err)
	require.Equal(t, "enable", req.Header.Get("X-DashScope-Async"))
	require.Equal(t, "enable", req.Header.Get("X-DashScope-OssResourceResolve"))
}

func TestChannelMetadata(t *testing.T) {
	adaptor := &TaskAdaptor{}
	require.Equal(t, ChannelName, adaptor.GetChannelName())
	require.Contains(t, adaptor.GetModelList(), "happyhorse-1.0-video-edit")
	require.NotContains(t, adaptor.GetModelList(), "kling/kling-v3-video-generation")
	require.NotContains(t, adaptor.GetModelList(), "qwen-plus")
}

func TestEstimateBillingAppliesConfiguredPerSecondMultiplier(t *testing.T) {
	withHappyHorseBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
		"billing_setting.per_second_multipliers": `{
			"happyhorse-1.0-t2v":{"resolution-1080P":2.25}
		}`,
	})

	c := happyHorseContext(`{
		"model":"happyhorse-1.0-t2v",
		"prompt":"horse",
		"size":"1080p",
		"duration":6
	}`)
	info := happyHorseRelayInfo("happyhorse-1.0-t2v", "", false)
	adaptor := &TaskAdaptor{}
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	ratios := adaptor.EstimateBilling(c, info)
	require.Equal(t, float64(6), ratios["seconds"])
	require.Equal(t, 2.25, ratios["resolution-1080P"])
}

func TestEstimateBillingSkipsNonPerSecondModel(t *testing.T) {
	withHappyHorseBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{}`,
	})

	c := happyHorseContext(`{
		"model":"happyhorse-1.0-t2v",
		"prompt":"horse",
		"duration":9
	}`)
	info := happyHorseRelayInfo("happyhorse-1.0-t2v", "", false)
	adaptor := &TaskAdaptor{}
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	require.Nil(t, adaptor.EstimateBilling(c, info))
}

func TestEstimateBillingUsesDefaultDurationAndResolution(t *testing.T) {
	withHappyHorseBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
	})

	c := happyHorseContext(`{
		"model":"happyhorse-1.0-t2v",
		"prompt":"horse"
	}`)
	info := happyHorseRelayInfo("happyhorse-1.0-t2v", "", false)
	adaptor := &TaskAdaptor{}
	taskErr := adaptor.ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	ratios := adaptor.EstimateBilling(c, info)
	require.Equal(t, float64(5), ratios["seconds"])
	require.InEpsilon(t, 1.6/0.9, ratios["resolution-1080P"], 0.000001)
}

func TestEstimateBillingParsesSecondsAndExplicitZeroFallback(t *testing.T) {
	withHappyHorseBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"happyhorse-1.0-t2v":"per_second"}`,
	})

	tests := []struct {
		name        string
		body        string
		wantSeconds float64
	}{
		{
			name: "seconds string ceil",
			body: `{
				"model":"happyhorse-1.0-t2v",
				"prompt":"horse",
				"seconds":"7.2"
			}`,
			wantSeconds: 8,
		},
		{
			name: "parameter zero falls back",
			body: `{
				"model":"happyhorse-1.0-t2v",
				"prompt":"horse",
				"parameters":{"duration":0}
			}`,
			wantSeconds: 5,
		},
		{
			name: "parameter duration wins",
			body: `{
				"model":"happyhorse-1.0-t2v",
				"prompt":"horse",
				"parameters":{"duration":11}
			}`,
			wantSeconds: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := happyHorseContext(tt.body)
			info := happyHorseRelayInfo("happyhorse-1.0-t2v", "", false)
			adaptor := &TaskAdaptor{}
			taskErr := adaptor.ValidateRequestAndSetAction(c, info)
			require.Nil(t, taskErr)

			ratios := adaptor.EstimateBilling(c, info)
			require.Equal(t, tt.wantSeconds, ratios["seconds"])
		})
	}
}

func withHappyHorseBillingConfig(t *testing.T, values map[string]string) {
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
