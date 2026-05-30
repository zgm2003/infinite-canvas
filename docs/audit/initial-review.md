# 初始项目问题审查台账

本台账起初基于只读范围内的文件取证：`AGENTS.md`、`README.md`、`docs/*.md`、`router/router.go`、`repository/db.go`、`handler/response.go`、`handler/ai.go`、`service/auth.go`、`service/settings.go`、`web/package.json`、`Dockerfile`、`web/src/services/api/request.ts`、`web/src/app/api/[...path]/route.ts`，以及前端最大页面/组件列表。后续“处理结果 / 验证”会补充实际修改和测试证据；未读代码不扩展新结论。

## P0

本轮原始只读审查没有发现需要立刻阻断开发的 P0；后续 diff 审查发现并修复了一个计费绕过级问题：客户端 `n` 超大可能导致算力点乘法溢出。当前更值得持续治理的是下面这些 P1：它们不是“吓人”的安全报告，而是会直接影响数据、扣费、发布判断和维护成本的工程问题。

## P1

### P1-1 AI 代理的扣减/退款边界太粗（已开始治理）

- 原现象：AI POST 请求在转发前先扣算力点，上游网络错误或 HTTP `>=400` 时退款；但异步视频任务创建成功后后续失败、上游 200 但业务体表示失败、响应拷贝中途失败等场景没有明确结算边界。
- 处理结果：`copyAIResponse` 不再忽略 `io.Copy` 写出错误；当上游已返回 2xx 但响应写给客户端失败时，会记录 URL、已写字节数和错误，并触发同一套失败回调，从而执行退款。云端渠道 POST 还会检查 HTTP 200 JSON 业务体里的 `error`、顶层数字或字符串 `code` 非成功值、`status=error/failed/cancelled`、`success=false` 等业务失败；即使上游漏写 JSON `Content-Type` 或误写成 `text/plain`，只要响应体是 JSON 错误也会按失败退款。`stream=true` 请求不会为了探测错误预读成功 SSE 长连接，避免破坏流式输出；大请求体里靠后的 `stream=true` 也会被识别；流式请求返回有明确长度的短 JSON 错误体时，仍会按业务失败处理并退款。视频创建响应如果没有任务 ID，也会映射为 HTTP `502` 并退款，避免前端拿不到可轮询任务却已经扣费。视频创建/轮询响应的成功校验不依赖上游正确设置 JSON `Content-Type`。
- 账务补充：AI 消费扣点和消费流水写入已合并进同一个数据库事务，避免余额已扣但消费流水写入失败；AI 普通退款和视频任务退款的余额返还与退款流水写入也已合并进同一个数据库事务，避免余额已返还但退款流水写入失败。
- 视频补充：`POST /api/v1/videos` 创建成功后会把消费流水 `related_id` 绑定到上游返回的任务 ID，并在流水 `extra` 里记录创建渠道名称和 base URL；即使视频模型配置为 0 算力点，也会记录 0 金额消费流水用于任务渠道绑定；如果任务 ID 绑定失败，会按本次失败创建退款，避免返回 502 后仍扣费；`GET /api/v1/videos/:id` 轮询到 `failed` / `cancelled` 时，会按该任务 ID 幂等退款，退款流水里的模型名优先使用创建时消费流水记录，不信任轮询 URL 里的 `model` 参数；视频退款流水使用用户 ID 和任务 ID 派生的确定性 ID，避免并发或重复轮询把同一任务重复返还；轮询和内容下载优先使用任务绑定渠道，避免多渠道同模型时查错渠道；如果多个渠道共享同一 base URL，会优先按绑定渠道名称精确匹配；base URL 末尾有无 `/v1` 会按等价处理；当前用户没有任务绑定流水、已绑定渠道缺失或绑定渠道 URL 无法构造请求时失败关闭并返回 502，不随机退回其他同模型渠道。
- 当前已覆盖的退款场景：上游网络失败、上游 HTTP `>=400`、上游 HTTP 200 但业务体失败、视频创建响应缺任务 ID、视频创建后消费流水绑定任务 ID 失败、上游成功但下游响应拷贝失败、视频任务最终失败或取消。
- 响应补充：AI 生成预扣算力点时，如果用户余额不足，`ConsumeUserCredits` 现在携带 HTTP `402`，不再通过 `FailError` 退化成 HTTP 200 + `code != 0`。
- 计数补充：客户端传入的生成数量 `n` 已做硬上限，算力点乘法前会检查整数溢出；service 层拒绝负数算力点消费，避免超大 `n` 造成不扣费、反向流水或负数消费。
- 验证：新增 `TestCopyAIResponseRefundsWhenClientWriteFails`、`TestCopyAIResponseRefundsWhenUpstreamBusinessErrorIsHTTP200`、`TestCopyAIResponseRefundsWhenUpstreamCodeErrorIsHTTP200`、`TestCopyAIResponseRefundsWhenUpstreamStringCodeErrorIsHTTP200`、`TestAIUpstreamBusinessFailureRecognizesErrorStatusAndSuccessFalse`、`TestCopyAIResponseRefundsWhenBusinessErrorLacksJSONContentType`、`TestCopyAIResponseDoesNotPreReadStreamingResponseWithoutContentType`、`TestCopyAIResponseDoesNotPreReadStreamingResponseWithJSONContentType`、`TestCopyAIResponseDoesNotPreReadLargeStreamingRequest`、`TestCopyAIResponseRefundsStreamingJSONBusinessError`、`TestAIVideoCreateWithoutTaskIDRefunds`、`TestAIVideoCreateWithoutJSONContentTypeStillValidatesTaskID`、`TestAIVideoCreateRefundsWhenTaskBindingFails`、`TestAIVideoQueryFailureRefundsBoundTaskOnce`、`TestAIVideoQueryUsesBoundTaskChannel`、`TestAIVideoQueryDoesNotFallbackWhenBoundChannelIsStale`、`TestAIVideoZeroCostCreateStillBindsTaskChannel`、`TestRefundVideoTaskCreditsUsesConsumeLogModel`、`TestVideoTaskChannelMatchesBoundNameWhenBaseURLIsShared`、`TestVideoTaskChannelFailsWhenBoundNameIsMissingFromSharedBaseURL`、`TestBindVideoTaskCreditLogFailsWhenConsumeLogIsMissing`、`TestConsumeUserCreditsRollsBackWhenCreditLogFails`、`TestRefundUserCreditsRollsBackWhenCreditLogFails`、`TestRefundVideoTaskCreditsRollsBackWhenCreditLogFails`、`TestRefundVideoTaskCreditsIsIdempotent` 和 `TestRefundVideoTaskCreditsDoesNotDoubleApplyDuplicateRefundLog`。
- 剩余风险：credit log 仍缺完整 request/task trace id。

