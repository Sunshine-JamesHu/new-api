package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelType2APITypeKeepsTaskOnlyVideoChannelsUnsupported(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
	}{
		{name: "happy horse", channelType: constant.ChannelTypeHappyHorse},
		{name: "new api video", channelType: constant.ChannelTypeNewApiVideo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiType, ok := ChannelType2APIType(tt.channelType)
			require.False(t, ok)
			require.Equal(t, -1, apiType)
		})
	}
}

func TestChannelType2APITypeFallbacksToOpenAIForCompatibleChannels(t *testing.T) {
	apiType, ok := ChannelType2APIType(constant.ChannelTypeAzure)
	require.False(t, ok)
	require.Equal(t, constant.APITypeOpenAI, apiType)
}
