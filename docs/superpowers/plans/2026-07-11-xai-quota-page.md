# xAI Quota Card Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated `/xai-quota` page that shows xAI weekly, product, pay-as-you-go, and monthly quota snapshots in responsive cards and refreshes weekly/monthly billing data in parallel.

**Architecture:** Extend the existing Cliproxy binding snapshot with xAI-specific normalized fields, keep the current role-aware binding APIs, and add a `type=xai` server-side filter. The refresh controller will branch for xAI, call both billing endpoints concurrently, merge partial successes with the stored snapshot, then the new Default frontend page will render pure display models derived from the binding response.

**Tech Stack:** Go 1.22+, Gin, GORM, Testify, React 19, TypeScript, TanStack Query/Router, Tailwind CSS, Base UI components, Bun `node:test`, i18next.

---

## File map

### Backend

- Modify `controller/cliproxy.go`: xAI response types, weekly/monthly parsers, parallel refresh orchestration, plan resolution, partial-success handling.
- Modify `controller/cliproxy_test.go`: parser, request header, merge, partial-success, and plan regression tests.
- Modify `model/cliproxy_auth_file.go`: snapshot columns, update semantics, xAI list filter.
- Modify `model/cliproxy_auth_file_test.go`: persistence, error preservation, partial-warning update, and type filter tests.
- Modify `model/user.go`: default sidebar module includes `xaiQuota`.
- Modify `controller/user.go`: default sidebar response includes `xaiQuota`.

### Frontend

- Modify `web/default/src/features/cliproxy-auth-files/api.ts`: pass optional binding type query.
- Modify `web/default/src/features/cliproxy-auth-files/types.ts`: xAI snapshot fields and `type` query parameter.
- Create `web/default/src/features/xai-quota/lib/xai-usage.ts`: pure parsing, percentage, currency, plan, product row, account masking helpers.
- Create `web/default/src/features/xai-quota/lib/xai-usage.test.ts`: deterministic display-model tests.
- Create `web/default/src/features/xai-quota/index.tsx`: responsive page, cards, loading/empty/error/partial-warning states, per-card refresh, account privacy toggle.
- Create `web/default/src/routes/_authenticated/xai-quota/index.tsx`: authenticated route.
- Modify `web/default/src/hooks/use-sidebar-data.ts`: navigation item.
- Modify `web/default/src/hooks/use-sidebar-config.ts`: default module and URL mapping.
- Modify `web/default/src/features/system-settings/maintenance/config.ts`: settings default.
- Modify `web/default/src/features/system-settings/maintenance/sidebar-modules-section.tsx`: administrator visibility control.
- Modify `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`: user-visible strings.
- Regenerate `web/default/src/routeTree.gen.ts` through the frontend build.

### Documentation

- Modify `docs/custom-development-features.md`: record the independent xAI quota page and dual billing refresh behavior.

---

### Task 1: Parse normalized weekly and monthly xAI billing snapshots

**Files:**
- Modify: `controller/cliproxy.go`
- Test: `controller/cliproxy_test.go`

- [ ] **Step 1: Add failing parser and request tests**

Add table-driven tests covering camelCase and snake_case payloads, weekly product usage, monthly amounts, SuperGrok Heavy, and the required Grok headers. The core assertions must include:

```go
func TestResolveCliproxyXAIWeeklyUsage(t *testing.T) {
	usage, ok := resolveCliproxyXAIWeeklyUsage(map[string]any{
		"config": map[string]any{
			"currentPeriod": map[string]any{
				"type": "weekly",
				"start": "2026-07-09T13:16:00Z",
				"end": "2026-07-16T13:16:00Z",
			},
			"creditUsagePercent": float64(45),
			"productUsage": []any{
				map[string]any{"product": "Api", "usagePercent": float64(45)},
			},
		},
	})
	require.True(t, ok)
	require.Equal(t, 45, usage.XAIWeeklyPercent)
	require.Equal(t, time.Date(2026, 7, 9, 13, 16, 0, 0, time.UTC).Unix(), usage.XAIWeeklyPeriodStartAt)
	require.Equal(t, time.Date(2026, 7, 16, 13, 16, 0, 0, time.UTC).Unix(), usage.XAIWeeklyPeriodEndAt)
	require.JSONEq(t, `[{"product":"Api","usage_percent":45}]`, usage.XAIProductUsage)
}

func TestResolveCliproxyXAIMonthlyUsageSupportsHeavyPlan(t *testing.T) {
	usage, ok := resolveCliproxyXAIMonthlyUsage(map[string]any{
		"config": map[string]any{
			"monthly_limit": map[string]any{"val": float64(150000)},
			"used": map[string]any{"val": float64(42000)},
			"on_demand_cap": map[string]any{"val": float64(10000)},
			"on_demand_used": map[string]any{"val": float64(300)},
			"billing_period_end": "2026-08-01T00:00:00Z",
		},
	})
	require.True(t, ok)
	require.Equal(t, "SuperGrok Heavy", usage.PlanType)
	require.Equal(t, 150000, usage.Quota)
	require.Equal(t, 42000, usage.UsedTokens)
	require.Equal(t, 10000, usage.OnDemandCap)
	require.Equal(t, 300, usage.XAIOnDemandUsed)
}
```

- [ ] **Step 2: Run the focused controller tests and confirm RED**

Run:

```bash
go test ./controller -run 'TestResolveCliproxyXAI|TestBuildCliproxyXAI' -count=1
```

Expected: compilation fails because the new parser fields and functions do not exist.

- [ ] **Step 3: Implement normalized parsers and request builders**

Add these domain types and fields:

```go
type cliproxyXAIProductUsage struct {
	Product      string `json:"product"`
	UsagePercent int    `json:"usage_percent"`
}

type cliproxyUsageRefreshBody struct {
	UsedTokens                int
	Quota                     int
	PlanType                  string
	FiveHourPercent           int
	FiveHourResetAt           int64
	WeeklyPercent             int
	WeeklyResetAt             int64
	CodexFiveHourPercent      int
	CodexFiveHourResetAt      int64
	CodexWeeklyPercent        int
	CodexWeeklyResetAt        int64
	OnDemandCap               int
	BillingPeriodEndAt        int64
	XAIWeeklyPercent       int
	XAIWeeklyPeriodStartAt int64
	XAIWeeklyPeriodEndAt   int64
	XAIProductUsage        string
	XAIOnDemandUsed        int
}
```

Use `common.Marshal` to serialize normalized product rows. Implement `resolveCliproxyXAIWeeklyUsage`, `resolveCliproxyXAIMonthlyUsage`, and `resolveCliproxyXAIPlan` with these exact plan rules:

```go
func resolveCliproxyXAIPlan(monthlyLimitCents int) string {
	switch monthlyLimitCents {
	case 15000:
		return "SuperGrok"
	case 150000:
		return "SuperGrok Heavy"
	default:
		return "SuperGrok"
	}
}
```

Build weekly and monthly requests from a shared helper so both include:

```go
map[string]string{
	"Authorization":         "Bearer $TOKEN$",
	"x-xai-token-auth":      "xai-grok-cli",
	"x-grok-client-version": "0.2.91",
	"accept":                "*/*",
	"user-agent":            "grok-pager/0.2.91 grok-shell/0.2.91 (macos; aarch64)",
}
```

- [ ] **Step 4: Run focused tests and confirm GREEN**

Run:

```bash
go test ./controller -run 'TestResolveCliproxyXAI|TestBuildCliproxyXAI|TestExtractCliproxyUsageSupportsXAIBillingPayload' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit parser work**

```bash
git add controller/cliproxy.go controller/cliproxy_test.go
git commit -F - <<'EOF'
feat(xai): 解析周度与月度额度快照

补充 xAI 周度周期、产品用量、按量付费和 SuperGrok Heavy 套餐解析，并统一 Grok CLI 请求头。

Tests:
- go test ./controller -run 'TestResolveCliproxyXAI|TestBuildCliproxyXAI|TestExtractCliproxyUsageSupportsXAIBillingPayload' -count=1

