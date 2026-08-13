package setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	UserViolationBanMaxRules         = 32
	UserViolationBanMaxKeywordLength = 256
	UserViolationBanMaxThreshold     = int64(2147483647)
	UserViolationBanMaxWindowHours   = 24 * 365
)

// UserViolationBanRule describes one upstream error response that contributes
// to a user's violation count.
type UserViolationBanRule struct {
	StatusCode int    `json:"status_code"`
	Keyword    string `json:"keyword"`
}

// UserViolationBanConfig is the in-memory snapshot used by the relay path.
type UserViolationBanConfig struct {
	Enabled     bool
	Rules       []UserViolationBanRule
	Threshold   int64
	WindowHours int
}

var userViolationBanState = struct {
	sync.RWMutex
	config UserViolationBanConfig
}{
	config: UserViolationBanConfig{},
}

func GetUserViolationBanConfig() UserViolationBanConfig {
	userViolationBanState.RLock()
	defer userViolationBanState.RUnlock()

	config := userViolationBanState.config
	config.Rules = append([]UserViolationBanRule(nil), config.Rules...)
	return config
}

func (config UserViolationBanConfig) IsActive() bool {
	return config.Enabled && config.Threshold > 0 && config.WindowHours > 0 && len(config.Rules) > 0
}

func (config UserViolationBanConfig) Matches(statusCode int, responseBody string) bool {
	if !config.IsActive() || responseBody == "" {
		return false
	}

	lowerBody := strings.ToLower(responseBody)
	for _, rule := range config.Rules {
		if rule.StatusCode == statusCode && strings.Contains(lowerBody, strings.ToLower(rule.Keyword)) {
			return true
		}
	}
	return false
}

func UserViolationBanRulesToJSONString() string {
	config := GetUserViolationBanConfig()
	data, err := common.Marshal(config.Rules)
	if err != nil {
		common.SysError("failed to marshal user violation ban rules: " + err.Error())
		return "[]"
	}
	return string(data)
}

func UserViolationBanThresholdString() string {
	return strconv.FormatInt(GetUserViolationBanConfig().Threshold, 10)
}

func UserViolationBanWindowHoursString() string {
	return strconv.Itoa(GetUserViolationBanConfig().WindowHours)
}

func ValidateUserViolationBanRulesJSON(value string) error {
	_, err := parseUserViolationBanRules(value)
	return err
}

func UpdateUserViolationBanRulesByJSONString(value string) error {
	rules, err := parseUserViolationBanRules(value)
	if err != nil {
		return err
	}

	userViolationBanState.Lock()
	userViolationBanState.config.Rules = rules
	userViolationBanState.Unlock()
	return nil
}

func ValidateUserViolationBanEnabled(value string) error {
	_, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("invalid user violation ban enabled value")
	}
	return nil
}

func UpdateUserViolationBanEnabled(value string) error {
	enabled, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("invalid user violation ban enabled value")
	}

	userViolationBanState.Lock()
	userViolationBanState.config.Enabled = enabled
	userViolationBanState.Unlock()
	return nil
}

func ValidateUserViolationBanThreshold(value string) error {
	threshold, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || threshold < 0 || threshold > UserViolationBanMaxThreshold {
		return fmt.Errorf("user violation ban threshold must be between 0 and %d", UserViolationBanMaxThreshold)
	}
	return nil
}

func UpdateUserViolationBanThreshold(value string) error {
	if err := ValidateUserViolationBanThreshold(value); err != nil {
		return err
	}
	threshold, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)

	userViolationBanState.Lock()
	userViolationBanState.config.Threshold = threshold
	userViolationBanState.Unlock()
	return nil
}

func ValidateUserViolationBanWindowHours(value string) error {
	hours, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || hours < 0 || hours > UserViolationBanMaxWindowHours {
		return fmt.Errorf("user violation ban window must be between 0 and %d hours", UserViolationBanMaxWindowHours)
	}
	return nil
}

func UpdateUserViolationBanWindowHours(value string) error {
	if err := ValidateUserViolationBanWindowHours(value); err != nil {
		return err
	}
	hours, _ := strconv.Atoi(strings.TrimSpace(value))

	userViolationBanState.Lock()
	userViolationBanState.config.WindowHours = hours
	userViolationBanState.Unlock()
	return nil
}

func parseUserViolationBanRules(value string) ([]UserViolationBanRule, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var rules []UserViolationBanRule
	if err := common.UnmarshalJsonStr(value, &rules); err != nil {
		return nil, fmt.Errorf("invalid user violation ban rules JSON: %w", err)
	}
	if len(rules) > UserViolationBanMaxRules {
		return nil, fmt.Errorf("user violation ban rules cannot contain more than %d entries", UserViolationBanMaxRules)
	}

	normalized := make([]UserViolationBanRule, 0, len(rules))
	for index, rule := range rules {
		keyword := strings.TrimSpace(rule.Keyword)
		if rule.StatusCode < 100 || rule.StatusCode > 599 {
			return nil, fmt.Errorf("user violation ban rule %d has invalid status code", index+1)
		}
		if keyword == "" {
			return nil, fmt.Errorf("user violation ban rule %d keyword cannot be empty", index+1)
		}
		if len([]byte(keyword)) > UserViolationBanMaxKeywordLength {
			return nil, fmt.Errorf("user violation ban rule %d keyword is too long", index+1)
		}
		normalized = append(normalized, UserViolationBanRule{
			StatusCode: rule.StatusCode,
			Keyword:    keyword,
		})
	}
	return normalized, nil
}
