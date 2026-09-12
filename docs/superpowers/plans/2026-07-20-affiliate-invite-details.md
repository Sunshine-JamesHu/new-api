# Affiliate Invite Details Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a signed-in user click the invite count and review a paginated list of invitees with display name, registration time, historical usage, and account status.

**Architecture:** Add a current-user-only API under the existing affiliate routes. The model query uses GORM with `Unscoped()` so soft-deleted invitees remain visible, scopes rows by `inviter_id`, and returns only the fields needed by the dialog. The default frontend opens a dialog from the invite count and fetches each page on demand with React Query.

**Tech Stack:** Go, Gin, GORM v2, React 19, TypeScript, TanStack Query, Base UI/shadcn components, Tailwind CSS, i18next.

---

### Task 1: Add the paginated invitee model query

**Files:**
- Create: `model/user_invitee.go`
- Create: `model/user_invitee_test.go`

- [ ] **Step 1: Write the failing model test**

Create invitees for two inviters, including an enabled user, a disabled user, and a soft-deleted user. Call `GetUserInvitees(inviterID, &common.PageInfo{Page: 1, PageSize: 2})` and assert the total, descending registration order, display name, `used_quota`, status, and deleted marker. Call page two and assert only the remaining invitee is returned; assert the other inviter's user is excluded.

- [ ] **Step 2: Run the targeted test and verify it fails**

Run: `go test ./model -run TestGetUserInvitees -count=1`

Expected: FAIL because `GetUserInvitees` is not defined.

- [ ] **Step 3: Implement the model query**

Add a focused response model and query:

```go
type UserInvitee struct {
	Id          int            `json:"id"`
	DisplayName string         `json:"display_name"`
	CreatedAt   int64          `json:"created_at"`
	UsedQuota   int            `json:"used_quota"`
	Status      int            `json:"status"`
	DeletedAt   gorm.DeletedAt `json:"-"`
	Deleted     bool           `json:"deleted" gorm:"-"`
}

func GetUserInvitees(inviterId int, pageInfo *common.PageInfo) ([]*UserInvitee, int64, error) {
	query := DB.Unscoped().Model(&User{}).Where("inviter_id = ?", inviterId)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var invitees []*UserInvitee
	err := query.Select("id", "display_name", "created_at", "used_quota", "status", "deleted_at").
		Order("created_at DESC").Order("id DESC").
		Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&invitees).Error
	for _, invitee := range invitees {
		invitee.Deleted = invitee.DeletedAt.Valid
	}
	return invitees, total, err
}
```

- [ ] **Step 4: Run the targeted model test**

Run: `go test ./model -run TestGetUserInvitees -count=1`

Expected: PASS.

### Task 2: Expose the current user's invitees API

**Files:**
- Modify: `controller/affiliate_rebate.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Add the authenticated controller**

Use `c.GetInt("id")` as the inviter ID, parse pagination with `common.GetPageQuery(c)`, call `model.GetUserInvitees`, populate `pageInfo.Items` and `pageInfo.Total`, then return `common.ApiSuccess(c, pageInfo)`. This guarantees callers cannot request another inviter's records.

- [ ] **Step 2: Register the route**

Add `selfRoute.GET("/aff/invitees", controller.GetUserInvitees)` next to `/aff/rebates` so it inherits `middleware.UserAuth()`.

- [ ] **Step 3: Format and run backend tests**

Run: `gofmt -w model/user_invitee.go model/user_invitee_test.go controller/affiliate_rebate.go router/api-router.go`

Run: `go test ./model ./controller ./router -count=1`

Expected: PASS.

### Task 3: Add the invitees dialog and click target

**Files:**
- Modify: `web/default/src/features/wallet/types.ts`
- Modify: `web/default/src/features/wallet/api.ts`
- Create: `web/default/src/features/wallet/components/dialogs/invitees-dialog.tsx`
- Modify: `web/default/src/features/wallet/components/affiliate-rewards-card.tsx`
- Modify: `web/default/src/features/wallet/index.tsx`

- [ ] **Step 1: Define the frontend API contract**

Add `UserInvitee` with `id`, `display_name`, `created_at`, `used_quota`, `status`, and `deleted`, plus `InviteeHistoryResponse` with `items` and `total`. Add `getUserInvitees(page, pageSize)` targeting `/api/user/aff/invitees`.

- [ ] **Step 2: Build the dialog**

Use the shared `Dialog`, a Base UI `Select` for 10/20/50/100 rows per page, skeleton rows while loading, a responsive four-column list, `StatusBadge` for Enabled/Disabled/Deleted, and the same previous/page-count/next controls as billing history. Use `useQuery` with query key `['wallet', 'invitees', page, pageSize]` and `enabled: open` so invitee data is loaded only after the user opens the dialog.

- [ ] **Step 3: Make the invite count interactive**

Add an `onShowInvitees` prop to `AffiliateRewardsCard`. Render only the invite count value as a `Button` with `variant="link"`, an accessible label, and a click handler that opens the dialog.

- [ ] **Step 4: Mount the dialog in Wallet**

Track `inviteesDialogOpen` in `Wallet`, pass its setter to the card, and render `InviteesDialog` next to `BillingHistoryDialog`.

### Task 4: Add translations and verify the frontend

**Files:**
- Create temporarily: `web/default/scripts/add-missing-keys.mjs`
- Modify through the script: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Run the i18n preflight**

Run from `web/default`: `bun run i18n:sync`

Expected: the sync report is generated successfully.

- [ ] **Step 2: Add all new keys through the sanctioned script**

Populate `add-missing-keys.mjs` for all six locales with the dialog title, description, field labels, empty/error copy, and invite-count accessibility label. Run `node scripts/add-missing-keys.mjs`, then remove the temporary script.

- [ ] **Step 3: Normalize and validate translations**

Run: `bun run i18n:sync`

Expected: every new key exists in all six locale files.

- [ ] **Step 4: Run frontend checks**

Run: `bun run typecheck`

Run lint against the modified TS/TSX files with `bunx oxlint -c .oxlintrc.json <files>`.

Run: `bun run build`

Expected: all commands succeed without errors.