Constraint: 本提交只建立解析和请求契约，尚未接入双接口刷新流程。
Scope-risk: medium
Confidence: high
EOF
```

### Task 2: Persist xAI snapshots and filter xAI bindings server-side

**Files:**
- Modify: `model/cliproxy_auth_file.go`
- Test: `model/cliproxy_auth_file_test.go`

- [ ] **Step 1: Add failing persistence and filter tests**

Add an xAI fixture, a Codex fixture, and a SuperGrok-plan fixture. Assert `CliproxyAuthFileBindingQuery{Type: "xai"}` returns only the two xAI records and respects `UserId`. Extend the usage preservation test with every new snapshot field. Add a partial-warning test using `AllowPartialUsage: true` that updates the new values even when `LastError` is non-empty.

```go
binding, err := UpdateCliproxyAuthFileBindingUsage(1, CliproxyUsageRefreshUpdate{
	LastUsageTokens:           4200,
	LastUsageQuota:            15000,
	LastPlanType:              "SuperGrok",
	LastXAIWeeklyPercent:      45,
	LastXAIWeeklyPeriodStartAt: 1783602960,
	LastXAIWeeklyPeriodEndAt:  1784207760,
	LastXAIProductUsage:       `[{"product":"Api","usage_percent":45}]`,
	LastXAIOnDemandCap:        2500,
	LastXAIOnDemandUsed:       300,
	LastXAIBillingPeriodEndAt: 1785542400,
	LastError:                 "月度额度刷新失败: timeout",
	AllowPartialUsage:         true,
})
require.NoError(t, err)
require.Equal(t, 45, binding.LastXAIWeeklyPercent)
require.Equal(t, 4200, binding.LastUsageTokens)
require.Equal(t, "月度额度刷新失败: timeout", binding.LastError)
```

- [ ] **Step 2: Run model tests and confirm RED**

Run:

```bash
go test ./model -run 'TestGetCliproxyAuthFileBindingsFiltersXAI|TestUpdateCliproxyAuthFileBindingUsage' -count=1
```

Expected: compilation fails on missing fields.

- [ ] **Step 3: Add model fields and merge semantics**

Add fields to `CliproxyAuthFileBinding` and `CliproxyUsageRefreshUpdate`:

```go
LastXAIWeeklyPercent       int    `json:"last_xai_weekly_percent" gorm:"default:0"`
LastXAIWeeklyPeriodStartAt int64  `json:"last_xai_weekly_period_start_at" gorm:"bigint;default:0"`
LastXAIWeeklyPeriodEndAt   int64  `json:"last_xai_weekly_period_end_at" gorm:"bigint;default:0"`
LastXAIProductUsage        string `json:"last_xai_product_usage" gorm:"type:text"`
LastXAIOnDemandUsed        int    `json:"last_xai_on_demand_used" gorm:"default:0"`
```

Add `Type string` to `CliproxyAuthFileBindingQuery` and `AllowPartialUsage bool` to `CliproxyUsageRefreshUpdate`. Preserve all snapshot fields when `LastError != "" && !AllowPartialUsage`; otherwise persist the supplied partial-merge snapshot.

For `type=xai`, append a cross-database `LOWER(...)` predicate matching `xai-%`, `xai_%`, `xai`, `supergrok`, and `supergrokheavy` across `auth_name`, `auth_file`, and normalized `last_plan_type`.

- [ ] **Step 4: Run model tests and confirm GREEN**

Run:

```bash
go test ./model -run 'TestGetCliproxyAuthFileBindingsFiltersXAI|TestUpdateCliproxyAuthFileBindingUsage' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit persistence work**

```bash
git add model/cliproxy_auth_file.go model/cliproxy_auth_file_test.go
git commit -F - <<'EOF'
feat(xai): 持久化额度卡片快照

保存 xAI 周周期、产品用量和按量已用金额，并为绑定列表增加角色兼容的 xAI 类型过滤。

Tests:
- go test ./model -run 'TestGetCliproxyAuthFileBindingsFiltersXAI|TestUpdateCliproxyAuthFileBindingUsage' -count=1

Constraint: 产品用量使用 TEXT JSON 保存以兼容 SQLite、MySQL 和 PostgreSQL。
Scope-risk: medium
Confidence: high
EOF
```

### Task 3: Refresh weekly and monthly xAI billing in parallel

**Files:**
- Modify: `controller/cliproxy.go`
- Test: `controller/cliproxy_test.go`

