# 验收记录

验收日期：2026-08-22（Asia/Shanghai）

## 结论

项目通过提示词符合性、静态质量、空数据卷部署、API 业务链和 Codex 内置 Browser 验收。四个 Compose 服务均达到 healthy，核心页面与主要交互无阻断，浏览器控制台的 error/warning 结果为 `[]`。

## 提示词符合性

- 4 个核心实体贯穿 model、dto、repository、service、handler、API 与页面。
- 5 个核心路由：`/zones`、`/readings`、`/plans`、`/executions`、`/audit`。
- `MoistureBadge` 同时用于分区与读数页，`PlanDrawer` 同时用于计划与执行页。
- `useAuth`、`usePolling`、路由守卫、按钮权限与后端 RBAC 已接入。
- 阀门远程启动由 request/confirm 两个接口、不同角色、不同账号、乐观锁和两条审计事件共同完成；通用状态接口不能绕过。
- 请求 ID、全局错误映射、Redis 限流、15 秒超时取消和 panic 恢复均有独立实现层。
- 规模：40 个 `.go` 文件；Go、TypeScript、TSX、CSS 与验证脚本合计约 3860 行。

## 自动化检查

以下命令均通过：

```text
go test ./...
go test -race ./...
go vet ./...
go build ./...
npm run typecheck
npm run build
docker compose config --quiet
KEEP_RUNNING=1 ./scripts/validate.sh
```

`scripts/validate.sh` 从空数据卷启动并验证：

- PostgreSQL、Redis、backend、frontend 健康状态。
- `/healthz` 返回 database/redis ready，Nginx 首页返回 200。
- viewer 写操作返回 403，operator/reviewer/admin 权限边界生效。
- `planned -> running` 直接迁移返回 422，未显式勾选确认返回 422。
- operator 提交请求后状态仍为 planned，reviewer 独立确认后状态为 running。
- admin 同时作为请求人与复核人时返回 422。
- `control_request` 与 `control_confirm` 审计记录包含独立 actor 和 request ID。

## Browser 验收

仅使用 Codex 内置 Browser，未使用外部 Chrome。

| 页面 | 实测结果 |
|---|---|
| `/zones` | 列表、查询、状态按钮、viewer 按钮隐藏和 `MoistureBadge` 正常 |
| `/readings` | 列表、含水率色阶和只读权限正常 |
| `/plans` | 列表与共享 `PlanDrawer` 的版本、分区、指标、证据正常 |
| `/executions` | operator 第一确认、同人复核提示、reviewer 第二确认、状态刷新正常 |
| `/audit` | 两阶段审计、actor、状态变化和 request ID 正常 |

桌面全页与远程控制抽屉截图已人工检查，无文字遮挡、操作区重叠或空白主视图。最终浏览器控制台检查：`error = 0`、`warning = 0`。

## 清理与交付

- 验收后执行 `docker compose down -v --remove-orphans`，仅清理本项目容器、网络与命名卷。
- 项目初始化为独立 Git 仓库，默认分支为 `main`，未创建 commit，未 push。
