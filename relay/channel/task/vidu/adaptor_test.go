package vidu

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestViduNormalizesResolutionForUpstream(t *testing.T) {
	for name, tc := range map[string]struct {
		req  relaycommon.TaskSubmitReq
		want string
	}{
		"top level 480 numeric": {
			req: relaycommon.TaskSubmitReq{
				Prompt:     "scene",
				Resolution: "480",
				Metadata:   map[string]any{"resolution": "1080P"},
			},
			want: "480p",
		},
		"top level 720 upper": {
			req: relaycommon.TaskSubmitReq{
				Prompt:     "scene",
				Size:       "480",
				Resolution: "720P",
				Metadata:   map[string]any{"resolution": "1080P"},
			},
			want: "720p",
		},
		"metadata 1080 lower": {
			req: relaycommon.TaskSubmitReq{
				Prompt:   "scene",
				Size:     "480",
				Metadata: map[string]any{"resolution": "1080p"},
			},
			want: "1080p",
		},
		"size 480": {
			req: relaycommon.TaskSubmitReq{
				Prompt: "scene",
				Size:   "480",
			},
			want: "480p",
		},
		"default 720": {
			req: relaycommon.TaskSubmitReq{
				Prompt: "scene",
			},
			want: "720p",
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := (&TaskAdaptor{}).convertToRequestPayload(&tc.req, &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "viduq1"},
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, req.Resolution)
		})
	}
}

func TestViduUsesEffectivePrompt(t *testing.T) {
	req, err := (&TaskAdaptor{}).convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "outer prompt",
		Metadata: map[string]any{
			"input": map[string]any{"prompt": "inner prompt"},
		},
	}, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "viduq1"},
	})

	require.NoError(t, err)
	require.Equal(t, "inner prompt", req.Prompt)
}
