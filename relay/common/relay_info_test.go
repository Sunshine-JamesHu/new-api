package common

import (
	"testing"

	commonjson "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestRelayInfoGetFinalRequestRelayFormatPrefersExplicitFinal(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		RequestConversionChain:  []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
		FinalRequestRelayFormat: types.RelayFormatOpenAIResponses,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatOpenAIResponses), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToConversionChain(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:            types.RelayFormatOpenAI,
		RequestConversionChain: []types.RelayFormat{types.RelayFormatOpenAI, types.RelayFormatClaude},
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatClaude), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatFallsBackToRelayFormat(t *testing.T) {
	info := &RelayInfo{
		RelayFormat: types.RelayFormatGemini,
	}

	require.Equal(t, types.RelayFormat(types.RelayFormatGemini), info.GetFinalRequestRelayFormat())
}

func TestRelayInfoGetFinalRequestRelayFormatNilReceiver(t *testing.T) {
	var info *RelayInfo
	require.Equal(t, types.RelayFormat(""), info.GetFinalRequestRelayFormat())
}

func TestTaskSubmitReqUnmarshalDurationAndSecondsVariants(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantDuration int
		wantSeconds  string
	}{
		{
			name:         "numeric duration and seconds",
			body:         `{"prompt":"p","model":"m","duration":7.2,"seconds":9}`,
			wantDuration: 8,
			wantSeconds:  "9",
		},
		{
			name:         "string duration and seconds",
			body:         `{"prompt":"p","model":"m","duration":"6.1","seconds":"8.5"}`,
			wantDuration: 7,
			wantSeconds:  "8.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req TaskSubmitReq
			require.NoError(t, commonjson.Unmarshal([]byte(tt.body), &req))
			require.Equal(t, tt.wantDuration, req.Duration)
			require.Equal(t, tt.wantSeconds, req.Seconds)
		})
	}
}

func TestTaskSubmitReqUnmarshalResolution(t *testing.T) {
	var req TaskSubmitReq
	require.NoError(t, commonjson.Unmarshal([]byte(`{"prompt":"p","model":"m","resolution":"1080p"}`), &req))
	require.Equal(t, "1080p", req.Resolution)
}
