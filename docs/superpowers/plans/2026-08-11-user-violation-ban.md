# User Violation Ban Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically disable a non-administrator user after configured upstream error responses reach a sliding-window threshold.

**Architecture:** Persist the administrator's rule set and thresholds through the existing system-option store, while keeping a validated in-memory snapshot for the relay path. Capture bounded upstream error bodies on `NewAPIError`, increment a per-user Redis counter with its expiration refreshed for every matching response, and reuse `model.User.Update(false)` to disable the account and revoke sessions. Expose the configuration as a Security & Limits settings section backed by React Hook Form and Zod.

**Tech Stack:** Go, Gin, GORM, Redis/go-redis, React 19, TypeScript, React Hook Form, Zod, i18next, Bun.

---

### Task 1: Add Validated System Options

**Files:**
- Create: `setting/violation_ban.go`
- Modify: `model/option.go`
- Test: `setting/violation_ban_test.go`

- [ ] **Step 1: Write settings validation tests**

```go
require.NoError(t, UpdateUserViolationBanRulesByJSONString(
    `[{"status_code":502,"keyword":"provider blocked"}]`,
))
require.NoError(t, UpdateUserViolationBanThreshold("3"))
require.NoError(t, UpdateUserViolationBanWindowHours("6"))
assert.True(t, GetUserViolationBanConfig().Matches(502, "Provider Blocked"))
assert.Error(t, ValidateUserViolationBanRulesJSON(`[{"status_code":99,"keyword":"blocked"}]`))
```

- [ ] **Step 2: Run the test before implementation**

Run: `go test ./setting -run UserViolationBan`

Expected: FAIL because the configuration API does not exist.

- [ ] **Step 3: Add a synchronized configuration snapshot**

```go
type UserViolationBanRule struct {
    StatusCode int    `json:"status_code"`
    Keyword    string `json:"keyword"`
}

func (config UserViolationBanConfig) Matches(statusCode int, responseBody string) bool {
    lowerBody := strings.ToLower(responseBody)
    for _, rule := range config.Rules {
        if rule.StatusCode == statusCode && strings.Contains(lowerBody, strings.ToLower(rule.Keyword)) {
            return true
        }
    }
    return false
}
```

Validate the JSON array, status codes, non-empty keywords, rule count, threshold, and window bounds before applying a system option update.

- [ ] **Step 4: Register option defaults, validation, and application**

```go
common.OptionMap["UserViolationBanEnabled"] = strconv.FormatBool(config.Enabled)
common.OptionMap["UserViolationBanRules"] = setting.UserViolationBanRulesToJSONString()
common.OptionMap["UserViolationBanThreshold"] = setting.UserViolationBanThresholdString()
common.OptionMap["UserViolationBanWindowHours"] = setting.UserViolationBanWindowHoursString()
```

Wire all four keys through `validateOptionValue` and the existing option application switch in `model/option.go`.

- [ ] **Step 5: Run settings tests**

Run: `go test ./setting -run UserViolationBan`

Expected: PASS.

### Task 2: Count Matching Errors and Disable the Account

**Files:**
- Modify: `common/redis.go`
- Create: `service/user_violation_ban.go`
- Modify: `relaykit/types/error.go`
- Modify: `service/error.go`
- Modify: `controller/relay.go`
- Test: `common/redis_sliding_test.go`
- Test: `service/error_test.go`
- Test: `service/user_violation_ban_test.go`

- [ ] **Step 1: Write Redis sliding-expiration tests**

```go
count, err := RedisIncrWithSlidingExpiration("test:violation", 2*time.Hour)
require.NoError(t, err)
assert.EqualValues(t, 1, count)

server.FastForward(90 * time.Minute)
count, err = RedisIncrWithSlidingExpiration("test:violation", 2*time.Hour)
require.NoError(t, err)
assert.EqualValues(t, 2, count)
```

- [ ] **Step 2: Implement atomic increment with refreshed TTL**

```go
const script = `
local count = redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], ARGV[1])
return count`

count, err := RDB.Eval(context.Background(), script, []string{key}, seconds).Int64()
```

Reject unavailable Redis, empty keys, and non-positive expiration durations.

- [ ] **Step 3: Preserve a bounded raw upstream body internally**

```go
func (e *NewAPIError) SetResponseBody(body string) {
    if e != nil {
        e.responseBody = body
    }
}
```

Capture at most 64 KiB in `RelayErrorHandler`, using a deferred setter so the body follows every error-construction branch without changing the client-facing error message.

