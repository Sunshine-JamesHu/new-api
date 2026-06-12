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
	require.Equal(t, map[string]float64{"seconds": 5, "resolution-720P": 1}, got)
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
			require.Equal(t, float64(1), got["resolution-720P"])
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
	require.Equal(t, float64(1), got["resolution-720P"])
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
	require.Equal(t, float64(1), got["resolution-720P"])
	require.InEpsilon(t, 28.0/46.0, got["video_input"], 0.000001)
}

func TestEstimateBillingIncludesNormalizedResolutionTier(t *testing.T) {
	withDoubaoBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"doubao-seedance-2-0-260128":"per_second"}`,
	})

	for name, tc := range map[string]struct {
		body string
		key  string
	}{
		"top level 480 numeric": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"480"}`,
			key:  "resolution-480P",
		},
		"top level 480P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"480P"}`,
			key:  "resolution-480P",
		},
		"top level 720 numeric": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"720"}`,
			key:  "resolution-720P",
		},
		"top level 720p": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"720p"}`,
			key:  "resolution-720P",
		},
		"top level 1080P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"1080P","metadata":{"resolution":"720P"}}`,
			key:  "resolution-1080P",
		},
		"top level 720P overrides metadata 1080P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"720P","metadata":{"resolution":"1080P"}}`,
			key:  "resolution-720P",
		},
		"metadata 720P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"metadata":{"resolution":"720P"}}`,
			key:  "resolution-720P",
		},
		"metadata 480p": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"metadata":{"resolution":"480p"}}`,
			key:  "resolution-480P",
		},
		"top level numeric 1080": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"1080"}`,
			key:  "resolution-1080P",
		},
		"default": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4}`,
			key:  "resolution-720P",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := doubaoTestContext(tc.body)
			setDoubaoTaskRequest(t, c)

			got := (&TaskAdaptor{}).EstimateBilling(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))

			require.Equal(t, float64(4), got["seconds"])
			require.Equal(t, float64(1), got[tc.key])
		})
	}
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

func TestBuildRequestBodyNormalizesResolution(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"top level 720P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"720P","metadata":{"resolution":"1080P"}}`,
			want: "720p",
		},
		"top level 720 numeric": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"720"}`,
			want: "720p",
		},
		"metadata 480P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"metadata":{"resolution":"480P"}}`,
			want: "480p",
		},
		"already lowercase": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"metadata":{"resolution":"720p"}}`,
			want: "720p",
		},
		"1080P": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"1080P"}`,
			want: "1080p",
		},
		"numeric 480": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"480"}`,
			want: "480p",
		},
		"uppercase 480": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"resolution":"480P"}`,
			want: "480p",
		},
		"size 1080p": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4,"size":"1080p"}`,
			want: "1080p",
		},
		"default": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"hello","duration":4}`,
			want: "720p",
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
			require.Equal(t, tc.want, payload.Resolution)
		})
	}
}

func TestBuildRequestBodyUsesEffectivePrompt(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"metadata content text": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"outer","metadata":{"content":[{"type":"text","text":"inner"}]}}`,
			want: "inner",
		},
		"outer prompt fallback": {
			body: `{"model":"doubao-seedance-2-0-260128","prompt":"outer"}`,
			want: "outer",
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
			require.Len(t, payload.Content, 1)
			require.Equal(t, tc.want, payload.Content[0].Text)
		})
	}
}

func TestBuildRequestBodyPreservesMetadataGenerateAudioFalse(t *testing.T) {
	c := doubaoTestContext(`{
		"model":"doubao-seedance-2-0-260128",
		"prompt":"hello",
		"duration":4,
		"metadata":{"generate_audio":false}
	}`)
	setDoubaoTaskRequest(t, c)

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, doubaoRelayInfo("doubao-seedance-2-0-260128"))
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)

	var payload requestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.NotNil(t, payload.GenerateAudio)
	require.Equal(t, dto.BoolValue(false), *payload.GenerateAudio)
}
