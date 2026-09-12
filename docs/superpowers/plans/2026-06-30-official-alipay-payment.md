# Official Alipay Payment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an official Alipay top-up gateway without changing existing Epay, Stripe, Creem, Waffo, or Waffo Pancake behavior.

**Architecture:** Keep the current `TopUp` table and payment provider guard. Add a standalone Alipay provider path with its own settings, user pay/amount endpoints, async notify endpoint, and frontend launch behavior for QR or redirect results.

**Tech Stack:** Go, Gin, GORM, `github.com/smartwalle/alipay/v3`, React 19, TypeScript, i18next.

---

### Task 1: Backend Alipay Settings And Provider

**Files:**
- Create: `setting/payment_alipay.go`
- Create: `controller/topup_alipay.go`
- Modify: `go.mod`

- [x] Add Alipay settings for app ID, private key, Alipay public key, optional return URL, and desktop payment mode.
- [x] Build a cached SDK client that loads the Alipay public key and resets when settings change.
- [x] Implement desktop QR-first payment creation with page-pay fallback and mobile WAP pay.
- [x] Implement async notification verification with SDK `DecodeNotification`.

### Task 2: Wire Existing Top-Up Lifecycle

**Files:**
- Modify: `model/topup.go`
- Modify: `model/option.go`
- Modify: `controller/payment_webhook_availability.go`
- Modify: `router/api-router.go`

- [x] Add `PaymentProviderAlipay` and keep existing provider checks untouched.
- [x] Persist and load Alipay options through the existing option map.
- [x] Expose `enable_alipay_topup` and append an official Alipay method only when configured and compliance is confirmed.
- [x] Add `/api/user/alipay/pay`, `/api/user/alipay/amount`, and `/api/alipay/notify`.

### Task 3: Frontend Configuration And Checkout

**Files:**
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/system-settings/billing/index.tsx`
- Modify: `web/default/src/features/system-settings/billing/section-registry.tsx`
- Modify: `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`
- Modify: `web/default/src/features/wallet/api.ts`
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/hooks/use-payment.ts`
- Modify: `web/default/src/features/wallet/lib/payment.ts`
- Modify: `web/default/src/features/wallet/constants.ts`
- Modify: `web/default/src/features/wallet/hooks/use-topup-info.ts`

- [x] Add system settings fields and a dedicated Alipay Gateway tab.
- [x] Add wallet API calls for Alipay amount and payment creation.
- [x] Launch `pay_url` in a new tab when returned; submit form payload only for existing Epay.

### Task 4: Tests And Browser Verification

**Files:**
- Create: `controller/topup_alipay_test.go`
- Modify: locale JSON files if new keys are reported by i18n sync.

- [x] Add deterministic unit tests for payment mode validation and page-pay URL behavior.
- [x] Run Go tests for payment-related packages.
- [x] Run frontend i18n sync and build.
- [x] Start the app and verify payment settings and wallet UI in a browser.
