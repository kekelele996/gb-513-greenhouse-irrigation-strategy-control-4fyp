请生成 `greenhouse-irrigation-strategy-control`「温室灌溉策略执行控制」Go 全栈项目，面向设施农业企业根据分区、土壤读数和作物阶段编排灌溉策略，并管理阀门执行确认。不要做农产品电商、库存、订单或记账。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`GreenhouseZone`（分区与作物）、`SoilReading`（含水率/盐度读数）、`IrrigationPlan`（策略版本）、`ValveExecution`（执行与反馈）贯穿数据库、Go 分层和前端。

### 核心页面

`/zones` 分区；`/readings` 土壤读数；`/plans` 灌溉计划；`/executions` 阀门执行；`/audit` 审计。`MoistureBadge` 在分区和读数页共用，`PlanDrawer` 在计划和执行页共用。

### 横切关注点

RBAC 与远程控制双确认联动角色、middleware、路由守卫和按钮显隐；策略版本及执行结果写审计和 request ID；全局错误处理、限流、超时取消必须有独立层。

### 共享枚举/组件

同步 `ZoneState`（active/dry/wet/locked）与 `ExecutionState`（planned/running/succeeded/failed）。共享 `StatusBadge`、`MetricCard`、`ConfirmDialog`，hooks 为 `useAuth`、`usePolling`。

### 技术与规模要求

前端 React 18 + TypeScript + Vite + Ant Design；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis。目标 2800–4000 行、28–40 个 `.go` 文件。

### 文件结构强制清单

前端 `api/stores/types/components/common/hooks/pages/router/utils`；后端 `model/dto/repository/service/handler/router/middleware/constants/util`，每个实体独立文件。

### 结构红线

严禁合并职责到单一文件；远程控制必须由多个后端和前端层共同完成。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: greenhouse-irrigation-strategy-control`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=greenhouse-irrigation-strategy-control`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18513`、后端端口 `19513`；Nginx `/api` 反代，数据库 healthcheck、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
