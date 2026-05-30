# 当前状态

本文只记录当前能从仓库文件和代码推断出的事实。不要把计划、猜测或未验证部署写成已完成。

## 项目形态

- 前端：Next.js App Router + React + TypeScript + Ant Design + Tailwind + Zustand。
- 后端：Go + Gin + GORM。
- 对外页面默认由 Next.js 提供，Docker 镜像里 Go API 在容器内部启动，`/api/*` 由前端运行时代理到后端。
- Docker / docker compose 是当前部署入口；源码也可以按前后端开发入口分别启动，但这不等于生产部署已验证。

## 数据边界

当前数据不是单一存储。

- 浏览器本地：画布项目、“我的素材”、生成记录、用户本地 AI 渠道配置等前端业务数据，默认使用 localForage；少量简单配置才适合 localStorage。
- 后端数据库：用户、算力流水、提示词、服务器素材库、系统配置。
- “我的素材”和服务器“素材库”不是一回事：前者主要在浏览器本地，后者走后端 DB。
- 当前不要写成已支持账号级画布云同步或素材云同步。

## 后端数据库

后端通过 GORM 初始化数据库。默认 `DB_AUTO_MIGRATE=false`，服务启动不主动改表；初始化或发布时用 `go run . migrate` 或镜像内 `/app/server migrate` 显式迁移。普通启动会先检查当前迁移表和字段是否存在，未迁移或旧 schema 缺字段时直接失败并提示先执行显式迁移，不再依赖默认管理员初始化间接暴露缺表问题。开发想省步骤时可临时设为 `DB_AUTO_MIGRATE=true`。

- 支持驱动：`sqlite`、`mysql`、`postgres` / `postgresql`。
- 默认驱动：`mysql`。
- Docker Compose 默认只启动应用容器，并通过 `MYSQL_DSN` 连接宿主机已有 MySQL，例如 `host.docker.internal:3307` 上的 `infinite_canvas` 数据库。
- `STORAGE_DRIVER` 为默认值或 `mysql` 时，`MYSQL_DSN` 会覆盖 `DATABASE_DSN`；MySQL 连接池通过 `MYSQL_MAX_OPEN_CONNS`、`MYSQL_MAX_IDLE_CONNS`、`MYSQL_CONN_MAX_LIFETIME` 配置。
- `render.yaml` 仍显式使用 `sqlite`，只适合体验和演示。
- 当前迁移表：`users`、`credit_logs`、`prompts`、`assets`、`settings`。
- 详细字段以 `docs/backend-database.md` 为准。

## API 和路由

当前路由入口在 `router/router.go`，路由注册已按公开接口、认证接口、AI 代理和后台管理接口拆成私有注册函数。

- 健康检查：`GET /api/health`，返回 `ok`。
- 认证：注册、登录、Linux.do OAuth、当前用户。
- 用户侧模型代理：`POST /api/v1/images/generations`、`POST /api/v1/images/edits`、`POST /api/v1/chat/completions`、`POST /api/v1/videos`、`GET /api/v1/videos/:id`、`GET /api/v1/videos/:id/content`。
- 公开资源：提示词、素材、公开设置。
- 管理后台：用户、算力流水、系统设置、提示词分类、提示词、素材库。
- 业务响应约定看 `docs/api-response.md`。
- 兼容旧业务接口，普通业务失败仍可能是 HTTP 200 + `code != 0`；明确的鉴权、登录失败、注册和后台用户保存校验、Linux.do OAuth 配置和上游失败、后台渠道配置缺项或格式错误、后台渠道网络、上游 HTTP 错误、HTTP 200 JSON 业务错误、非法 JSON 响应或测试模型响应缺少内容、AI 算力点不足、参数错误和 AI 上游失败已经开始返回 401/403/402/400/404/409/502，并仍保留 `{ code, data, msg }` JSON 结构。后台渠道接口地址和 API Key 会在保存或临时测试时裁掉前后空白；编辑已有渠道时空白 API Key 会沿用已保存密钥。`FailError` 会通过 `errors.As` 识别直接或 `%w` 包装后的 typed service error，避免包装后丢失安全文案和 HTTP 状态。

## AI API Key 和渠道边界

当前有两种模型调用模式，别混写。