### P1-2 AutoMigrate 直接绑定运行时启动（已开始治理）

- 原现象：数据库连接初始化时直接执行 `db.AutoMigrate`，覆盖用户、积分日志、提示词、素材、设置等表。
- 处理结果：已新增 `DB_AUTO_MIGRATE` 开关，默认 `false`；`repository.DB()` 默认只打开连接，不主动迁移。显式迁移入口为 `go run . migrate`，Docker 镜像启动脚本也先执行 `/app/server migrate`，再启动 Go API 和 Next.js。普通服务启动会先检查当前应用表和字段是否存在，缺表或旧 schema 缺字段时直接失败并提示显式迁移；这个检查独立于默认管理员初始化，禁用默认管理员也不能让空库或明显过期的 schema 静默启动。
- 兼容边界：开发环境如果想保留旧体验，可以临时设置 `DB_AUTO_MIGRATE=true`；但受控部署应先跑显式迁移，不再依赖服务启动时偷偷改 schema。
- 验证：`TestDBCanOpenWithoutAutoMigrateAndMigrateExplicitly` 覆盖关闭启动迁移后 `DB()` 不建表、`Migrate()` 建表；`TestEnsureMigratedRejectsStaleSchema` 覆盖表存在但字段缺失的旧 schema 会被启动前检查拦下；`TestDBRetriesAfterInitialOpenFailure` 覆盖首次打开失败后不会把错误永久锁死；`TestDBReopensWhenConfigChanges` 覆盖测试进程中数据库配置变化会重新打开连接，避免全局连接污染后续用例；`TestRunChecksMigrationBeforeServing` 覆盖未迁移空库启动失败、执行 `migrate` 后可启动的命令入口和启动前检查；`TestStopPromptSyncSchedulerStopsAndAllowsRestart` 覆盖提示词同步 scheduler 不会在测试或服务返回后留下不可重启的全局 cron。
- 剩余风险：当前仍使用 GORM `AutoMigrate` 作为迁移动作，没有版本化 migration 文件。下一步如果要支持严肃多人生产部署，应继续引入版本化迁移记录和回滚策略。