- [ ] **Step 1: Add failing orchestration tests with a fake caller**

Define a local test fake implementing the production interface:

```go
type fakeCliproxyAPICaller struct {
	responses map[string]*service.CliproxyAPICallResponse
	errors    map[string]error
}

func (f *fakeCliproxyAPICaller) CallAPI(_ context.Context, request service.CliproxyAPICallRequest) (*service.CliproxyAPICallResponse, error) {
	return f.responses[request.URL], f.errors[request.URL]
}
```

Cover four cases:

- both requests succeed and all fields merge;
- weekly succeeds/monthly fails and stored monthly values survive;
- monthly succeeds/weekly fails and stored weekly values survive;
- both fail and the helper returns an error.

For partial success assert `AllowPartialUsage == true` and the warning identifies the failed side.

- [ ] **Step 2: Run orchestration tests and confirm RED**

Run:

```bash
go test ./controller -run 'TestRefreshCliproxyXAIUsage' -count=1
```

Expected: compilation fails because the caller interface and helper do not exist.

- [ ] **Step 3: Implement concurrent refresh and controller branch**

Introduce:

```go
type cliproxyAPICaller interface {
	CallAPI(context.Context, service.CliproxyAPICallRequest) (*service.CliproxyAPICallResponse, error)
}

type cliproxyXAIRefreshResult struct {
	Usage             cliproxyUsageRefreshBody
	Warning           string
	AllowPartialUsage bool
}
```

Start both `CallAPI` operations in goroutines, send results through buffered channels, parse each side independently, and merge successful values into a snapshot initialized from the existing binding. Do not mutate the binding from goroutines.

In `RefreshCliproxyAuthFileBindingUsage`, branch immediately after client construction:

```go
if isCliproxyXAIAuthFile(binding) {
	refresh, refreshErr := refreshCliproxyXAIUsage(c.Request.Context(), client, binding)
	if refreshErr != nil {
		// preserve old snapshot through the existing error update path
	}
	updatedBinding, updateErr := model.UpdateCliproxyAuthFileBindingUsage(id, model.CliproxyUsageRefreshUpdate{
		LastUsageTokens:            refresh.Usage.UsedTokens,
		LastUsageQuota:             refresh.Usage.Quota,
		LastPlanType:               refresh.Usage.PlanType,
		LastXAIWeeklyPercent:       refresh.Usage.XAIWeeklyPercent,
		LastXAIWeeklyPeriodStartAt: refresh.Usage.XAIWeeklyPeriodStartAt,
		LastXAIWeeklyPeriodEndAt:   refresh.Usage.XAIWeeklyPeriodEndAt,
		LastXAIProductUsage:        refresh.Usage.XAIProductUsage,
		LastXAIOnDemandCap:         refresh.Usage.OnDemandCap,
		LastXAIOnDemandUsed:        refresh.Usage.XAIOnDemandUsed,
		LastXAIBillingPeriodEndAt:  refresh.Usage.BillingPeriodEndAt,
		LastError:                  refresh.Warning,
		AllowPartialUsage:          refresh.AllowPartialUsage,
	})
	// return the updated binding
}
```

Pass `c.Query("type")` into `CliproxyAuthFileBindingQuery.Type` in the list handler.

- [ ] **Step 4: Run controller and model packages**

Run:

```bash
go test ./controller ./model -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit orchestration work**

```bash
git add controller/cliproxy.go controller/cliproxy_test.go
git commit -F - <<'EOF'
feat(xai): 并行刷新双账单额度

并行读取 xAI 周度与月度账单，支持单侧成功时更新可用数据并保留另一侧历史快照。

Tests:
- go test ./controller ./model -count=1

