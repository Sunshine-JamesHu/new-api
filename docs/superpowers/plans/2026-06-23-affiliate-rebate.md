# Affiliate Rebate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add invite-based recharge rebates that mature only after the invitee consumes the quota from the triggering top-up.

**Architecture:** Reuse the existing user invite relationship and `aff_quota` transfer flow. Persist each eligible successful top-up as an affiliate rebate record with `remaining_quota`; wallet consumption decrements the oldest pending records and settles the rebate into the inviter's affiliate quota only when a record reaches zero.

**Tech Stack:** Go/Gin/GORM, SQLite/MySQL/PostgreSQL-compatible migrations, React 19 default frontend, React/Semi classic frontend, Bun scripts.

---

### Task 1: Backend Data And Settings

**Files:**
- Create: `model/affiliate_rebate.go`
- Modify: `model/main.go`
- Modify: `setting/operation_setting/payment_setting.go`
- Modify: `controller/option.go`
- Test: `model/affiliate_rebate_test.go`

- [x] Add `AffiliateRebate` with pending/settled status, unique `topup_id`, inviter/invitee ids, total and remaining top-up quota, rebate quota, rate percent, provider and timestamps.
- [x] Include the model in both normal and fast AutoMigrate paths.
- [x] Add `AffiliateRebateEnabled` and `AffiliateRebateRate` under `payment_setting`.
- [x] Reject positive rebate settings until payment compliance is confirmed.

### Task 2: Rebate Creation And Maturity

**Files:**
- Create: `service/affiliate_rebate.go`
- Modify: `model/topup.go`
- Modify: `controller/topup.go`
- Modify: `service/quota.go`
- Modify: `service/billing_session.go`
- Test: `service/affiliate_rebate_test.go`

- [x] Create pending rebate rows after successful wallet top-ups when affiliate rebates are enabled, the invitee has an inviter, and the rate is positive.
- [x] Decrement pending rows from oldest to newest only for wallet consumption.
- [x] Settle rows atomically into inviter `aff_quota` and `aff_history` when `remaining_quota` reaches zero.
- [x] Keep idempotency by making `topup_id` unique.

### Task 3: Frontend Settings

**Files:**
- Modify: `web/default/src/features/system-settings/integrations/payment-settings-section.tsx`
- Modify: `web/default/src/i18n/locales/*.json`
- Modify: `web/classic/src/components/settings/PaymentSetting.jsx`
- Create: `web/classic/src/pages/Setting/Payment/SettingsAffiliateRebate.jsx`

- [ ] Add a payment settings tab for affiliate rebates in default.
- [ ] Add a classic Semi settings panel for the same options.
- [ ] Add translations for all default locales and classic i18n extraction compatibility.

### Task 4: Verification

**Commands:**
- `go test ./model ./service ./controller`
- `go test ./...`
- `cd web/default && bun run typecheck && bun run build && bun run i18n:sync`
- `cd web/classic && bun run build`
- Start backend/frontend and use the browser to verify admin payment settings and the top-up to consumption maturity flow.
