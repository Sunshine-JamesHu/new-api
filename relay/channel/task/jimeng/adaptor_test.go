package jimeng

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestJimengIgnoresOuterResolution(t *testing.T) {
	c := jimengContext(`{"model":"jimeng_vgfm_t2v_l20","prompt":"p","resolution":"4K"}`)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, jimengRelayInfo("jimeng_vgfm_t2v_l20"))
	require.Nil(t, taskErr)
}

func TestJimengFetchTaskRequiresReqKey(t *testing.T) {
	_, err := (&TaskAdaptor{}).FetchTask("https://example.com", "ak|sk", map[string]any{"task_id": "task"}, "")
	require.ErrorContains(t, err, "missing req_key")
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

	c := jimengContext(`{"model":"jimeng_vgfm_t2v_l20","prompt":"p","duration":6,"resolution":"720P","metadata":{"duration":30,"resolution":"1080P"}}`)
	info := jimengRelayInfo("jimeng_vgfm_t2v_l20")
	require.Nil(t, (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info))

	ratios := (&TaskAdaptor{}).EstimateBilling(c, info)
	require.Equal(t, float64(6), ratios["seconds"])
	require.NotContains(t, ratios, "resolution-720P")
	require.NotContains(t, ratios, "resolution-1080P")
}