Constraint: 不新增后台刷新任务，仍由用户触发单个认证文件刷新。
Scope-risk: medium
Confidence: high
EOF
```

### Task 4: Build the xAI card display model with TDD

**Files:**
- Modify: `web/default/src/features/cliproxy-auth-files/types.ts`
- Modify: `web/default/src/features/cliproxy-auth-files/api.ts`
- Create: `web/default/src/features/xai-quota/lib/xai-usage.ts`
- Create: `web/default/src/features/xai-quota/lib/xai-usage.test.ts`

- [ ] **Step 1: Write failing pure-function tests**

Create fixtures for the reference screenshot and assert:

```ts
const summary = buildXAIQuotaSummary({
  last_usage_tokens: 1768,
  last_usage_quota: 15000,
  last_plan_type: 'SuperGrok',
  last_xai_weekly_percent: 45,
  last_xai_weekly_period_start_at: 1783599360,
  last_xai_weekly_period_end_at: 1784204160,
  last_xai_product_usage: '[{"product":"Api","usage_percent":45}]',
  last_xai_on_demand_cap: 0,
  last_xai_on_demand_used: 0,
  last_xai_billing_period_end_at: 1785542400,
})

assert.equal(summary.planLabel, 'SuperGrok')
assert.equal(summary.weekly.usedPercent, 45)
assert.equal(summary.weekly.remainingPercent, 55)
assert.equal(summary.products[0].label, 'API')
assert.equal(summary.monthly.remainingPercent, 88)
assert.equal(summary.monthly.remainingLabel, '$132.32')
assert.equal(summary.monthly.limitLabel, '$150.00')
assert.equal(summary.payAsYouGo.enabled, false)
assert.equal(maskXAIAccountName('xai-duboislee1988@gmail.com.json'), 'xai-d***********8@gmail.com.json')
```

Also cover malformed product JSON, over-limit monthly usage, enabled pay-as-you-go, and color thresholds.

- [ ] **Step 2: Run the test and confirm RED**

Run:

```bash
bun test src/features/xai-quota/lib/xai-usage.test.ts
```

Expected: module-not-found failure.

- [ ] **Step 3: Implement the pure display model**

Export `buildXAIQuotaSummary`, `maskXAIAccountName`, `remainingProgressClass`, and typed row models. Parse `last_xai_product_usage` with `JSON.parse` inside a guarded function; malformed or non-array data returns `[]`. Normalize all percentages to `0..100` and all cents to non-negative integers.

Use the exact remaining-color contract:

```ts
export function remainingProgressClass(percent: number): string {
  if (percent < 30) return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  if (percent < 70) return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}
```

Extend `GetCliproxyAuthFileBindingsParams` with `type?: 'xai'` and add the five snapshot properties to `CliproxyAuthFileBinding`.

- [ ] **Step 4: Run utility tests and typecheck**

Run:

```bash
bun test src/features/xai-quota/lib/xai-usage.test.ts src/features/cliproxy-auth-files/lib/usage-summary.test.ts
bun run typecheck
```

Expected: all tests pass and `tsgo -b` exits 0.

- [ ] **Step 5: Commit display-model work**

```bash
git add web/default/src/features/cliproxy-auth-files/api.ts web/default/src/features/cliproxy-auth-files/types.ts web/default/src/features/xai-quota/lib
git commit -F - <<'EOF'
feat(default): 建立 xAI 额度卡片模型

归一化周额度、产品用量、按量付费和月度金额，并覆盖账号脱敏与剩余额度颜色阈值。

Tests:
- bun test src/features/xai-quota/lib/xai-usage.test.ts src/features/cliproxy-auth-files/lib/usage-summary.test.ts
- bun run typecheck

Constraint: 本提交仅提供数据模型和 API 类型，尚未接入页面路由。
Scope-risk: low
Confidence: high
EOF
```

### Task 5: Implement the responsive xAI quota page

**Files:**
- Create: `web/default/src/features/xai-quota/index.tsx`
- Create: `web/default/src/routes/_authenticated/xai-quota/index.tsx`
- Modify: `web/default/src/hooks/use-sidebar-data.ts`
- Modify: `web/default/src/hooks/use-sidebar-config.ts`
- Modify: `web/default/src/features/system-settings/maintenance/config.ts`
- Modify: `web/default/src/features/system-settings/maintenance/sidebar-modules-section.tsx`
- Modify: `model/user.go`
- Modify: `controller/user.go`
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify: `web/default/src/i18n/locales/fr.json`
- Modify: `web/default/src/i18n/locales/ru.json`
- Modify: `web/default/src/i18n/locales/ja.json`
- Modify: `web/default/src/i18n/locales/vi.json`
- Regenerate: `web/default/src/routeTree.gen.ts`

- [ ] **Step 1: Add the authenticated route shell and confirm route generation is RED**

Create the route:

```tsx
import { createFileRoute } from '@tanstack/react-router'

