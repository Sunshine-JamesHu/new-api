package kling

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

func withKlingBillingConfig(t *testing.T, values map[string]string) {
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

func klingTestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

func setKlingTaskRequest(t *testing.T, c *gin.Context) {
	t.Helper()
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := relaycommon.ValidateMultipartDirect(c, info)
	require.Nil(t, taskErr)
}

func klingRelayInfo(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: model,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: model,
		},
	}
}

func TestEstimateBillingSkipsNonPerSecondModel(t *testing.T) {
	withKlingBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{}`,
	})

	c := klingTestContext(`{"model":"kling-v1","prompt":"hello","duration":7}`)
	setKlingTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, klingRelayInfo("kling-v1"))
	require.Nil(t, got)
}

func TestEstimateBillingUsesDefaultDuration(t *testing.T) {
	withKlingBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"kling-v1":"per_second"}`,
	})

	c := klingTestContext(`{"model":"kling-v1","prompt":"hello"}`)
	setKlingTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, klingRelayInfo("kling-v1"))
	require.Equal(t, map[string]float64{"seconds": 5}, got)
}

func TestEstimateBillingParsesNumericAndStringDuration(t *testing.T) {
	withKlingBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"kling-v1":"per_second"}`,
	})

	for name, body := range map[string]string{
		"number":  `{"model":"kling-v1","prompt":"hello","duration":10}`,
		"string":  `{"model":"kling-v1","prompt":"hello","duration":"8"}`,
		"seconds": `{"model":"kling-v1","prompt":"hello","seconds":9}`,
	} {
		t.Run(name, func(t *testing.T) {
			c := klingTestContext(body)
			setKlingTaskRequest(t, c)

			got := (&TaskAdaptor{}).EstimateBilling(c, klingRelayInfo("kling-v1"))

			if name == "number" {
				require.Equal(t, float64(10), got["seconds"])
			} else if name == "string" {
				require.Equal(t, float64(8), got["seconds"])
			} else {
				require.Equal(t, float64(9), got["seconds"])
			}
		})
	}
}

func TestEstimateBillingUsesMetadataDuration(t *testing.T) {
	withKlingBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"kling-v1":"per_second"}`,
	})

	c := klingTestContext(`{"model":"kling-v1","prompt":"hello","duration":5,"metadata":{"duration":12}}`)
	setKlingTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, klingRelayInfo("kling-v1"))
	require.Equal(t, float64(12), got["seconds"])
}

func TestBuildRequestBodySupportsMetadataDurationString(t *testing.T) {
	c := klingTestContext(`{"model":"kling-v1","prompt":"hello","metadata":{"duration":"12"}}`)
	setKlingTaskRequest(t, c)

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, klingRelayInfo("kling-v1"))
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(data), `"duration":"12"`)
}
