package setting

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useUserViolationBanConfig(t *testing.T) {
	t.Helper()
	previous := GetUserViolationBanConfig()
	previousRules := UserViolationBanRulesToJSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateUserViolationBanRulesByJSONString(previousRules))
		require.NoError(t, UpdateUserViolationBanThreshold(strconv.FormatInt(previous.Threshold, 10)))
		require.NoError(t, UpdateUserViolationBanWindowHours(strconv.Itoa(previous.WindowHours)))
		require.NoError(t, UpdateUserViolationBanEnabled(strconv.FormatBool(previous.Enabled)))
	})
}

func TestUserViolationBanRulesMatchConfiguredResponse(t *testing.T) {
	useUserViolationBanConfig(t)
	require.NoError(t, UpdateUserViolationBanRulesByJSONString(`[
  {"status_code": 502, "keyword": "upstream unavailable"},
  {"status_code": 403, "keyword": "account blocked"}
]`))
	require.NoError(t, UpdateUserViolationBanThreshold("3"))
	require.NoError(t, UpdateUserViolationBanWindowHours("6"))
	require.NoError(t, UpdateUserViolationBanEnabled("true"))

	config := GetUserViolationBanConfig()
	assert.True(t, config.IsActive())
	assert.True(t, config.Matches(502, `{"message":"UPSTREAM UNAVAILABLE"}`))
	assert.True(t, config.Matches(403, `{"error":"account blocked by provider"}`))
	assert.False(t, config.Matches(502, `{"message":"temporary failure"}`))
	assert.False(t, config.Matches(500, `{"message":"upstream unavailable"}`))
}

func TestUserViolationBanRulesRejectInvalidEntries(t *testing.T) {
	tests := []string{
		`[{"status_code":99,"keyword":"blocked"}]`,
		`[{"status_code":502,"keyword":"   "}]`,
		`{"status_code":502,"keyword":"blocked"}`,
	}

	for _, value := range tests {
		assert.Error(t, ValidateUserViolationBanRulesJSON(value), value)
	}
	assert.Error(t, ValidateUserViolationBanThreshold("-1"))
	assert.Error(t, ValidateUserViolationBanWindowHours("-1"))
}