import { XAIQuotaPage } from '@/features/xai-quota'

export const Route = createFileRoute('/_authenticated/xai-quota/')({
  component: XAIQuotaPage,
})
```

Run `bun run typecheck`; expected failure until the page exists and the route tree is regenerated.

- [ ] **Step 2: Implement the page and card component**

`XAIQuotaPage` must:

- query `getCliproxyAuthFileBindings({ p: 1, page_size: 100, type: 'xai' })` with query key `['cliproxy-auth-file-bindings', 'xai']`;
- display the API `total` in a Badge beside the title;
- store account visibility under `xai-quota-account-display` in localStorage, defaulting to masked;
- track refreshing IDs with `Set<number>` so only the clicked card is disabled;
- invalidate both `['cliproxy-auth-file-bindings', 'xai']` and the existing binding list after refresh;
- render skeleton cards, a page-level retry Alert, an empty-state link to `/cliproxy-auth-files`, and `last_error` as a non-blocking card warning;
- use `grid gap-4 md:grid-cols-2 2xl:grid-cols-3`;
- render weekly and product rows with “used” labels but remaining-width progress bars;
- render pay-as-you-go disabled as a compact label row;
- render monthly remaining percent, remaining/limit amounts, and reset time;
- keep all visible text behind `t()`.

The title action uses `Eye`/`EyeOff`, visible text, `aria-pressed`, and an outline button. Card actions use `RefreshCw` and show spin only for the target card.

- [ ] **Step 3: Wire navigation and administrator visibility settings**

Add `xaiQuota: true` beside GLM/DeepSeek in all frontend and backend default sidebar maps. Add `/xai-quota` to `URL_TO_CONFIG_MAP`, a `Gauge` navigation item titled `t('xAI Quota')`, and the maintenance description `t('Track xAI account quota usage.')`.

- [ ] **Step 4: Add locale keys**

Add the same English keys to all six locale files, translated in `zh.json` at minimum and using English fallback text for untranslated locales:

```json
"xAI Quota": "xAI 额度",
"Track xAI account quota usage.": "跟踪 xAI 账户额度使用情况。",
"Show full accounts": "显示完整账号",
"Mask accounts": "隐藏账号信息",
"Plan": "套餐",
"Weekly Limit": "周限额",
"Used {{percent}}%": "已用 {{percent}}%",
"Reset {{time}}": "重置 {{time}}",
"{{product}} Usage": "{{product}} 使用",
"Pay-as-you-go": "按量付费",
"Not enabled": "未启用",
"Monthly Credits": "月度额度",
"Refresh Quota": "刷新额度",
"No xAI auth files are bound yet.": "尚未绑定 xAI 认证文件。",
"Manage auth files": "管理认证文件",
"Some quota data could not be refreshed.": "部分额度数据刷新失败。",
"Failed to fetch xAI quota bindings": "获取 xAI 额度绑定失败"
```

- [ ] **Step 5: Generate the route tree and run frontend checks**

Run:

```bash
bun run build
bun run typecheck
bunx oxlint -c .oxlintrc.json \
  src/features/xai-quota/index.tsx \
  src/features/xai-quota/lib/xai-usage.ts \
  src/features/xai-quota/lib/xai-usage.test.ts \
  src/features/cliproxy-auth-files/api.ts \
  src/features/cliproxy-auth-files/types.ts \
  src/hooks/use-sidebar-data.ts \
  src/hooks/use-sidebar-config.ts \
  src/features/system-settings/maintenance/config.ts \
  src/features/system-settings/maintenance/sidebar-modules-section.tsx
```

Expected: build, typecheck, and lint exit 0; `routeTree.gen.ts` contains `/xai-quota`.

- [ ] **Step 6: Run backend sidebar tests and commit the page**

Run:

```bash
go test ./controller ./model -count=1
```

Then commit:

```bash
git add controller/user.go model/user.go web/default/src/features/xai-quota/index.tsx web/default/src/routes/_authenticated/xai-quota/index.tsx web/default/src/hooks/use-sidebar-data.ts web/default/src/hooks/use-sidebar-config.ts web/default/src/features/system-settings/maintenance/config.ts web/default/src/features/system-settings/maintenance/sidebar-modules-section.tsx web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/vi.json web/default/src/routeTree.gen.ts
git commit -F - <<'EOF'
feat(default): 新增 xAI 额度卡片页

