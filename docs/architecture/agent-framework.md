# Agent 分工框架

本文档定义本项目的 AI / 自动化 agent 分工方式。它只细化 `AGENTS.md` 已有规则，不替代、不放宽项目规则。

## 定位

本项目是单仓应用：后端是 Go + Gin + GORM，前端是 Next.js App Router + React + TypeScript。agent 框架用于把一次需求拆成可审查的窄切片，避免一个 agent 同时做架构、后端、前端、审查和文档，最后没人知道改动边界在哪里。

原则很简单：

- 一个 agent 只负责一个清楚角色。
- 每次只改和任务直接相关的文件。
- 先读现有代码和文档，再落改动。
- 输出要能让下一个角色接手，不靠口头猜。
- 不照搬其它项目的目录、流程或治理口径。

## 四类角色

### Architect

负责架构边界、问题拆分和文档治理。Architect 可以读全局上下文、决定切片顺序、指出哪些文件归谁处理，但不直接改业务实现。

典型产物：

- 任务拆分。
- 影响范围。
- 需要更新的文档归属。
- 后端 / 前端 / reviewer 的交接说明。

### Backend Worker

负责 Go 后端窄切片。遵守 `handler/`、`service/`、`repository/`、`model/` 分层，不让 `handler` 直连数据库，不把业务校验塞进 repository。

典型产物：

- 路由、handler、service、repository、model 的最小改动。
- 新增或变更表时同步 `docs/backend-database.md`。
- 接口仍保持 `{ code, data, msg }`。

### Frontend Worker

负责 Next.js / React 前端窄切片。遵守 App Router、`web/src/services/api/`、`web/src/stores/`、画布内聚、Ant Design 和 Tailwind 规则。

典型产物：

- 页面、组件、hook、store、API client 的最小改动。
- 画布相关内容留在 `web/src/app/(user)/canvas/` 内部。
- 避免无意义 props 传递和继续膨胀的大组件。

### Reviewer

负责审查越界、文档真实性、坏味道和验证证据。Reviewer 不做大范围重构，不用“顺手优化”扩大任务。

典型产物：

- 越界文件清单。
- 与 `AGENTS.md` 冲突的点。
- 文档是否说了真实已实现行为。
- 必要验证和缺失证据。

## 通用工作流

1. 读 `AGENTS.md`、`docs/README.md`、`docs/status/current-status.md`、需求和直接相关代码。
2. 判断这是架构、后端、前端、审查还是混合任务。
3. 混合任务先由 Architect 拆切片，不允许一个 agent 全包。
4. Worker 只处理自己的窄切片，不能顺手改其它层。
5. 需要文档时按 `AGENTS.md` 文档归属更新：功能说明进 `docs/features.md`，待办进 `docs/todo.md`，待测试进 `docs/pending-test.md`，数据库结构进 `docs/backend-database.md`，接口响应规则进 `docs/api-response.md`。
6. Worker 可以更新自己切片直接造成的事实文档；禁止的是顺手重写 README、功能介绍或不相关文档，不是禁止同步必要文档。
7. Reviewer 最后检查边界、事实和证据。

## 禁止全能 agent

禁止出现“我读完整仓，然后后端、前端、文档、审查一起改完”的全能 agent。它通常会制造三类垃圾：

- 任务边界失控，改到无关文件。
- 文档和实现互相污染，没人知道哪个是真的。
- 前后端各做一半，reviewer 只能猜意图。

如果一个需求同时涉及后端、前端和文档，必须显式拆成多个窄切片，并写清楚交接点。

## 文档归属

- `docs/architecture/`：架构边界、分工规则、跨模块设计。
- `agents/`：角色职责、输入输出、禁区和交接格式。
- `docs/README.md`：唯一冷启动入口和文档索引。
- `docs/status/current-status.md`：当前事实、运行边界和验证缺口。
- `docs/audit/initial-review.md`：结构性问题台账。
- `docs/features.md`：用户确认稳定、可以写成正式说明的功能；仍待测的实现留在 `docs/pending-test.md`。
- `docs/todo.md`：后续值得处理但未完成的事项。
- `docs/pending-test.md`：已经实现、还需要用户测试确认的事项。
- `docs/backend-database.md`：后端真实使用的数据表。
- `docs/api-response.md`：接口响应约定。

不要把计划写成已完成，不要把本地浏览器存储写成云同步，不要把未验证的 Docker 静态资源路径写成生产已验证。

## 输出格式

每个 agent 结束时按下面格式回复，少废话：

```text
结果：完成了什么 / 没完成什么。
改动：列出文件路径和关键点。
证据：读过或验证过的关键文件、命令、现象。
风险：仍可能有问题的地方，没有就写“无”。
交接：下一个角色需要接什么，没有就写“无”。
```

如果没有执行构建或测试，直说“按项目规则未执行构建/测试”，不要伪造验证。
