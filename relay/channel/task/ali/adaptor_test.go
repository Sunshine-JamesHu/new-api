package ali

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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

func TestAliLegacyHeaderDoesNotEnableOssResolve(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://dashscope.aliyuncs.com/api/v1/services/aigc/video-generation/video-synthesis", nil)
	err := (&TaskAdaptor{apiKey: "sk-test", ChannelType: constant.ChannelTypeAli}).BuildRequestHeader(nil, req, aliUnmappedRelayInfo())
	require.NoError(t, err)
	require.Equal(t, "enable", req.Header.Get("X-DashScope-Async"))
	require.Empty(t, req.Header.Get("X-DashScope-OssResourceResolve"))
}

func TestAliChannelMetadata(t *testing.T) {
	adaptor := &TaskAdaptor{ChannelType: constant.ChannelTypeAli}
	require.Equal(t, ChannelName, adaptor.GetChannelName())
	require.Contains(t, adaptor.GetModelList(), "wan2.5-i2v-preview")
	require.NotContains(t, adaptor.GetModelList(), "kling/kling-v3-video-generation")
	require.NotContains(t, adaptor.GetModelList(), "happyhorse-1.0-video-edit")
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
}

func TestProcessAliOtherRatiosEmitsResolutionKeyWithoutDefaultMultiplier(t *testing.T) {
	ratios, err := ProcessAliOtherRatios(&aliVideoRequestV2{
		Model: "unknown-video-model",
		Parameters: &aliVideoParametersV2{
			Resolution: "480",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 1.0, ratios["resolution-480P"])
}

func TestApplyAliParameterOverrideNormalizesResolution(t *testing.T) {
	for name, tc := range map[string]struct {
		input string
		want  string
	}{
		"480 numeric": {"480", "480P"},
		"720 lower":   {"720p", "720P"},
		"1080 upper":  {"1080P", "1080P"},
	} {
		t.Run(name, func(t *testing.T) {
			target := &aliVideoParametersV2{}
			applyAliParameterOverride(&aliVideoParametersV2{Resolution: tc.input}, target)
			require.Equal(t, tc.want, target.Resolution)
		})
	}
}

func TestAliRequestResolutionFieldNormalizesForUpstream(t *testing.T) {
	for name, tc := range map[string]struct {
		resolution string
		want       string
	}{
		"480 lower":    {"480p", "480P"},
		"720 numeric":  {"720", "720P"},
		"1080 upper":   {"1080P", "1080P"},
		"default 720P": {"", "720P"},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := (&TaskAdaptor{}).convertToAliRequestV2(aliRelayInfo("wan2.5-i2v-preview", "", false), relaycommon.TaskSubmitReq{
				Model:      "wan2.5-i2v-preview",
				Resolution: tc.resolution,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, req.Parameters.Resolution)
		})
	}

	for name, tc := range map[string]struct {
		resolution string
		want       string
	}{
		"480 numeric": {"480", "480P"},
		"720 lower":   {"720p", "720P"},
		"1080 lower":  {"1080p", "1080P"},
	} {
		t.Run("legacy "+name, func(t *testing.T) {
			legacyReq, err := (&TaskAdaptor{}).convertToAliRequest(aliRelayInfo("wan2.5-i2v-preview", "", false), relaycommon.TaskSubmitReq{
				Model:      "wan2.5-i2v-preview",
				Resolution: tc.resolution,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, legacyReq.Parameters.Resolution)
		})
	}
}

func TestAliValidateAndBuildLegacyRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{
		"model":"wan2.5-i2v-preview",
		"prompt":"scene",
		"input_reference":"https://example.com/start.png",
		"size":"720p",
		"duration":6
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	info := aliRelayInfo("wan2.5-i2v-preview", "", false)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)

	body, err := (&TaskAdaptor{ChannelType: constant.ChannelTypeAli}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data := make([]byte, 4096)
	n, err := body.Read(data)
	require.NoError(t, err)
	require.Contains(t, string(data[:n]), `"model":"wan2.5-i2v-preview"`)
	require.Contains(t, string(data[:n]), `"duration":6`)
}