### P1-3 后端错误响应 HTTP 状态太弱（已开始治理）

- 原现象：`Fail` 只写 `{ code: 1 }` JSON，没有设置 HTTP 状态码；文档明确说业务失败也返回 HTTP 200。
- 处理结果：已保留 `Fail(w, msg)` 的旧行为，同时新增 `FailStatus(w, status, msg)`。鉴权中间件现在对未登录返回 `401`，对已登录但非管理员返回 `403`；AI POST 代理对坏请求返回 `400`，对缺用户上下文返回 `401`，对上游网络或上游 `>=400` 返回 `502`。`AdminLogin` 登录成功但非管理员返回 `403`。`FailError` 已开始支持 typed service error：service 错误实现 `StatusCode()` 时会返回对应 HTTP 状态；即使 typed error 被 `%w` 包装，安全文案和 HTTP 状态也不会退化成旧的 HTTP 200。登录用户名或密码错误现在携带 `401`，账号禁用携带 `403`；注册和后台保存用户的明确校验错误开始携带 `400/403/404/409`；Linux.do OAuth authorize 和 callback 都会在未开启、缺配置和上游登录失败时携带 `403/400/502`，缺配置时不会继续请求上游；后台渠道缺接口地址、接口地址格式错误、缺 API Key 或测试模型名携带 `400`；后台渠道网络错误、上游 HTTP 错误、HTTP 200 JSON 业务错误、非法 JSON 响应、测试模型响应缺少内容和兜底上游错误携带 `502`；AI 普通退款和视频任务退款遇到用户缺失时携带 `404`。
- 额外修复：后台保存用户时，如果传入了不存在的用户 ID，旧逻辑会继续 `SaveUser` 并可能创建一个指定 ID 的新用户；现在返回 `404 用户不存在`，不再把编辑不存在用户变成创建用户。
- 额外修复：后台渠道弹窗在缺 API Key、且没有可回填保存渠道时，会以 `index=-1` 查 saved channel，旧逻辑可能触发 `saved[-1]` panic；现在 `findSavedChannel` 只允许非负下标回退。编辑已有渠道时，临时拉模型或测试模型也会把只填空白的 API Key 视为留空，并沿用已保存密钥。
- 前端配套：`requestImageQuestion` 和 `requestVideoGeneration` 现在能从非 2xx JSON 字符串里继续提取后端 `msg`，避免把“未登录或权限不足”“算力点不足”等错误退化成“请求失败：401”或“视频生成失败：402”。
- 验证：`go test -p 1 -vet=off ./handler ./router -run "TestAIProxyRejectsBadRequestsWithHTTPStatus|TestProtectedRoutesUseHTTPUnauthorized|TestFailErrorUsesTypedHTTPStatus|TestFailErrorUsesWrappedTypedHTTPStatus" -count=1` 通过；`go test -p 1 -vet=off ./service -run "TestLoginWrongPasswordCarriesUnauthorizedStatus|TestRegisterValidationCarriesHTTPStatus|TestLinuxDoValidationCarriesHTTPStatus|TestSaveUserValidationCarriesHTTPStatus|TestConsumeUserCreditsInsufficientCreditsCarriesPaymentRequiredStatus|TestRefundMissingUserCarriesNotFoundStatus|TestRefundVideoTaskMissingUserCarriesNotFoundStatus|TestAdminChannelValidationCarriesBadRequestStatus|TestAdminChannelInvalidBaseURLCarriesBadRequestStatus|TestAdminChannelBaseURLWhitespaceIsTrimmedBeforeRequest|TestAdminChannelWhitespaceAPIKeyFallsBackToSavedChannel|TestSaveSettingsTrimsModelChannelBaseURLAndAPIKey|TestAdminChannelUpstreamErrorsCarryBadGatewayStatus|TestAdminChannelFallbackErrorCarriesBadGatewayStatus|TestAdminChannelNetworkErrorsCarryBadGatewayStatus|TestAdminChannelHTTP200BusinessErrorsCarryBadGatewayStatus|TestAdminChannelHTTP200InvalidJSONCarriesBadGatewayStatus|TestAdminChannelTestModelMissingContentCarriesBadGatewayStatus|TestAdminChannelMissingModelCarriesBadRequestStatus" -count=1` 通过；`cd web && npm run test -- src/services/api/image.test.ts src/services/api/video.test.ts` 通过。
- 剩余风险：并非所有 service 业务错误都已经 typed；未携带 `StatusCode()` 的 safe error 仍走旧 HTTP 200。不要靠字符串把未知业务错误或 DB 错误硬猜成 400/404/500；后续继续按具体 service 路径逐个补 typed status。

