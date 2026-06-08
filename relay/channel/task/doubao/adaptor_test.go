package doubao

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withDoubaoBillingConfig(t *testing.T, values map[string]string) {
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

func doubaoTestContext(body string) *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

func setDoubaoTaskRequest(t *testing.T, c *gin.Context) {
	t.Helper()
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	taskErr := relaycommon.ValidateMultipartDirect(c, info)
	require.Nil(t, taskErr)
}

func doubaoRelayInfo(model string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: model,
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: model,
		},
	}
}

func TestEstimateBillingSkipsNonPerSecondModelWithoutVideoInput(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{}`,
	})

	c := doubaoTestContext(`{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":7}`)
	setDoubaoTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))
	require.Nil(t, got)
}

func TestEstimateBillingUsesDefaultDuration(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"doubao-seedance-2-0-260128":"per_second"}`,
	})

	c := doubaoTestContext(`{"model":"doubao-seedance-2-0-260128","prompt":"hello"}`)
	setDoubaoTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))
	require.Equal(t, map[string]float64{"seconds": 5}, got)
}

func TestEstimateBillingParsesTopLevelDurationAndSeconds(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"doubao-seedance-2-0-260128":"per_second"}`,
	})

	for name, tc := range map[string]struct {
		body string
		want float64
	}{
		"duration number": {body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":10}`, want: 10},
		"duration string": {body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":"8"}`, want: 8},
		"seconds number":  {body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","seconds":9}`, want: 9},
		"seconds string":  {body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","seconds":"7.2"}`, want: 8},
	} {
		t.Run(name, func(t *testing.T) {
			c := doubaoTestContext(tc.body)
			setDoubaoTaskRequest(t, c)

			got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))

			require.Equal(t, tc.want, got["seconds"])
		})
	}
}

func TestEstimateBillingIgnoresMetadataDuration(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"doubao-seedance-2-0-260128":"per_second"}`,
	})

	c := doubaoTestContext(`{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":5,"metadata":{"duration":12}}`)
	setDoubaoTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))
	require.Equal(t, float64(5), got["seconds"])
}

func TestEstimateBillingCombinesVideoInputAndSeconds(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"doubao-seedance-2-0-260128":"per_second"}`,
	})

	c := doubaoTestContext(`{
		"model":"doubao-seedance-2-0-260128",
		"prompt":"hello",
		"duration":6,
		"metadata":{"content":[{"type":"video_url","video_url":{"url":"https://example.com/in.mp4"}}]}
	}`)
	setDoubaoTaskRequest(t, c)

	got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))

	require.Equal(t, float64(6), got["seconds"])
	require.InEpsilon(t, 28.0/46.0, got["video_input"], 0.000001)
}

func TestBuildRequestBodyUsesTrustedTopLevelDuration(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want dto.IntValue
	}{
		"duration": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":6}`,
			want: dto.IntValue(6),
		},
		"seconds ceil": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","seconds":"7.2"}`,
			want: dto.IntValue(8),
		},
		"metadata ignored": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":6,"metadata":{"duration":12}}`,
			want: dto.IntValue(6),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := doubaoTestContext(tc.body)
			setDoubaoTaskRequest(t, c)

			reader, err := (&TaskAdaptor{}).BuildRequestBody(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))
			require.NoError(t, err)
			data, err := io.ReadAll(reader)
			require.NoError(t, err)

			var payload requestPayload
			require.NoError(t, common.Unmarshal(data, &payload))
			require.NotNil(t, payload.Duration)
			require.Equal(t, tc.want, *payload.Duration)
		})
	}
}
