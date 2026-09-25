# 温室灌溉策略执行控制

设施农业分区、土壤读数、灌溉策略和阀门执行平台。项目采用前后端分离和明确的领域分层，重点保证状态迁移、RBAC、审计日志、请求追踪与限流在各层保持一致。

## Docker Compose 快速启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

- Web 工作台：http://127.0.0.1:18513
- 后端健康检查：http://127.0.0.1:19513/healthz
- 后端 API：http://127.0.0.1:19513/api

演示账号统一使用密码 `Admin123!`：

| 账号 | 角色 | 用途 |
|---|---|---|
| `viewer` | 只读观察员 | 查询业务数据与审计 |
| `operator` | 现场操作员 | 新建记录、提交远程启动请求 |
| `reviewer` | 质量复核员 | 复核计划、独立确认远程启动 |
| `admin` | 系统管理员 | 全部治理操作，但仍不能自我完成双确认 |

这些账号仅用于本地演示，生产环境必须删除或更换密码。

停止并清理本项目容器与数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

| 业务模块 | 后端实体 | API 前缀 | 状态流 |
|---|---|---|---|
| 温室分区 | `GreenhouseZone` | `/api/zones` | active, dry, wet, locked |
| 土壤读数 | `SoilReading` | `/api/readings` | fresh, validated, anomalous, expired |
| 灌溉计划 | `IrrigationPlan` | `/api/plans` | draft, approved, scheduled, completed |
| 阀门执行 | `ValveExecution` | `/api/executions` | planned, running, succeeded, failed |

- JWT 登录和 viewer/operator/reviewer/admin 四级 RBAC。
- 所有状态变化使用乐观锁并写入不可覆盖的审计日志。
- 请求 ID、结构化日志、全局错误映射、15 秒超时取消和 Redis 分布式限流。
- 提供脱敏运行配置、当前会话、审计汇总和单实体审计历史接口。
- `MoistureBadge` 在分区和读数页统一展示含水率，`PlanDrawer` 在计划和执行页复用。
- `useAuth` 提供安全会话与演示角色切换，`usePolling` 每 30 秒刷新工作台数据。
- 阀门执行强制关联温室分区与灌溉计划；复核人可通过 `GET /api/executions/:id/control-check` 预览同分区运行中任务、最近一次已校验读数（30 分钟有效期）和计划停灌线。

## RBAC 与远程启动红线

| 操作 | 允许角色 |
|---|---|
| 查询全部业务与审计 | viewer/operator/reviewer/admin |
| 新建、编辑业务记录 | operator/admin |
| 分区、读数状态迁移 | operator/reviewer/admin |
| 计划状态复核 | reviewer/admin |
| 提交阀门远程启动请求 | operator/admin |
| 独立确认并启动阀门 | reviewer/admin，且不得与请求人相同 |
| 删除记录 | admin |

阀门执行不能通过通用状态接口直接从 `planned` 进入 `running`。操作员先调用 `POST /api/executions/:id/control-request`，另一位复核人先在控制详情（`GET /api/executions/:id/control-check`）核对以下三项，再调用 `POST /api/executions/:id/control-confirm`；两步都要求显式确认、正确版本号，并分别写入 `control_request`、`control_confirm` 审计事件：

1. **同分区运行中任务**：同一 `zoneCode` 下若已有 `running` 的阀门执行（该分区可能正在浇水），判定冲突。
2. **最近一次已校验读数**：取该分区最新一条 `validated` 土壤读数；没有已校验读数，或读数时间早于确认时刻 30 分钟（防止夜班拿半小时前的读数启动），判定冲突。
3. **计划停灌线**：关联灌溉计划必须属于该分区并设置了 `stopMoisture`（含水率停灌线，%）；读数含水率达到或超过停灌线，判定冲突。