- 本地直连：用户在浏览器本地配置 OpenAI 兼容 `baseUrl`、`apiKey` 和模型列表，由前端直接请求上游。AI API Key 存在浏览器本地。
- 云端渠道：管理员在后端私有配置里维护渠道，后端按模型名和权重选择渠道，再通过 `/api/v1/*` 代理调用。渠道密钥属于后端私有配置。
- `allowCustomChannel` 控制用户侧是否允许本地直连；关闭后用户侧应走后端渠道。
- 云端渠道 POST 请求已接入按模型扣算力点和失败退款逻辑，并有本地回归测试覆盖；真实渠道、客户端中断和故障注入验收仍以 `docs/pending-test.md` 为准。当前覆盖的失败边界包括上游网络失败、上游 HTTP `>=400`、上游 HTTP 200 但 JSON 业务体失败、视频创建响应缺任务 ID、视频创建后消费流水绑定任务 ID 失败、请求生成数量 `n` 超大时硬封顶并防止算力点乘法溢出、以及上游成功但响应写给客户端失败。
- HTTP 200 业务失败识别不只依赖标准 JSON `Content-Type`，上游漏写或误写成 `text/plain` 但响应体是 JSON 错误时也会按失败处理；顶层 `error`、数字或字符串 `code` 非成功值、`status=error/failed/cancelled`、`success=false` 都会被视为业务失败；较大的 JSON 响应会在明确上限内完整校验，不再只按 64KB 前缀判断视频任务 ID 或失败状态；`stream=true` 的请求不会为了探测错误预读成功 SSE 长连接，避免破坏流式输出；大请求体中靠后的 `stream=true` 也会被识别；流式请求返回有明确长度的短 JSON 错误体时，仍会按业务失败处理并退款；视频创建/轮询响应的校验也不依赖上游正确设置 JSON `Content-Type`。
- AI 消费扣点和消费流水写入已改为同一数据库事务；AI 普通退款、视频任务退款与对应退款流水写入也已改为同一数据库事务；后台手动调整算力点的余额更新和 `admin_adjust` 流水写入同样已改为同一事务。这些事务边界有本地回归测试覆盖，真实故障注入仍在待测项里。
- 视频 POST 创建成功后会把消费流水绑定到视频任务 ID，绑定更新会检查消费流水确实存在；即使视频模型配置为 0 算力点，也会记录 0 金额消费流水用于任务渠道绑定；后续轮询到同一任务 `failed` / `cancelled` 时，会按任务 ID 幂等退款，视频退款流水 ID 由用户 ID 和任务 ID 确定性派生，避免重复轮询重复返还；退款流水里的模型名优先来自创建时消费流水，不信任轮询 URL 里的 `model` 参数。
- 视频任务的轮询和内容下载已接入创建时绑定渠道的优先选择；多渠道同模型时，不再只靠模型名重新随机选择渠道；如果多个渠道共享同一个 base URL，会优先按创建时记录的渠道名称精确匹配；绑定渠道匹配会把 `https://host` 和 `https://host/v1` 视为等价，避免管理员把等价 base URL 补上 `/v1` 后旧任务失效；当前用户没有该任务绑定消费流水、已绑定任务找不到对应渠道或绑定渠道 URL 无法构造请求时，会失败关闭并返回网关错误，不随机退回其他同模型渠道。真实多渠道环境仍需要按待测项验收。

## 部署状态

- `docker-compose.yml` 使用 `ghcr.io/basketikun/infinite-canvas:latest`。
- `docker-compose.local.yml` 从本仓库 Dockerfile 构建 `infinite-canvas:local`。
- Dockerfile 是多阶段构建：先用 Bun 和 `web/bun.lock` 构建 Next.js，再构建 Go 后端；运行镜像用 `node /app/scripts/docker-start.mjs` 先执行 `/app/server migrate`，再同时启动内部 Go API 和已构建的 Next.js，对外暴露 3000。
- Docker 运行脚本已接入 Go API 和 Next.js 两个子进程监督，并有 Node 内置测试覆盖；真实容器内迁移、长期运行和子进程退出验收仍见 `PT-DOCKER-001`。
- 源码前端开发入口仍是 `web/package.json` 里的 npm 脚本；Docker 运行阶段不执行 `npm install`，也不通过 `npm run start` 间接启动。
- 当前只能说 Docker 是部署入口；不要声称生产部署、静态资源路径、长期运行都已验证。

## 待测状态

`docs/pending-test.md` 是本版本待测台账，不是正式功能说明。