- [ ] **Step 4: Disable only eligible users at threshold**

```go
if count < config.Threshold {
    return
}
user, err := model.GetUserById(userID, false)
if err != nil || user.Status != common.UserStatusEnabled || user.Role >= common.RoleAdminUser {
    return
}
user.Status = common.UserStatusDisabled
if err := user.Update(false); err != nil {
    return
}
_ = model.InvalidateUserTokensCache(userID)
```

Invoke this logic from `processChannelError` before channel auto-ban behavior. Do not log raw response bodies.

- [ ] **Step 5: Add behavior regressions**

```go
newAPIError := RelayErrorHandler(context.Background(), resp, false)
require.Equal(t, body, newAPIError.ResponseBody())

HandleUserViolationBan(context.Background(), user.Id, matchingViolationBanError())
HandleUserViolationBan(context.Background(), user.Id, matchingViolationBanError())
require.Equal(t, common.UserStatusDisabled, disabled.Status)
```

Cover threshold behavior, session revocation, administrator exemption, unavailable Redis, response-body capture, and sliding expiration.

- [ ] **Step 6: Run backend regression tests**

Run: `go test ./setting ./common ./service ./controller ./model`

Expected: PASS.

### Task 3: Build the Security & Limits Settings UI

**Files:**
- Create: `web/src/features/system-settings/request-limits/violation-ban-section.tsx`
- Modify: `web/src/features/system-settings/security/section-registry.tsx`
- Modify: `web/src/features/system-settings/security/index.tsx`
- Modify: `web/src/features/system-settings/types.ts`

- [ ] **Step 1: Define the form schema and defaults**

```ts
const createViolationBanSchema = (t: TFunction) =>
  z.object({
    UserViolationBanEnabled: z.boolean(),
    UserViolationBanThreshold: z.number().int().min(1).max(2147483647),
    UserViolationBanWindowHours: z.number().int().min(1).max(24 * 365),
    UserViolationBanRules: z
      .array(z.object({ statusCode: z.number().int().min(100).max(599), keyword: z.string().trim().min(1).max(256) }))
      .min(1)
      .max(32),
  })
```

- [ ] **Step 2: Write the form with the established settings primitives**

Use `Form`, `SettingsForm`, `SettingsSwitchItem`, `Input`, `Button`, and `useFieldArray`. Give each row a status-code input, keyword input, and an accessible icon-only remove button. Keep the enable switch separate from the rules and thresholds so configuration can be prepared before it is enabled.

- [ ] **Step 3: Save options in a valid order**

```ts
for (const update of updates) {
  await updateOption.mutateAsync(update)
}
```

When enabling, persist the rules, threshold, and window before `UserViolationBanEnabled`. When disabling, persist the enabled flag first. Serialize the rule array as `{ status_code, keyword }` JSON for the backend option.

- [ ] **Step 4: Register the section and initial values**

```tsx
{
  id: 'violation-ban',
  titleKey: 'Violation Ban',
  build: (settings) => <ViolationBanSection defaultValues={{ /* option values */ }} />,
}
```

Add the four option fields to `SecuritySettings` and `defaultSecuritySettings`.

### Task 4: Localize and Verify

**Files:**
- Modify through script only: `web/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Modify: `web/scripts/add-missing-keys.mjs`

- [ ] **Step 1: Add all UI strings through the locale script**

```js
const newKeys = {
  en: { 'Violation Ban': 'Violation Ban' },
  zh: { 'Violation Ban': '违规封禁' },
  'zh-TW': { 'Violation Ban': '違規封禁' },
  fr: { 'Violation Ban': 'Bannissement pour infraction' },
  ja: { 'Violation Ban': '違反による禁止' },
  ru: { 'Violation Ban': 'Блокировка за нарушение' },
  vi: { 'Violation Ban': 'Cam do vi pham' },
}
```

Use the same script entry for every new `t(...)` key, then run i18n synchronization. Remove the temporary script only if it did not already exist in the repository.

- [ ] **Step 2: Run frontend checks**

Run: `cd web && bun run i18n:sync && bun run typecheck && bun run lint && bun run build`

Expected: all commands pass.

- [ ] **Step 3: Run final Go checks and inspect the diff**

Run: `go test ./...`

Run: `cd relaykit && GOWORK=off go build ./...`

Run: `git diff --check && git status --short`

Expected: all tests and builds pass, with only the scoped feature and localization changes present.
