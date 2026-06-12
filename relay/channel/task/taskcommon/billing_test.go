package taskcommon

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestResolveVideoBillingDurationUsesOuterFieldsOnly(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Duration: 6,
		Seconds:  "9",
		Metadata: map[string]any{"duration": 30},
	}

	require.Equal(t, 6, ResolveVideoBillingDuration(req, 5))

	req.Duration = 0
	require.Equal(t, 9, ResolveVideoBillingDuration(req, 5))

	req.Seconds = ""
	require.Equal(t, 5, ResolveVideoBillingDuration(req, 5))
}

func TestResolveVideoBillingResolution(t *testing.T) {
	for name, tc := range map[string]struct {
		req  relaycommon.TaskSubmitReq
		want string
	}{
		"top level 480 numeric": {req: relaycommon.TaskSubmitReq{Resolution: "480"}, want: "480P"},
		"top level 480 lower":   {req: relaycommon.TaskSubmitReq{Resolution: "480p"}, want: "480P"},
		"top level 480 upper":   {req: relaycommon.TaskSubmitReq{Resolution: "480P"}, want: "480P"},
		"top level 720 numeric": {req: relaycommon.TaskSubmitReq{Resolution: "720"}, want: "720P"},
		"top level 720 lower":   {req: relaycommon.TaskSubmitReq{Resolution: "720p"}, want: "720P"},
		"top level 720 upper":   {req: relaycommon.TaskSubmitReq{Resolution: "720P"}, want: "720P"},
		"top level 1080 numeric": {
			req:  relaycommon.TaskSubmitReq{Resolution: "1080"},
			want: "1080P",
		},
		"top level 1080 lower": {req: relaycommon.TaskSubmitReq{Resolution: "1080p"}, want: "1080P"},
		"top level 1080 upper": {req: relaycommon.TaskSubmitReq{Resolution: "1080P"}, want: "1080P"},
		"size fallback":        {req: relaycommon.TaskSubmitReq{Size: "1080p"}, want: "1080P"},
		"default":              {req: relaycommon.TaskSubmitReq{}, want: "720P"},
	} {
		t.Run(name, func(t *testing.T) {
			resolution, err := ResolveVideoBillingResolution(tc.req, "720P")
			require.NoError(t, err)
			require.Equal(t, tc.want, resolution)
		})
	}

	_, err := ResolveVideoBillingResolution(relaycommon.TaskSubmitReq{Resolution: "4K"}, "720P")
	require.Error(t, err)
}