### P1-4 路由注册过度集中（已开始治理）

- 原现象：公开接口、用户接口、AI 代理、后台管理接口都堆在一个 `router/router.go` 注册函数里。
- 处理结果：已把路由注册按权限和业务边界拆成 `registerPublicRoutes`、`registerAuthRoutes`、`registerAIRoutes`、`registerAdminRoutes`，没有引入路由生成器或新框架。
- 关键边界：`/api/admin/login` 仍在后台受保护分组外；`/api/v1` 仍整组挂 `UserAuth`；`/api/auth/me`、`/api/prompts`、`/api/assets` 仍使用 `OptionalAuth`；`NoRoute` 仍在全部路由注册之后。
- 验证：新增 `TestRegisteredRouteSurface` 固定完整 method/path 路由面；`go test -p 1 -vet=off ./router -count=1` 通过。
- 剩余风险：路由文件仍持有所有域的注册表；后续只有当单个域继续膨胀时，再把路由注册拆成独立文件，别提前造复杂模块系统。

### P1-5 pending-test 曾经堆成事实上的未发布清单

- 现象：`docs/pending-test.md` 原先有大量待用户确认项，`docs/todo.md` 基本为空壳，项目缺少当前版本待测状态结构。本轮已把待测项改成带 ID、状态、模块和重点验证的表格，并新增手工验收手册；第一批不依赖外部 AI 的条目已经完成本地验收并移出 pending。
- 证据：`docs/pending-test.md` 当前保留仍待 UI 交互、模型渠道、管理员配置或部署环境验证的条目；`docs/testing/pending-test-acceptance.md` 保留已从待测移出场景的回归步骤，并继续列出剩余待测项；`docs/features.md` 只记录当前作为正式说明保留的画布 zip、素材 zip、图片撤销恢复和未登录导航行为；`docs/todo.md` 仍只记录后续真实待办。
- 风险：如果后续不按手册执行并流转状态，待测表还会重新变成垃圾场；README 和 features 也会继续被未确认功能污染。
- 建议第一步：已完成。下一步按 `docs/testing/pending-test-acceptance.md` 继续执行依赖模型配置和后台配置的验收，不要把缺环境的项强行写成已确认。

