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
	resolution, err := ResolveVideoBillingResolution(relaycommon.TaskSubmitReq{Resolution: "1080p"}, "720P")
	require.NoError(t, err)
	require.Equal(t, "1080P", resolution)

	resolution, err = ResolveVideoBillingResolution(relaycommon.TaskSubmitReq{}, "720P")
	require.NoError(t, err)
	require.Equal(t, "720P", resolution)

	_, err = ResolveVideoBillingResolution(relaycommon.TaskSubmitReq{Resolution: "4K"}, "720P")
	require.Error(t, err)
}
