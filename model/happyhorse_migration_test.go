package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestMigrateHappyHorseChannelTypeMigratesLegacy999(t *testing.T) {
	truncateTables(t)

	channel := &Channel{
		Type:   999,
		Key:    "sk-legacy",
		Name:   "legacy-ali-aigc",
		Status: 1,
	}
	require.NoError(t, DB.Create(channel).Error)

	require.NoError(t, migrateHappyHorseChannelType())

	var migrated Channel
	require.NoError(t, DB.First(&migrated, channel.Id).Error)
	require.Equal(t, constant.ChannelTypeHappyHorse, migrated.Type)
	require.Equal(t, "sk-legacy", migrated.Key)
	require.Equal(t, "legacy-ali-aigc", migrated.Name)
}
