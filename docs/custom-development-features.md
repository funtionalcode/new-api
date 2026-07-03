# 二开功能记录

更新时间：2026-07-03

本文记录当前项目已做的二开功能，便于后续升级、排查和继续开发时快速确认改动范围。后续新增或调整二开功能时，需要同步更新本文。

维护规则：
- 记录功能名称、涉及范围、关键提交和验证方式。
- 不记录 curl、cookie、session、代理密码、SSH 密码等敏感信息。
- 只记录项目代码和配置层面的改动；临时服务器排查操作不作为长期功能记录。

## 功能清单

| 模块 | 功能 | 涉及范围 | 关键提交 |
| --- | --- | --- | --- |
| GLM / DeepSeek 额度 | 新增 GLM 和 DeepSeek 额度菜单，支持管理员配置 curl、刷新额度、普通用户只刷新 | `controller/*_quota.go`、`model/*_quota.go`、`router/api-router.go`、`web/classic/src/pages/*Quota` | `88600745` |
| GLM / DeepSeek 额度 | 额度配置支持代理地址，刷新时按配置代理请求；普通用户隐藏 curl 和 proxy | `controller/quota_http_client.go`、`controller/*_quota.go`、`model/*_quota.go`、`web/classic/src/pages/*Quota` | `68748b43` |
| 渠道管理 | 渠道支持开放用户限制；未配置时默认开放给所有用户 | `controller/channel.go`、`model/channel.go`、`middleware/distributor.go`、`service/channel_select.go`、`web/classic/src/components/table/channels` | `001cd50f` |
| 用户管理 | 新增每日、每周、每月可使用 Token 数量限制 | `model/user_token_limit.go`、`middleware/distributor.go`、`controller/user.go`、`web/classic/src/components/table/users` | `ef3f665e` |
| 用户管理 | 注销用户支持管理员恢复 | `controller/user.go`、`model/user.go`、`web/classic/src/components/table/users` | `a8c7d4c7` |
| 使用日志 | 支持 IP 模糊搜索，并在用户名称下显示用户备注 | `controller/log.go`、`model/log.go`、`web/classic/src/components/table/usage-logs` | `7a9077cf` |
| 使用日志 | 支持输入开始时间和结束时间后计算平均耗时，并改为点击查询按钮后触发 | `controller/log.go`、`model/log.go`、`web/classic/src/components/table/usage-logs`、`web/classic/src/hooks/usage-logs` | `0f2ce33f`、`81a499ab`、`6de8ec1f` |
| 数据看板 | 用户消耗排行默认以 Tokens 为单位；数据看板显示用户备注 | `web/classic/src/components/dashboard`、`web/classic/src/helpers/dashboard.jsx`、`web/classic/src/hooks/dashboard` | `bed9fbd8`、`d3ebbbb8` |
| 真实 IP / 反代 | 支持 nginx 反代真实 IP 记录，并补充 host 3000 反代示例配置 | `middleware`、`setting`、`docs/installation/nginx-new-api-3000.conf` | `d5233257`、`333c84b1` |
| 流式诊断 | 补充流式转发断开来源诊断日志，记录 request_id、model、elapsed、chunk_count 和请求上下文错误 | `relay` / stream forward 相关代码 | `26bafa7b` |
| Cliproxy 认证文件 | 兼容认证文件额度、备注回显、备注字段统一为 note，并调整绑定刷新权限 | `controller/cliproxy*`、`model/cliproxy*`、`web/classic/src/pages/CliproxyAuthFiles` | `f09f1beb`、`aef05809`、`9c6da6db`、`651370c0` |
| 构建上下文 | Docker 构建忽略运行态文件，缩小构建上下文 | `.dockerignore` | `c6c55020` |

## 验证记录

最近一次额度代理改造验证：
- `go test ./controller ./model`
- `git diff --check`
- `cd web/classic && bun run build` 未通过：本地缺少 `vite`，报 `vite: command not found`
