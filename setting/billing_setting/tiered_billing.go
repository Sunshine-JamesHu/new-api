package billing_setting

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
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
	maxTaskExprSmokeTests     = 64
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
		if modelMultipliers := NormalizePerSecondMultipliers(multipliers); len(modelMultipliers) > 0 {
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
	if _, err := billingexpr.CompileFromCache(exprStr); err != nil {
		return err
	}
	usageKeys := billingexpr.UsedUsageKeys(exprStr)
	if len(usageKeys) > 0 {
		sortedKeys := make([]string, 0, len(usageKeys))
		for key := range usageKeys {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)
		return fmt.Errorf("expression references usage keys %v but the model has no task plugin usage schema", sortedKeys)
	}

	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}

	for _, v := range vectors {
		for _, request := range billingExprSmokeRequests() {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: result must be finite and non-negative, got %f", v.P, v.C, result)
			}
		}
	}
	return nil
}

// SmokeTestTaskExpr validates a task usage expression against the usage facts
// declared by its plugin. Literal u() keys must be declared; dynamic calls are
// still exercised by the generated runtime vectors when possible.
func SmokeTestTaskExpr(exprStr string, schema map[string]jsplugin.UsageFieldSchema) error {
	if _, err := billingexpr.CompileFromCache(exprStr); err != nil {
		return err
	}
	for key := range billingexpr.UsedUsageKeys(exprStr) {
		if _, declared := schema[key]; !declared {
			return fmt.Errorf("usage key %q is not declared by the task plugin", key)
		}
	}

	for _, usage := range taskUsageSmokeVectors(schema) {
		for _, request := range billingExprSmokeRequests() {
			request.Usage = usage
			result, _, err := billingexpr.RunExprWithRequest(exprStr, billingexpr.TokenParams{}, request)
			if err != nil {
				return fmt.Errorf("usage vector %v: run failed: %w", usage, err)
			}
			if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 {
				return fmt.Errorf("usage vector %v: result must be finite and non-negative, got %f", usage, result)
			}
		}
	}
	return nil
}

type usageSmokeDimension struct {
	name   string
	values []any
}

func taskUsageSmokeVectors(schema map[string]jsplugin.UsageFieldSchema) []map[string]any {
	names := make([]string, 0, len(schema))
	for name := range schema {
		names = append(names, name)
	}
	sort.Strings(names)

	dimensions := make([]usageSmokeDimension, 0, len(names))
	for _, name := range names {
		field := schema[name]
		if len(field.Enum) > 0 {
			values := make([]any, len(field.Enum))
			for index, value := range field.Enum {
				values[index] = value
			}
			dimensions = append(dimensions, usageSmokeDimension{name: name, values: values})
			continue
		}
		if field.Type == "boolean" {
			dimensions = append(dimensions, usageSmokeDimension{name: name, values: []any{false, true}})
			continue
		}
		limit := relaycommon.MaxTaskDurationSeconds
		if field.Unit == "count" {
			limit = dto.MaxImageN
		}
		if field.Unit == "token" || field.Unit == "credit" {
			limit = common.MaxQuota
		}
		dimensions = append(dimensions, usageSmokeDimension{
			name:   name,
			values: []any{float64(0), float64(1), float64(limit)},
		})
	}

	if usageSmokeCombinationCount(dimensions, maxTaskExprSmokeTests) > maxTaskExprSmokeTests {
		for index := range dimensions {
			field := schema[dimensions[index].name]
			if len(field.Enum) <= 2 {
				continue
			}
			dimensions[index].values = []any{field.Enum[0], field.Enum[len(field.Enum)-1]}
		}
	}

	vectors := make([]map[string]any, 0, maxTaskExprSmokeTests)
	var appendVectors func(int, map[string]any)
	appendVectors = func(index int, current map[string]any) {
		if len(vectors) >= maxTaskExprSmokeTests {
			return
		}
		if index == len(dimensions) {
			vector := make(map[string]any, len(current))
			for key, value := range current {
				vector[key] = value
			}
			vectors = append(vectors, vector)
			return
		}
		for _, value := range dimensions[index].values {
			current[dimensions[index].name] = value
			appendVectors(index+1, current)
		}
		delete(current, dimensions[index].name)
	}
	appendVectors(0, make(map[string]any, len(dimensions)))

	combinationCount := usageSmokeCombinationCount(dimensions, maxTaskExprSmokeTests)
	if combinationCount > maxTaskExprSmokeTests && len(vectors) > 0 {
		last := make(map[string]any, len(dimensions))
		for _, dimension := range dimensions {
			last[dimension.name] = dimension.values[len(dimension.values)-1]
		}
		vectors[len(vectors)-1] = last
	}
	return vectors
}

func usageSmokeCombinationCount(dimensions []usageSmokeDimension, stopAfter int) int {
	count := 1
	for _, dimension := range dimensions {
		if len(dimension.values) == 0 {
			return 0
		}
		if count > stopAfter/len(dimension.values) {
			return stopAfter + 1
		}
		count *= len(dimension.values)
	}
	return count
}

func billingExprSmokeRequests() []billingexpr.RequestInput {
	return []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}
}