存在冲突时执行状态保持 `planned`（待启动），不会进入双人启动；服务端在控制详情中写入冲突编号（如 `CFL-VE-002-001`，多项冲突追加 `-A/-B`）、读数时间、含水率和冲突说明，并写入 `control_blocked` 审计事件。冲突消除后复核人可重新确认；检查通过时阀门才进入 `running`，并保存当时的检查快照（复核人、复核时刻、读数编号/时间/含水率/读数年龄、停灌线、同分区运行中任务数）到 `controlCheckSnapshot`。阀门执行创建/更新时必须提供真实存在的 `zoneCode` 与属于该分区的 `planCode`。

## 技术栈

| 层次 | 技术 |
|---|---|
| 前端 | React 18 + TypeScript + Vite + Ant Design |
| 后端 | Go 1.22 + Gin + GORM |
| 数据 | PostgreSQL + Redis |
| 部署 | Docker Compose + Nginx |

## 本地开发

后端可使用 SQLite 开发模式，不需要先启动数据库：

```bash
cd backend
go mod download
DATABASE_DRIVER=sqlite DATABASE_DSN=local.db REDIS_ADDR='' \
JWT_SECRET=local-development-secret PORT=8080 go run ./cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

质量检查：

```bash
cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...
cd ../frontend && npm run typecheck && npm run build
cd .. && docker compose config --quiet
```

也可以从项目根目录执行 `./scripts/validate.sh`。脚本会从空数据卷构建并启动项目，检查健康状态、四角色 RBAC、远程双确认、自我复核阻断和审计事件，并在结束时关闭容器。设置 `KEEP_RUNNING=1` 可为浏览器验收保留服务。

## 目录结构

```text
.
├── backend/
│   ├── cmd/server/                 # 服务入口与优雅退出
│   └── internal/
│       ├── config/                 # 环境配置
│       ├── constants/              # 状态枚举与迁移图
│       ├── database/               # 连接、迁移与演示数据
│       ├── dto/                    # 输入契约
│       ├── handler/                # HTTP 接口
│       ├── middleware/             # JWT、追踪、限流、超时取消
│       ├── model/                  # GORM 实体
│       ├── repository/             # 持久化边界
│       ├── router/                 # 路由装配
│       ├── service/                # 业务规则与审计
│       └── util/                   # 统一 HTTP 响应
├── frontend/src/
│   ├── api/                        # 按实体拆分的 API
│   ├── components/                 # PlanDrawer 与共享业务组件
│   ├── hooks/                      # useAuth、usePolling、分页 hooks
│   ├── pages/                      # 五个路由页面
│   ├── router/                     # 路由配置
│   ├── stores/                     # 按实体拆分的状态仓库
│   ├── types/                      # 共享类型与枚举
│   └── utils/                      # 格式化与状态工具
├── docker-compose.yml
└── runtime_smoke.json
```

## 共享枚举位置

| 枚举 | 值 | 前后端出现位置 |
|---|---|---|
| `ZoneState` | `active, dry, wet, locked` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |
| `ExecutionState` | `planned, running, succeeded, failed` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |

每个实体自己的完整迁移图同样位于 `backend/internal/constants/status.go`；页面使用的状态列表位于 `frontend/src/types/status.ts`。修改状态时必须同步两处并更新对应服务测试。

## 环境变量

| 变量 | 说明 |
|---|---|
| `COMPOSE_PROJECT_NAME` | 固定英文 Compose 项目名，支持中文父目录 |
| `DB_NAME/DB_USER/DB_PASSWORD` | 数据库名称与业务账号 |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须替换 |
| `FRONTEND_PORT/BACKEND_PORT/DB_PORT` | 宿主机端口映射 |
| `REDIS_PORT` | Redis 宿主机端口 |

## API 使用示例

```bash
token=$(curl -sS -X POST http://127.0.0.1:19513/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin123!"}' | jq -r '.data.token')

curl -sS http://127.0.0.1:19513/api/overview \
  -H "Authorization: Bearer $token"
```

## License

MIT