### P1-6 前端 TypeScript 静态检查当前不通过（已修复）

- 原现象：仓库没有 `typecheck` 脚本，直接执行 `npx tsc --noEmit --pretty false` 会失败。
- 根因：多个联合类型没有先按 `kind` 或响应形态收窄就访问分支字段；部分 nullable 值直接传给要求非空数组的函数；`ModelPicker` 用 `.filter(Boolean)` 但 TypeScript 无法据此收窄成 `string[]`。
- 处理结果：已修复 `asset-transfer.ts`、`image/page.tsx`、`canvas-client-page.tsx`、`model-picker.tsx`、`services/api/video.ts` 中的类型问题，并在 `web/package.json` 增加 `typecheck` 脚本。
- 验证：`cd web && npm run typecheck` 已通过。

## P2

### P2-1 文档真相源不足

- 现象：原先 README 只链接功能、部署、手册、待办和数据库说明，没有“当前状态/发布准入/已验证范围”的入口；features 同时写能力和限制，pending-test 写待确认，todo 又空。本轮已新增冷启动和当前状态入口，但后续仍要维护，不能让它再漂移。
- 证据：`README.md` 是简短入口；`docs/README.md` 固定冷启动阅读顺序；`docs/status/current-status.md` 声明当前状态口径；`docs/features.md` 已把 AI 调用从旧的“前端直接请求”修正为“本地直连 + 后端云端渠道”两条边界；`docs/pending-test.md` 定义待测状态并按模块列出待确认变更。
- 风险：新贡献者会从 README 或 features 误判项目成熟度；维护者会在多个文档之间同步同一事实，最后必然漂移。
- 建议第一步：后续每个业务切片结束时同步 `docs/status/current-status.md`；README 只链接状态入口，不复制状态细节。

### P2-2 前端大文件开始膨胀

- 现象：前端最大页面/组件已经明显偏大，尤其画布客户端页集中了导入、生成、节点、素材、主题、对话等大量职责。
- 证据：本次只读统计最大文件：`web/src/app/(user)/canvas/[id]/canvas-client-page.tsx:1` 所在文件 2652 行；`web/src/app/(admin)/admin/settings/page.tsx:1` 所在文件 980 行；`web/src/app/(user)/image/page.tsx:1` 所在文件 790 行；`web/src/app/(user)/canvas/components/canvas-assistant-panel.tsx:1` 所在文件 661 行；`web/src/app/(user)/video/page.tsx:1` 所在文件 638 行。
- 风险：不是“行数大就犯罪”，而是画布核心页承担太多变化原因。后续修一个导入导出、AI 生成或节点交互，很容易踩到无关状态。
- 建议第一步：只拆真实复用或真实独立的逻辑：先从导入/导出、生成请求编排、节点选择/批量操作这类纯逻辑 hook 开始，不要为了好看拆空壳组件。

### P2-3 npm 与 Bun/Docker 依赖路径不一致（已开始治理）

- 现象：Web 包脚本是 npm/Next 常规脚本，Docker 构建阶段用 Bun 安装和构建，运行阶段又回到 Node 镜像里执行 `npm run start`。
- 处理结果：仓库当前只有 `web/bun.lock`，没有 `package-lock.json`，所以 Docker 主路径收敛为 Bun 安装和构建；运行阶段仍使用 Node 镜像，但改为通过 `/app/scripts/docker-start.mjs` 直接执行 `node node_modules/next/dist/bin/next start`，不再走 `npm run start`。`docs/deployment.md` 已写清源码开发 npm 脚本、Docker Bun 构建和 Docker Node 运行三者边界。
- 证据：`Dockerfile` 构建阶段使用 `oven/bun`、`bun install --frozen-lockfile` 和 `bun run build`；运行阶段使用 `node:22-bookworm-slim` 执行 `/app/scripts/docker-start.mjs`，脚本直接启动 Next.js；`docs/deployment.md` 的“前端依赖路径”记录这条边界。
- 剩余风险：本地开发仍常用 npm 脚本，而 Docker 构建使用 Bun lockfile；如果后续要做到完全一致，需要正式选择单一包管理器并补齐对应 lockfile。当前不凭空生成 `package-lock.json`。

