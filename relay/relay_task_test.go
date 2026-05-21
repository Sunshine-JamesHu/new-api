package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func withRelayBillingConfig(t *testing.T, values map[string]string) {
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

func TestApplyConfiguredPerSecondMultipliersOverridesRequestFactors(t *testing.T) {
	withRelayBillingConfig(t, map[string]string{
		"billing_setting.billing_mode": `{"video-model":"per_second","ratio-model":"ratio"}`,
		"billing_setting.per_second_multipliers": `{
			"video-model":{
				"resolution-1080P":1.777778,
				"audio":2
			}
		}`,
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "video-model",
		PriceData: types.PriceData{
			OtherRatios: map[string]float64{
				"seconds":          5,
				"resolution-1080P": 1,
				"audio":            1.2,
			},
		},
	}

	applyConfiguredPerSecondMultipliers(info)

	require.Equal(t, 5.0, info.PriceData.OtherRatios["seconds"])
	require.Equal(t, 1.777778, info.PriceData.OtherRatios["resolution-1080P"])
	require.Equal(t, 2.0, info.PriceData.OtherRatios["audio"])

	ratioInfo := &relaycommon.RelayInfo{
		OriginModelName: "ratio-model",
		PriceData: types.PriceData{
			OtherRatios: map[string]float64{"resolution-1080P": 1},
		},
	}

	applyConfiguredPerSecondMultipliers(ratioInfo)

	require.Equal(t, 1.0, ratioInfo.PriceData.OtherRatios["resolution-1080P"])
}
