package gemini

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiTestContext(t *testing.T, req relaycommon.TaskSubmitReq) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(nil))
	c.Set("task_request", req)
	return c
}

func TestBuildRequestBodyNormalizesResolutionForUpstream(t *testing.T) {
	for name, tc := range map[string]struct {
		req  relaycommon.TaskSubmitReq
		want string
	}{
		"top level 480 numeric": {
			req: relaycommon.TaskSubmitReq{
				Model:      "veo-3.1-generate-preview",
				Prompt:     "scene",
				Resolution: "480",
				Metadata:   map[string]any{"resolution": "1080P"},
			},
			want: "480p",
		},
		"top level 720 lower": {
			req: relaycommon.TaskSubmitReq{
				Model:      "veo-3.1-generate-preview",
				Prompt:     "scene",
				Resolution: "720p",
			},
			want: "720p",
		},
		"metadata 1080 upper": {
			req: relaycommon.TaskSubmitReq{
				Model:    "veo-3.1-generate-preview",
				Prompt:   "scene",
				Size:     "720p",
				Metadata: map[string]any{"resolution": "1080P"},
			},
			want: "1080p",
		},
		"size 480 upper": {
			req: relaycommon.TaskSubmitReq{
				Model:  "veo-3.1-generate-preview",
				Prompt: "scene",
				Size:   "480P",
			},
			want: "480p",
		},
		"default empty": {
			req: relaycommon.TaskSubmitReq{
				Model:  "veo-3.1-generate-preview",
				Prompt: "scene",
			},
			want: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			body, err := (&TaskAdaptor{}).BuildRequestBody(geminiTestContext(t, tc.req), &relaycommon.RelayInfo{
				TaskRelayInfo: &relaycommon.TaskRelayInfo{},
			})
			require.NoError(t, err)
			data, err := io.ReadAll(body)
			require.NoError(t, err)

			var payload VeoRequestPayload
			require.NoError(t, common.Unmarshal(data, &payload))
			require.NotNil(t, payload.Parameters)
			require.Equal(t, tc.want, payload.Parameters.Resolution)
		})
	}
}

func TestBuildRequestBodyUsesEffectivePrompt(t *testing.T) {
	body, err := (&TaskAdaptor{}).BuildRequestBody(geminiTestContext(t, relaycommon.TaskSubmitReq{
		Model:  "veo-3.1-generate-preview",
		Prompt: "outer prompt",
		Metadata: map[string]any{
			"prompt": "metadata prompt",
		},
	}), &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload VeoRequestPayload
	require.NoError(t, common.Unmarshal(data, &payload))
	require.Len(t, payload.Instances, 1)
	require.Equal(t, "metadata prompt", payload.Instances[0].Prompt)
}