- 当前已有待测项已经按模块、状态、重点验证项整理。
- 待测执行步骤集中在 `docs/testing/pending-test-acceptance.md`。
- 不依赖外部 AI Key 的画布 zip 导入导出、画布图片撤销恢复、“我的素材” zip 导入导出和未登录顶部导航已从 `docs/pending-test.md` 移出；后续仍按 `docs/testing/pending-test-acceptance.md` 保留的步骤做回归。
- storageKey-only 图片节点本地恢复问题已经用本地浏览器红绿脚本复现并修复；图片节点现在会先按 `storageKey` 恢复，再回退到旧 `content` 逻辑。
- 画布打开时的媒体补水逻辑已改为失败隔离：单个图片或视频本地媒体恢复失败时会保留该节点并标记错误，storageKey-only 媒体丢失或旧 `blob:` fallback 失效不会被误当作恢复成功，画布助手消息里的单个图片恢复失败也会保留原引用，不再让整个画布加载流程失败；真实项目验收见 `PT-CANVAS-006`。
- 画布生成运行态已改为节点集合而不是单个 `runningNodeId` 字符串：多个异步生成任务并发时，任一任务结束只会移除自己的运行态，不会把其它仍在运行的任务提前清空；生成前参考媒体恢复失败也不会把当前任务永久卡在运行中；参考图补水为空会按“参考图片已丢失”失败处理，前端参考图转文件入口也会拒绝空数据，不再继续发送空参考文件。真实并发 UI 验收见 `PT-CANVAS-007`。
- 前端已新增 `typecheck` 脚本，当前 `cd web && npm run typecheck` 通过。
- 前端已新增最小 `test` 脚本，当前 `cd web && npm run test` 通过；覆盖范围包括画布媒体补水纯逻辑、节点和助手图片媒体恢复失败隔离、生成运行态集合清理规则、参考图补水为空拒绝继续生成、空参考图拒绝转成上传文件、AI 文本接口、视频生成接口默认优先使用视频模型、显式视频模型覆盖不被全局 `videoModel` 吞掉、非 2xx JSON 错误提取、JSON blob 错误拒绝、通用 MIME 小 JSON 错误 blob 拒绝、取消提示保留和视频轮询超时。
- 后端已新增 Go 回归测试，覆盖受保护路由未登录返回 401、AI POST 代理坏请求返回 400、缺用户上下文返回 401、退款缺失用户返回 404、后台渠道兜底上游错误返回 502、后台渠道 HTTP 200 JSON 业务错误、非法 JSON 响应和测试模型响应缺少内容返回 502、后台渠道接口地址格式错误和缺测试模型名返回 400、后台渠道接口地址和 API Key 前后空白裁剪、编辑已有渠道时空白 API Key 沿用保存密钥、AI 生成数量超大时封顶并防溢出、负数算力点消费拒绝、视频任务无当前用户绑定时不 fallback 到默认渠道、绑定渠道 base URL 的 `/v1` 等价匹配、绑定渠道 URL 无效时返回 502、数据库首次打开失败后可重试且测试中配置变化会重新打开连接、提示词同步 scheduler 可停止并重新启动；当前 `go test -p 1 -vet=off ./... -count=1` 通过。
- Docker 启动 supervisor 已新增 Node 内置测试，覆盖 API / Web 任一子进程退出时终止另一个子进程，以及 SIGTERM 转发；当前 `node --test scripts/docker-start.test.mjs` 通过。
- AI 代理已新增响应拷贝失败、HTTP 200 `error` / 数字或字符串 `code` 非成功值 / `status=error/failed/cancelled` / `success=false` 业务失败、HTTP 200 JSON 错误但缺 JSON `Content-Type`、超过 64KB 的 JSON 业务失败和视频创建成功响应、`stream=true` 大请求体识别和成功 SSE 不预读、流式短 JSON 错误退款、生成数量超大防扣费溢出、无可用模型渠道返回 502、视频创建缺任务 ID 退款、视频创建响应缺 JSON `Content-Type` 时仍校验任务 ID、视频任务绑定失败退款、0 算力点视频任务仍绑定渠道、消费流水失败回滚、视频任务最终失败幂等退款、视频任务绑定渠道同 base URL 多渠道精确匹配和 `/v1` 等价匹配的回归测试，防止扣费后用户拿不到有效结果。
- 当前 `git diff --check` 通过，未发现空白错误。
- 现有待测条目当前状态均为 `待测`，主要剩余项依赖模型配置、后端渠道或管理员配置环境。
- 用户确认通过后，再从 `docs/pending-test.md` 移出，并按需要更新 `docs/features.md` 或 `CHANGELOG.md`。
- 用户测出问题后，先在 `docs/pending-test.md` 标记 `已测失败`，再拆成修复切片或写入 `docs/todo.md`。

## 已知验证缺口

- 本文不把构建或全量浏览器回归写成已验证；项目规则要求不要在用户未要求时主动执行构建。
- 前端当前只有最小 Vitest 单测，不能替代 React DOM、Playwright 或真实模型渠道回归。
- 后端当前已有状态码、路由面、显式迁移、启动前迁移检查、AI 算力点结算和后台调点事务回归测试；但还不是完整后端测试体系，少量旧 `FailError` service 错误仍缺 typed status 映射。
- credit log 仍缺完整 request/task trace id。
- 生产部署未在本文中验证。
- Docker 静态资源路径仍按项目规则视为待确认点，不要过度承诺。
- 画布项目和“我的素材”没有确认支持账号云同步。
- 移动端触控体验、多人公网使用、历史数据兼容都不要当成已稳定能力。
- `docs/pending-test.md` 的剩余当前条目仍需要带模型渠道或管理员环境的实测确认。

## 结束任务时的文档判断

- 功能行为变了：先更新 `docs/pending-test.md`，用户确认后再进 `docs/features.md`。
- 后端表结构变了：同步 `docs/backend-database.md`。
- 系统配置字段变了：同步 `docs/system-settings.md`。
- API 响应约定变了：同步 `docs/api-response.md`。
- 只是新增冷启动或状态说明：不需要把 `docs/todo.md` 或 `docs/pending-test.md` 硬塞一条假变更。
