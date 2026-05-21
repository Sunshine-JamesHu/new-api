package billing_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func withBillingConfig(t *testing.T, values map[string]string) {
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

func TestPerSecondBillingMode(t *testing.T) {
	withBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"video-model":"per_second","expr-model":"tiered_expr"}`,
	})

	require.Equal(t, BillingModePerSecond, GetBillingMode("video-model"))
	require.True(t, IsPerSecondBilling("video-model"))
	require.False(t, IsPerSecondBilling("expr-model"))
	require.Equal(t, BillingModeRatio, GetBillingMode("missing-model"))
}

func TestGetBillingModeCopyIncludesPerSecond(t *testing.T) {
	withBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"video-model":"per_second"}`,
	})

	copied := GetBillingModeCopy()
	require.Equal(t, BillingModePerSecond, copied["video-model"])

	copied["video-model"] = BillingModeRatio
	require.Equal(t, BillingModePerSecond, GetBillingMode("video-model"))
}

func TestPerSecondMultipliers(t *testing.T) {
	withBillingConfig(t, map[string]string{
		"billing_setting.per_second_multipliers": `{
			"video-model":{
				"resolution-720P":1,
				"resolution-1080P":1.777778,
				"zero":0,
				"negative":-1
			}
		}`,
	})

	multipliers := GetPerSecondMultipliers("video-model")
	require.Equal(t, 1.0, multipliers["resolution-720P"])
	require.Equal(t, 1.777778, multipliers["resolution-1080P"])
	require.NotContains(t, multipliers, "zero")
	require.NotContains(t, multipliers, "negative")

	multipliers["resolution-720P"] = 9
	value, ok := GetPerSecondMultiplier("video-model", "resolution-720P")
	require.True(t, ok)
	require.Equal(t, 1.0, value)
}
