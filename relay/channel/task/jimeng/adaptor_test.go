package jimeng

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func jimengContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func jimengRelayInfo(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: model,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: model,
		},
	}
}

func TestJimengBuildRequestBodyUsesMetadataAndOuterBillingFields(t *testing.T) {
	c := jimengContext(`{
		"model":"jimeng_vgfm_t2v_l20",
		"prompt":"ignored",
		"image":"ignored",
		"images":["ignored"],
		"duration":10,
		"metadata":{"prompt":"real prompt","frames":999,"image_urls":["https://example.com/a.png"]}
	}`)
	info := jimengRelayInfo("jimeng_vgfm_t2v_l20")
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	require.Contains(t, string(data), `"req_key":"jimeng_vgfm_t2v_l20"`)
	require.Contains(t, string(data), `"frames":241`)
	require.Contains(t, string(data), `"prompt":"real prompt"`)
	require.NotContains(t, string(data), `"ignored"`)
}

func TestJimengBuildRequestBodyUsesMappedUpstreamModelAsReqKey(t *testing.T) {
	c := jimengContext(`{"model":"alias-jimeng","duration":5,"metadata":{"prompt":"real prompt"}}`)
	info := jimengRelayInfo("alias-jimeng")
	info.UpstreamModelName = "jimeng_t2v_v30"
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))

	body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	require.Contains(t, string(data), `"req_key":"jimeng_t2v_v30"`)
	require.NotContains(t, string(data), `"req_key":"alias-jimeng"`)
}

func TestJimengSubmitLogSummaryRedactsPayload(t *testing.T) {
	got := jimengSubmitLogSummary(map[string]any{
		"req_key":            "jimeng_t2v_v30",
		"prompt":             "real prompt",
		"frames":             121,
		"binary_data_base64": []string{"secret-base64"},
	})

	require.Contains(t, got, `"req_key":"jimeng_t2v_v30"`)
	require.Contains(t, got, `"frames":121`)
	require.Contains(t, got, `"has_prompt":true`)
	require.Contains(t, got, `"binary_data_base64s":1`)
	require.NotContains(t, got, "real prompt")
	require.NotContains(t, got, "secret-base64")
}

func TestJimengIgnoresOuterResolution(t *testing.T) {
	c := jimengContext(`{"model":"jimeng_vgfm_t2v_l20","resolution":"4K","metadata":{"prompt":"p"}}`)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, jimengRelayInfo("jimeng_vgfm_t2v_l20"))
	require.Nil(t, taskErr)
}

func TestJimengActionForTextToVideoModel(t *testing.T) {
	c := jimengContext(`{"model":"jimeng_t2v_v30","duration":5,"metadata":{"prompt":"p"}}`)
	info := jimengRelayInfo("jimeng_t2v_v30")
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionTextGenerate, info.Action)
}

func TestJimengActionUsesMappedTextToVideoModel(t *testing.T) {
	c := jimengContext(`{"model":"alias-jimeng","duration":5,"metadata":{"prompt":"p"}}`)
	c.Set("model_mapping", `{"alias-jimeng":"jimeng_t2v_v30"}`)
	info := jimengRelayInfo("alias-jimeng")
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionTextGenerate, info.Action)
}

func TestJimengActionUsesFinalResolvedReqKey(t *testing.T) {
	c := jimengContext(`{"model":"jimeng_v30","duration":5,"metadata":{"prompt":"p"}}`)
	info := jimengRelayInfo("jimeng_v30")
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	require.Nil(t, taskErr)
	require.Equal(t, constant.TaskActionTextGenerate, info.Action)
}

func TestJimengTextToVideoRejectsMissingEffectivePrompt(t *testing.T) {
	c := jimengContext(`{"model":"jimeng_t2v_v30","duration":5,"metadata":{"duration":5}}`)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, jimengRelayInfo("jimeng_t2v_v30"))
	require.NotNil(t, taskErr)
	require.Equal(t, "invalid_request", taskErr.Code)
}

func TestJimengTextToVideoPromptIsTopLevelUpstreamField(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"metadata prompt wins": {
			body: `{"model":"jimeng_t2v_v30","prompt":"outer ignored","duration":5,"metadata":{"prompt":"real prompt","input":{"prompt":"nested ignored"}}}`,
			want: `"prompt":"real prompt"`,
		},
		"outer prompt fallback": {
			body: `{"model":"jimeng_t2v_v30","prompt":"outer prompt","duration":5,"metadata":{"duration":5}}`,
			want: `"prompt":"outer prompt"`,
		},
		"metadata input prompt fallback": {
			body: `{"model":"jimeng_t2v_v30","prompt":"outer ignored","duration":5,"metadata":{"input":{"prompt":"nested prompt"}}}`,
			want: `"prompt":"nested prompt"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := jimengContext(tc.body)
			info := jimengRelayInfo("jimeng_t2v_v30")
			require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))

			body, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)

			require.Contains(t, string(data), tc.want)
			require.NotContains(t, string(data), `"outer ignored"`)
		})
	}
}

func TestJimengFetchTaskRequiresReqKey(t *testing.T) {
	_, err := (&TaskAdaptor{}).FetchTask("https://example.com", "ak|sk", map[string]any{"task_id": "task"}, "")
	require.ErrorContains(t, err, "missing req_key")
}

func TestJimengParseTaskResultGenerating(t *testing.T) {
	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"code":10000,
		"message":"Success",
		"data":{"status":"generating","video_url":""}
	}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusInProgress, taskInfo.Status)
	require.Equal(t, "30%", taskInfo.Progress)
}

func TestJimengEstimateBillingUsesOuterFieldsOnly(t *testing.T) {
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"jimeng_vgfm_t2v_l20":"per_second"}`,
	}))

	c := jimengContext(`{"model":"jimeng_vgfm_t2v_l20","duration":6,"resolution":"720P","metadata":{"prompt":"p","duration":30,"resolution":"1080P"}}`)
	info := jimengRelayInfo("jimeng_vgfm_t2v_l20")
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))

	ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
	require.Equal(t, float64(6), ratios["seconds"])
	require.NotContains(t, ratios, "resolution-720P")
	require.NotContains(t, ratios, "resolution-1080P")
}
