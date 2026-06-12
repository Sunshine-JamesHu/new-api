package hailuo

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestHailuoNormalizesResolutionForUpstream(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "I2V-01-Director"},
	}

	for name, tc := range map[string]struct {
		req  relaycommon.TaskSubmitReq
		want string
	}{
		"top level 1080 lower": {
			req: relaycommon.TaskSubmitReq{
				Prompt:     "scene",
				Size:       "720p",
				Resolution: "1080p",
				Metadata:   map[string]any{"resolution": "720P"},
			},
			want: "1080P",
		},
		"top level 720 numeric": {
			req: relaycommon.TaskSubmitReq{
				Prompt:     "scene",
				Resolution: "720",
				Metadata:   map[string]any{"resolution": "1080P"},
			},
			want: "720P",
		},
		"metadata unsupported 480 falls back": {
			req: relaycommon.TaskSubmitReq{
				Prompt:   "scene",
				Metadata: map[string]any{"resolution": "480p"},
			},
			want: "720P",
		},
		"size 1080 upper": {
			req: relaycommon.TaskSubmitReq{
				Prompt: "scene",
				Size:   "1080P",
			},
			want: "1080P",
		},
		"default 720": {
			req: relaycommon.TaskSubmitReq{
				Prompt: "scene",
			},
			want: "720P",
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := (&TaskAdaptor{}).convertToRequestPayload(&tc.req, info)
			require.NoError(t, err)
			require.Equal(t, tc.want, req.Resolution)
		})
	}
}

func TestHailuoUsesEffectivePrompt(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "I2V-01-Director"},
	}
	req, err := (&TaskAdaptor{}).convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "outer prompt",
		Metadata: map[string]any{
			"input": map[string]any{"prompt": "inner prompt"},
		},
	}, info)

	require.NoError(t, err)
	require.Equal(t, "inner prompt", req.Prompt)
}
