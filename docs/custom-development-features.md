# 二开功能记录

更新时间：2026-07-07

本文记录当前项目已做的二开功能，便于后续升级、排查和继续开发时快速确认改动范围。后续新增或调整二开功能时，需要同步更新本文。

维护规则：
- 记录功能名称、涉及范围、关键提交和验证方式。
- 不记录 curl、cookie、session、代理密码、SSH 密码等敏感信息。
- 只记录项目代码和配置层面的改动；临时服务器排查操作不作为长期功能记录。

## 功能清单

| 模块 | 功能 | 涉及范围 | 关键提交 |
| --- | --- | --- | --- |
| GLM / DeepSeek 额度 | 新增 GLM 和 DeepSeek 额度菜单，支持管理员配置 curl、刷新额度、普通用户只刷新 | `controller/*_quota.go`、`model/*_quota.go`、`router/api-router.go`、`web/default/src/features/quota-bindings` | `88600745` |
| GLM / DeepSeek 额度 | 额度配置支持代理地址，刷新时按配置代理请求；普通用户隐藏 curl 和 proxy | `controller/quota_http_client.go`、`controller/*_quota.go`、`model/*_quota.go`、`web/default/src/features/quota-bindings` | `68748b43` |
| GLM / DeepSeek 额度 | curl 解析支持 `-x`、`--proxy`、`--proxy=...` 代理，未单独配置代理时使用 curl 内代理，修复额度刷新请求外网超时 | `controller/quota_http_client.go`、`controller/deepseek_quota.go`、`controller/glm_quota.go` | 本次同步 |
| GLM / DeepSeek 额度 | 在 `web/default` 实现额度管理页面，新增侧边栏入口、绑定列表、创建/编辑、删除、单个刷新和全部刷新 | `web/default/src/features/quota-bindings`、`web/default/src/routes/_authenticated/*-quota`、`web/default/src/hooks/use-sidebar-*` | 本次同步 |
| GLM / DeepSeek 额度 | DeepSeek 额度改为剩余金额进度条，显示本月 Token 和本月消费，Token 悬浮显示精确值 | `controller/deepseek_quota.go`、`model/deepseek_quota.go`、`web/default/src/features/quota-bindings` | 本次同步 |
| GLM / DeepSeek 额度 | DeepSeek 保留今日 Token 字段兼容未来接口，当前页面按接口实际返回的 `monthly_token_usage` / `monthly_usage` 展示本月 Token | `controller/deepseek_quota.go`、`model/deepseek_quota.go`、`web/default/src/features/quota-bindings` | 本次同步 |
| GLM / DeepSeek 额度 | 编辑额度配置时，未修改的 curl 和代理字段不提交，后端复用已保存配置，避免无回显或直接确认时清空代理 | `web/default/src/features/quota-bindings` | 本次同步 |
| 常规菜单 | 认证文件、GLM 额度和 DeepSeek 额度移动到常规菜单，普通用户可进入查看并刷新，编辑、新建、删除仍限管理员 | `controller/cliproxy.go`、`router/api-router.go`、`web/default/src/hooks/use-sidebar-*`、`web/default/src/features/*quota*`、`web/default/src/features/cliproxy-auth-files` | 本次同步 |
| 渠道管理 | 渠道支持开放用户限制；未配置时默认开放给所有用户 | `controller/channel.go`、`model/channel.go`、`middleware/distributor.go`、`service/channel_select.go`、`web/default/src/features/channels` | `001cd50f` |
| 渠道管理 | 在 `web/default` 实现渠道开放用户限制，支持编辑抽屉搜索选择用户、列表展示开放用户范围 | `web/default/src/features/channels` | 本次同步 |
| 用户管理 | 新增每日、每周、每月可使用 Token 数量限制 | `model/user_token_limit.go`、`middleware/distributor.go`、`controller/user.go`、`web/default/src/features/users` | `ef3f665e` |
| 用户管理 | 在 `web/default` 实现用户周期 Token 限制，创建/编辑用户时可维护每日、每周、每月 Token 上限，列表展示限制状态 | `web/default/src/features/users` | 本次同步 |
| 用户管理 | 注销用户支持管理员恢复 | `controller/user.go`、`model/user.go`、`web/default/src/features/users` | `a8c7d4c7` |
| 使用日志 | 支持 IP 模糊搜索，并在用户名称下显示用户备注 | `controller/log.go`、`model/log.go`、`web/default/src/features/usage-logs` | `7a9077cf` |
| 使用日志 | 支持输入开始时间和结束时间后计算平均耗时，并改为点击查询按钮后触发 | `controller/log.go`、`model/log.go`、`web/default/src/features/usage-logs` | `0f2ce33f`、`81a499ab`、`6de8ec1f` |
| 使用日志 | 在 `web/default` 实现 IP 筛选和平均耗时统计，高级筛选支持 IP，统计卡展示平均耗时 | `web/default/src/features/usage-logs` | 本次同步 |
| 使用日志 | 通用日志用户列显示用户备注、恢复 IP 列，并按日志 `use_time` 秒口径展示平均耗时 | `model/log.go`、`web/default/src/features/usage-logs` | 本次同步 |
| 使用日志 | 通用日志统计卡 RPM/TPM 按当前筛选时间窗口计算每分钟平均值，无筛选区间时保留最近 60 秒实时口径 | `model/log.go`、`web/default/src/features/usage-logs` | 本次同步 |
| 使用日志 | 通用日志统计卡的用量、RPM、TPM、平均耗时完整跟随模型、分组、令牌、用户、IP、请求 ID 和上游请求 ID 筛选动态计算 | `controller/log.go`、`model/log.go`、`web/default/src/features/usage-logs` | 本次同步 |
| 使用日志 | 通用日志耗时 badge 按实际耗时秒数阈值变色，不再受输出吞吐率影响 | `web/default/src/features/usage-logs` | 本次同步 |
| Token 展示 | Token 数值统一使用中文短单位展示，关键表格和卡片悬浮显示精确 token 详情 | `web/default/src/lib/format.ts`、`web/default/src/features/dashboard`、`web/default/src/features/user-consumption`、`web/default/src/features/quota-bindings`、`web/default/src/features/usage-logs` | 本次同步 |
| 数据看板 | 用户消耗排行默认以 Tokens 为单位；数据看板显示用户备注 | `web/default/src/features/dashboard`、`web/default/src/features/dashboard/hooks`、`web/default/src/features/dashboard/lib` | `bed9fbd8`、`d3ebbbb8` |
| 数据看板 | 数据看板新增令牌消耗排行 tab，按令牌聚合 Token 消耗并显示排行 | `web/default/src/features/dashboard`、`web/default/src/features/dashboard/hooks`、`web/default/src/features/dashboard/lib` | 本次同步提交 |
| 数据看板 | 在 `web/default` 实现令牌消耗分析，包含令牌趋势、分布和排行图表，并提供 `/dashboard/tokens` 入口 | `web/default/src/features/dashboard`、`web/default/src/hooks/use-sidebar-data.ts` | 本次同步 |
| 数据看板 | 用户统计显示当前统计时间区间，并支持自定义开始/结束时间筛选计算数据 | `web/default/src/features/dashboard/components/users`、`web/default/src/features/dashboard/lib` | 本次同步 |
| 数据看板 | 用户统计快捷区间会同步填充开始/结束时间控件，时间控件独立显示在筛选行上方，避免横向挤满筛选条 | `web/default/src/features/dashboard/components/users`、`web/default/src/features/dashboard/lib` | 本次同步 |
| 数据看板 | 用户统计时间筛选切换后强制刷新排行和趋势图表实例，避免图表复用旧时间窗口数据 | `web/default/src/features/dashboard/components/users` | 本次同步 |
| 数据看板 | 普通用户可访问用户统计 tab，并查看全量用户聚合统计 | `controller/usedata.go`、`router/api-router.go`、`web/default/src/features/dashboard` | 本次同步 |
| 用户消耗 | 修复 ClickHouse 日志库与主库分离时用户消耗查询跨库 JOIN 主库 `users` 表导致报错的问题，改为日志库聚合后回主库补充用户、渠道和认证文件信息 | `model/cliproxy_auth_file.go`、`model/cliproxy_user_consumption_test.go` | 本次同步 |
| 用户消耗 | 用户消耗 token 聚合表保留用户备注，用户名下展示备注，悬浮展示完整用户名和备注 | `model/cliproxy_auth_file.go`、`web/default/src/features/user-consumption` | 本次同步 |
| 用户消耗 | 管理员用户消耗菜单列表上方新增令牌消耗排行图，按当前筛选条件拉取并聚合令牌 Token 消耗 | `web/default/src/features/user-consumption` | 本次同步 |
| 用户消耗 | 管理员用户消耗菜单补齐快捷日期筛选和开始/结束时间选择，排行图、统计卡和列表共用当前筛选条件 | `web/default/src/features/user-consumption` | 本次同步 |
| 真实 IP / 反代 | 支持 nginx 反代真实 IP 记录，并补充 host 3000 反代示例配置 | `middleware`、`setting`、`docs/installation/nginx-new-api-3000.conf` | `d5233257`、`333c84b1` |
| 流式诊断 | 补充流式转发断开来源诊断日志，记录 request_id、model、elapsed、chunk_count 和请求上下文错误 | `relay` / stream forward 相关代码 | `26bafa7b` |
| Cliproxy 认证文件 | 兼容认证文件额度、备注回显、备注字段统一为 note，并调整绑定刷新权限 | `controller/cliproxy*`、`model/cliproxy*`、`web/default/src/features/cliproxy-auth-files` | `f09f1beb`、`aef05809`、`9c6da6db`、`651370c0` |
| Cliproxy 认证文件 | 认证文件和用户消耗相关菜单补齐 `web/default` i18n key，避免缺少语言包文案 | `web/default/src/i18n/locales` | 本次同步 |
| Cliproxy 认证文件 | 认证文件绑定列表精简用量列，仅默认展示主窗口进度，Codex 窗口、重置时间、token/quota 和错误详情改为悬浮展示 | `web/default/src/features/cliproxy-auth-files` | 本次同步 |
| Cliproxy 认证文件 | 普通用户只能查看并刷新自己的认证文件绑定，管理员才可配置服务、拉取远程认证文件、新建绑定、编辑和删除绑定 | `controller/cliproxy.go`、`model/cliproxy_auth_file.go`、`web/default/src/features/cliproxy-auth-files` | 本次同步 |
| 游乐场 | 恢复图片生成和 TTS 生成模式，复用当前模型/分组，生成结果写回对话消息 | `router/relay-router.go`、`web/default/src/features/playground` | 本次同步 |
| 游乐场 | 图片/TTS 模式下重试和编辑后提交继续走对应生成接口，图片和音频结果直接渲染为媒体控件，兼容裸 base64 返回，并按登录用户隔离本地会话历史 | `web/default/src/features/playground` | 本次同步 |
| 游乐场 | 图片生成历史保留 data URL 内容并增加内存兜底，避免切换菜单后因 base64 图片过大导致聊天消息丢失 | `web/default/src/features/playground/lib/storage` | 本次同步 |
| 构建上下文 | Docker 构建忽略运行态文件，缩小构建上下文 | `.dockerignore` | `c6c55020` |

## 验证记录

最近一次同步验证：
- `cd web/default && bun test`
- `cd web/default && bun oxlint -c .oxlintrc.json src/features/cliproxy-auth-files/index.tsx src/features/playground/components/input/playground-input.tsx src/features/user-consumption/components/token-consumption-charts.tsx src/hooks/use-sidebar-data.ts src/features/dashboard/components/models/token-stat-cards.tsx src/features/quota-bindings/index.tsx src/features/user-consumption/index.tsx`
- `cd web/default && bun run typecheck`
- `cd web/default && bun run build`
- `go test ./model ./controller -count=1`
- `git diff --check`
