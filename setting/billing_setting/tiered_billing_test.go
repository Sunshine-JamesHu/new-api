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