新增独立 xAI 额度入口和响应式卡片，展示周限额、产品使用、按量付费和月度额度，并支持账号脱敏与单卡刷新。

Tests:
- go test ./controller ./model -count=1
- cd web/default && bun run build
- cd web/default && bun run typecheck
- cd web/default && bunx oxlint -c .oxlintrc.json src/features/xai-quota/index.tsx src/features/xai-quota/lib/xai-usage.ts src/features/xai-quota/lib/xai-usage.test.ts src/features/cliproxy-auth-files/api.ts src/features/cliproxy-auth-files/types.ts src/hooks/use-sidebar-data.ts src/hooks/use-sidebar-config.ts src/features/system-settings/maintenance/config.ts src/features/system-settings/maintenance/sidebar-modules-section.tsx

Constraint: 认证文件的创建、编辑、启停和删除仍保留在认证文件页面。
Scope-risk: medium
Confidence: high
EOF
```

### Task 6: Document, visually verify, and merge the feature branch

**Files:**
- Modify: `docs/custom-development-features.md`

- [ ] **Step 1: Record the delivered behavior**

Add one row describing the independent `/xai-quota` page, dual billing refresh, partial-success snapshot, and responsive card display.

- [ ] **Step 2: Run the full verification set**

Run from repository root:

```bash
go test ./controller ./model ./service -count=1
cd web/default && bun test \
  src/features/xai-quota/lib/xai-usage.test.ts \
  src/features/cliproxy-auth-files/lib/usage-summary.test.ts \
  src/features/cliproxy-auth-files/lib/auth-file-type.test.ts
cd web/default && bun run typecheck
cd web/default && bun run build
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 3: Run browser QA**

Start the local development stack with `go run main.go` from the repository root and `bun run dev` from `web/default`. Open `/xai-quota` with an authenticated session and capture:

- desktop two-column layout around 1440px;
- wide-screen three-column layout at or above 1536px;
- mobile single-column layout around 390px;
- masked and full-account states;
- one successful refresh and one card warning/error state.

Verify keyboard focus, `aria-pressed`, filename truncation/title, progress semantics, and that only the selected card spins.

- [ ] **Step 4: Commit documentation and final fixes**

```bash
git add docs/custom-development-features.md
git commit -F - <<'EOF'
docs(xai): 记录独立额度卡片页

记录 xAI 双账单刷新、部分成功快照和独立额度页面的交付范围与验证结果。

Tests:
- go test ./controller ./model ./service -count=1
- cd web/default && bun test src/features/xai-quota/lib/xai-usage.test.ts src/features/cliproxy-auth-files/lib/usage-summary.test.ts src/features/cliproxy-auth-files/lib/auth-file-type.test.ts
- cd web/default && bun run typecheck
- cd web/default && bun run build
- git diff --check

Constraint: 不新增后台定时刷新，也不重构 GLM/DeepSeek 额度页面。
Scope-risk: low
Confidence: high
EOF
```

- [ ] **Step 5: Review and integrate**

Use `superpowers:requesting-code-review`, address findings with `superpowers:receiving-code-review`, rerun the verification set with `superpowers:verification-before-completion`, merge the feature branch into `master`, then delete the merged feature branch and its worktree.

---

## Plan self-review

- Spec coverage: route, card layout, account count/privacy, weekly/monthly parallel requests, product usage, pay-as-you-go, partial success, role filtering, sidebar visibility, i18n, responsive states, browser QA, and cleanup all map to explicit tasks.
- Placeholder scan: no `TBD`, `TODO`, deferred implementation, or undefined task references remain.
- Type consistency: backend snapshot names map one-to-one to frontend snake_case properties; `type=xai`, `AllowPartialUsage`, and progress semantics are consistent across tasks.
- Scope check: the plan changes one cohesive feature across its required backend and frontend boundary; GLM/DeepSeek unification remains explicitly excluded.
