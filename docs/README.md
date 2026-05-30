# 项目冷启动入口

这份文档是新会话进入仓库时的第一入口。先看这里，再按任务去读代码。别靠聊天记忆猜项目状态。

## 项目定位

`infinite-canvas` 是面向图片创作的无限画布工作台：画布编排、AI 图片/视频生成、对话助手、提示词库、素材沉淀放在同一套前后端里。

当前技术栈：

- 后端：Go + Gin + GORM。
- 前端：Next.js App Router + React + TypeScript + Ant Design + Tailwind + Zustand。
- 部署入口：Docker / docker compose。

## 冷启动阅读顺序

1. `AGENTS.md`：读完本入口后先看项目规则，优先级高。
2. `docs/status/current-status.md`：当前真实状态和验证缺口。
3. `README.md`：面向用户的项目介绍和快速开始。
4. `docs/features.md`：已经写成正式说明的功能。
5. `docs/pending-test.md`：已实现但还需要用户测试确认的变更。
6. `docs/todo.md`：后续待办。
7. `docs/architecture/agent-framework.md`：需要 AI/agent 分工时读取。
8. `docs/audit/initial-review.md`：需要了解当前已知工程问题时读取。
9. 按任务选择专项文档：
   - 接口响应：`docs/api-response.md`
   - 数据库：`docs/backend-database.md`
   - 系统配置：`docs/system-settings.md`
   - 部署：`docs/deployment.md`
   - 待测验收：`docs/testing/pending-test-acceptance.md`
   - 画布操作：`docs/canvas-node-manual.md`、`docs/canvas-shortcuts.md`

## 运行时边界

- 画布项目和“我的素材”主要在浏览器本地持久化，默认使用 localForage；不要写成已支持云同步。
- 后端数据库保存用户、算力流水、提示词、服务器素材、系统配置等服务端数据。
- AI Key 有两条边界：用户本地直连配置保存在浏览器本地；后台渠道密钥保存在后端私有配置里，由后端代理接口使用。
- Docker 是当前部署入口；不要把生产部署、静态资源路径、云同步写成已完整验证，除非本轮真的验证过。

## 当前状态入口

当前状态只看：`docs/status/current-status.md`。

它应该记录“现在确认是什么”，不是计划、愿望或聊天里的口头结论。代码和文档冲突时，优先看当前运行代码与配置。

## 文档真相源顺序

遇到冲突时按这个顺序判断：

1. 当前运行代码和配置：`router/`、`handler/`、`service/`、`repository/`、`model/`、`web/src/`、`scripts/`、`Dockerfile`、`docker-compose*.yml`、`.env.example`、`render.yaml`。
2. 当前状态：`docs/status/current-status.md`。
3. 项目规则：`AGENTS.md`。
4. 专项文档：`docs/backend-database.md`、`docs/system-settings.md`、`docs/api-response.md` 等。
5. 面向用户说明：`README.md`、`docs/features.md`。
6. 待确认内容：`docs/pending-test.md`、`docs/todo.md`。

不要用旧聊天记录覆盖代码事实。

## 常用本地启动/验证入口

Docker 运行：

```bash
docker compose up -d
```

本地构建镜像运行：

```bash
docker compose -f docker-compose.local.yml up -d --build
```

前端源码开发入口：

```bash
cd web
npm run dev
```

后端源码入口：

```bash
go run .
```

基础验证入口：

- 页面默认端口：`http://localhost:3000`
- 后端健康检查：`/api/health`
- 接口响应格式：`docs/api-response.md`
- 数据库结构：`docs/backend-database.md`
- 待测验收手册：`docs/testing/pending-test-acceptance.md`

## 结束任务前检查

只要任务改了功能、接口、数据结构或用户可见行为，结束前检查：

- `docs/todo.md`：完成的待办要移走；新增真实待办要写进去。
- `docs/pending-test.md`：本轮可测试但未由用户确认的变更写这里。
- `docs/features.md`：只有用户确认稳定后，再把功能写成正式说明。
- `docs/backend-database.md`：新增或修改后端表结构必须同步。
- `docs/system-settings.md`：系统配置字段变化必须同步。
- `docs/api-response.md`：响应约定变化必须同步。
- `docs/status/current-status.md`：运行边界或验证状态变化必须同步。
- `docs/audit/initial-review.md`：发现新的结构性问题时补充证据、风险和第一步建议。

如果没有功能状态变化，就不要硬写文档。
