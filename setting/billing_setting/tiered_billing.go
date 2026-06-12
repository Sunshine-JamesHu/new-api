package billing_setting

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/samber/lo"
)

const (
	BillingModeRatio          = "ratio"
	BillingModeTieredExpr     = "tiered_expr"
	BillingModePerSecond      = "per_second"
	BillingModeField          = "billing_mode"
	BillingExprField          = "billing_expr"
	PerSecondMultipliersField = "per_second_multipliers"
)

// BillingSetting is managed by config.GlobalConfig.Register.
// DB keys: billing_setting.billing_mode, billing_setting.billing_expr,
// billing_setting.per_second_multipliers.
type BillingSetting struct {
	BillingMode          map[string]string             `json:"billing_mode"`
	BillingExpr          map[string]string             `json:"billing_expr"`
	PerSecondMultipliers map[string]map[string]float64 `json:"per_second_multipliers"`
}

var billingSetting = BillingSetting{
	BillingMode:          make(map[string]string),
	BillingExpr:          make(map[string]string),
	PerSecondMultipliers: make(map[string]map[string]float64),
}

func init() {
	config.GlobalConfig.Register("billing_setting", &billingSetting)
}

// ---------------------------------------------------------------------------
// Read accessors (hot path, must be fast)
// ---------------------------------------------------------------------------

func GetBillingMode(model string) string {
	if mode, ok := billingSetting.BillingMode[model]; ok {
		return mode
	}
	return BillingModeRatio
}

func IsPerSecondBilling(model string) bool {
	return GetBillingMode(model) == BillingModePerSecond
}

func GetBillingExpr(model string) (string, bool) {
	expr, ok := billingSetting.BillingExpr[model]
	return expr, ok
}

func GetBillingModeCopy() map[string]string {
	return lo.Assign(billingSetting.BillingMode)
}

func GetBillingExprCopy() map[string]string {
	return lo.Assign(billingSetting.BillingExpr)
}

func validPerSecondMultiplier(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func NormalizePerSecondMultiplierKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}

	const prefix = "resolution-"
	if len(trimmed) <= len(prefix) || !strings.EqualFold(trimmed[:len(prefix)], prefix) {
		return trimmed
	}

	value := strings.ToLower(strings.TrimSpace(trimmed[len(prefix):]))
	value = strings.TrimSuffix(value, "p")
	switch value {
	case "480", "720", "1080":
		return prefix + value + "P"
	default:
		return trimmed
	}
}

func canonicalResolutionMultiplierKey(key string) bool {
	switch key {
	case "resolution-480P", "resolution-720P", "resolution-1080P":
		return true
	default:
		return false
	}
}

func NormalizePerSecondMultipliers(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}

	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	type normalizedValue struct {
		value    float64
		priority int
	}
	normalized := make(map[string]normalizedValue, len(src))
	for _, key := range keys {
		value := src[key]
		normalizedKey := NormalizePerSecondMultiplierKey(key)
		if normalizedKey == "" || !validPerSecondMultiplier(value) {
			continue
		}

		priority := 1
		if canonicalResolutionMultiplierKey(normalizedKey) && key == normalizedKey {
			priority = 2
		}
		if existing, ok := normalized[normalizedKey]; ok && existing.priority > priority {
			continue
		}
		normalized[normalizedKey] = normalizedValue{value: value, priority: priority}
	}

	if len(normalized) == 0 {
		return nil
	}

	cleaned := make(map[string]float64, len(normalized))
	for key, item := range normalized {
		cleaned[key] = item.value
	}
	return cleaned
}

func sanitizePerSecondMultipliers(src map[string]map[string]float64) map[string]map[string]float64 {
	cleaned := make(map[string]map[string]float64)
	for model, multipliers := range src {
		if model == "" || len(multipliers) == 0 {
			continue
		}
		modelMultipliers := NormalizePerSecondMultipliers(multipliers)
		if len(modelMultipliers) > 0 {
			cleaned[model] = modelMultipliers
		}
	}
	return cleaned
}

func GetPerSecondMultipliers(model string) map[string]float64 {
	multipliers, ok := sanitizePerSecondMultipliers(billingSetting.PerSecondMultipliers)[model]
	if !ok {
		return nil
	}
	return lo.Assign(multipliers)
}

func GetPerSecondMultiplier(model, key string) (float64, bool) {
	multipliers := GetPerSecondMultipliers(model)
	if len(multipliers) == 0 {
		return 0, false
	}
	normalizedKey := NormalizePerSecondMultiplierKey(key)
	if value, ok := multipliers[normalizedKey]; ok {
		return value, true
	}
	for configuredKey, value := range multipliers {
		if strings.EqualFold(configuredKey, normalizedKey) || strings.EqualFold(configuredKey, key) {
			return value, true
		}
	}
	return 0, false
}

func GetPerSecondMultipliersCopy() map[string]map[string]float64 {
	return sanitizePerSecondMultipliers(billingSetting.PerSecondMultipliers)
}

func GetPricingSyncData(base map[string]any) map[string]any {
	extra := make(map[string]any, 3)
	if modes := GetBillingModeCopy(); len(modes) > 0 {
		extra[BillingModeField] = modes
	}
	if exprs := GetBillingExprCopy(); len(exprs) > 0 {
		extra[BillingExprField] = exprs
	}
	if multipliers := GetPerSecondMultipliersCopy(); len(multipliers) > 0 {
		extra[PerSecondMultipliersField] = multipliers
	}
	return lo.Assign(base, extra)
}

// ---------------------------------------------------------------------------
// Smoke test (called externally for validation before save)
// ---------------------------------------------------------------------------

func SmokeTestExpr(exprStr string) error {
	return smokeTestExpr(exprStr)
}

func smokeTestExpr(exprStr string) error {
	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}
	requests := []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}

	for _, v := range vectors {
		for _, request := range requests {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: result %f < 0", v.P, v.C, result)
			}
		}
	}
	return nil
}