### P2-4 部署文档过窄（已开始治理）

- 现象：README 写“部署：Docker”并给出 docker compose 快速开始，但部署文档实际几乎只讲 Render。
- 处理结果：`docs/deployment.md` 已从单一 Render 说明改成部署总览，列出 Render、Docker Compose、本地构建、源码开发启动、数据库迁移、持久化边界和未验证项。
- 风险：用户按 README 以为 Docker 部署路径完整，进部署文档却只看到 Render。真实生产需要的持久化、环境变量、静态资源、数据库选择边界没有集中说明。
- 剩余风险：本文档仍不等于生产部署验收；Docker 静态资源路径、长期运行和公网多人使用仍要按待测/待办处理，不能写成已验证能力。

### P2-4b Docker 运行时缺少子进程监督（已修复）

- 原现象：Docker `CMD` 通过 shell 先把 Go API 放到后台，再以前台方式运行 Next.js。Go API 后续退出时，容器主进程仍可能是 Next.js，容器看起来存活但 `/api/*` 已经不可用。
- 处理结果：新增 `/app/scripts/docker-start.mjs` 作为 Docker 运行入口；脚本先执行 `/app/server migrate`，再启动内部 Go API 和 Next.js，并监督两个子进程。任一子进程退出时会终止另一个进程并让容器退出，交给 compose / 平台重启策略处理。
- 验证：`node --test scripts/docker-start.test.mjs` 覆盖 API 退出会杀掉 Web、Web 退出会杀掉 API，以及 `SIGTERM` 会转发给两个子进程。
- 剩余风险：尚未实际执行 Docker build 和容器内 kill 子进程验收，仍需按 `PT-DOCKER-001` 做部署环境验证。

### P2-5 前端 API 代理和后端响应约定耦合紧

- 现象：Next API catch-all 代理所有方法到 Go `/api/*`，前端请求层又假设后端返回统一 JSON；但 AI 代理成功时会透传上游响应体和状态。
- 证据：`web/src/app/api/[...path]/route.ts:54` 到 `web/src/app/api/[...path]/route.ts:60` 导出全部 HTTP 方法；`web/src/services/api/request.ts:70` 到 `web/src/services/api/request.ts:77` 要求响应体是 `{ code, data, msg }`；AI 成功响应在 `handler/ai.go:129` 到 `handler/ai.go:138` 透传上游 header、状态和 body。
- 风险：普通业务接口和 AI 兼容接口混在同一 `/api` 代理下，调用方必须知道哪些接口是业务 JSON，哪些是上游透传。约定不清会制造前端误处理。
- 建议第一步：在 API 文档里分清“业务 JSON 接口”和“OpenAI 兼容透传接口”；前端服务层按接口族分开封装，不要让通用 `apiRequest` 覆盖所有路径。

### P2-6 图片节点本地恢复逻辑和视频节点不一致（已修复）

- 原现象：视频节点只要有 `storageKey` 就会尝试从本地媒体存储恢复 `content`，图片节点却要求 `metadata.content` 先非空，才会用 `storageKey` 恢复。storageKey-only 的图片节点会显示成“空图片节点”。
- 根因：`web/src/app/(user)/canvas/[id]/canvas-client-page.tsx` 的 `hydrateCanvasImages` 原先先判断 `node.type !== CanvasNodeType.Image || !content`，导致有 `storageKey` 但缺少 `content` 的图片节点根本不会走 `resolveImageUrl`。
- 处理结果：已把图片恢复顺序改成和视频一致：非图片直接返回；图片有 `storageKey` 时先 `resolveImageUrl(storageKey, content || "")`；没有 `storageKey` 且没有 `content` 时再返回；旧 `data:image/...` 迁移逻辑仍保留。单个图片或视频恢复失败时会保留该节点并标记 `本地媒体恢复失败`，不再让整个画布打开流程失败。
- 验证：用本地浏览器脚本先复现失败，再应用修复后重跑；`storageKey-only 图片节点` 从“空图片节点”变为正常图片，页面显示 `64 x 48 · 230 B`。新增 Vitest 覆盖 storageKey-only 视频恢复和单个媒体恢复失败隔离。

