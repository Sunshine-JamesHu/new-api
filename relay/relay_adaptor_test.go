package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/happyhorse"
	"github.com/stretchr/testify/require"
)

func TestGetTaskAdaptorUsesHappyHorseChannelType(t *testing.T) {
	require.Less(t, constant.ChannelTypeHappyHorse, constant.ChannelTypeDummy)

	adaptor := GetTaskAdaptor(constant.TaskPlatform("998"))
	require.IsType(t, &happyhorse.TaskAdaptor{}, adaptor)
	require.Equal(t, "happyhorse", adaptor.GetChannelName())
}