### P2-7 前端缺少测试基础（已开始治理）

- 原现象：前端没有测试脚本、测试配置和仓库内测试文件；只能靠浏览器脚本、`tsc` 或构建验证行为。
- 处理结果：已引入最小 Vitest 回归测试，把画布媒体补水逻辑从 `canvas-client-page.tsx` 抽到 `hydrate-canvas-images.ts`，覆盖图片 `storageKey`、图片旧 `content` fallback、视频 `storageKey`、storageKey-only 视频、内联 `data:image/...` 迁移、单个媒体恢复失败隔离、空图片节点和非媒体节点这些分支。
- 并发补充：画布运行态已从单个 `runningNodeId` 改成 `Set<string>` 节点集合；旧实现里多个异步生成/角度生成并发时，任一任务结束可能清掉其它任务运行态。现在新增 `addRunningNodeId`、`clearFinishedRunningNodeId` 和 `runWithRunningNodeId` 纯逻辑，完成或失败时只移除自己的节点 ID。
- 参考图补充：生成前参考图补水如果拿到空结果，会直接报“参考图片已丢失，无法继续生成”，并让节点进入错误状态；`dataUrlToFile` 也会拒绝空参考图数据，不再把空字符串生成空参考文件发给上游。
- 证据：`web/package.json` 已新增 `test` 脚本和 `vitest` devDependency；`web/src/app/(user)/canvas/[id]/hydrate-canvas-images.test.ts` 是当前第一组仓库内回归测试。
- 验证：`cd web && npm run test` 通过，`cd web && npm run typecheck` 通过。
- 剩余风险：这只是最小测试地基，不是完整测试体系。React DOM 交互、导入导出 zip、AI 生成链路和后台配置还没有稳定自动化覆盖。

### P2-8 后台手动算力调整流水曾经非事务（已修复）

- 原现象：AI 消费和退款流水已经事务化，但后台手动调整算力点仍先保存用户余额，再单独保存 `admin_adjust` 流水。
- 处理结果：已新增 `repository.AdjustUserCreditsWithLog`，把用户余额更新和 `admin_adjust` 流水写入合并到同一个数据库事务；流水写入失败时余额变更回滚。
- 验证：`TestAdjustUserCreditsRollsBackWhenCreditLogFails` 先用 SQLite trigger 复现“流水插入失败但余额变更成功”，再确认修复后余额保持原值。
- 剩余风险：credit log 仍缺完整 request/task trace id；后台页面的真实失败提示还需要用户环境验收。

### P2-9 Go 测试曾经假装按 DSN 隔离（已修复）

- 原现象：`service` 和 `handler` 包多个测试各自设置 `config.Cfg.DatabaseDSN`，但 `repository.DB()` 通过全局 `sync.Once` 只初始化一次；后续测试改 DSN 不会真的换数据库。
- 风险：测试看起来每个 case 都是独立内存库，实际共享同一个 GORM 连接，容易因为数据、trigger 或 settings 泄漏造成假绿或顺序依赖。
- 处理结果：新增 `setupServiceTestDB` 和 `setupHandlerTestDB`，统一设置测试库、确保表存在，并在每个相关测试前清空 `credit_logs`、`users`、`settings`、`prompts`、`assets`；`service` helper 还会清理本轮故障注入用的 SQLite trigger。
- 验证：`go test -p 1 -vet=off ./service -count=1` 和 `go test -p 1 -vet=off ./handler -count=1` 通过。

