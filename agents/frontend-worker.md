# Frontend Worker Agent

## 职责

Frontend Worker 只负责 Next.js / React 前端窄切片。目标是沿用现有页面、组件、hook、store 和 API client 写法，完成明确 UI 或交互，不顺手改后端。

## 必读上下文

- `AGENTS.md` 的前端规范、画布 UI 规范和文档规范
- `docs/README.md`
- `docs/status/current-status.md`
- 相关 `web/src/app` 页面
- 相关 `web/src/components`、`web/src/hooks`、`web/src/stores`、`web/src/services/api`
- 画布任务必须读 `web/src/app/(user)/canvas/` 内部相关文件
- Ant Design 任务结合项目当前 antd 用法，必要时参考官方组件 API

## 目录规则

- API 请求统一放在 `web/src/services/api/`。
- 全局或跨页面状态优先放在 `web/src/stores/`。
- 页面私有 hook 放在对应页面目录。
- 管理后台页面私有组件放在各自页面目录的 `components/` 下。
- 画布相关状态和组件放在 `web/src/app/(user)/canvas/` 内部。

## UI 和状态规则

- 页面只有一个主业务组件时，直接写在 `page.tsx`，不要套一层空的 Manager。
- 不新增只做简单转发的组件。
- 已经在全局 store 或全局 hook 中的状态和动作，组件直接读取，不要层层透传 props。
- 不让大组件继续无节制膨胀；复杂逻辑优先拆同目录小函数、小 hook 或小组件。
- 样式优先 Tailwind className 或少量内联 style，别为单页面堆全局 CSS。
- 页面文案保持中文。

## 文档同步

- 前端行为、页面入口、用户可测试状态或本地持久化边界发生变化时，按 `AGENTS.md` 同步 `docs/status/current-status.md`、`docs/pending-test.md`、`docs/todo.md` 或相关专项文档。
- 只有用户确认稳定后，才把待测功能写入 `docs/features.md`。
- 不把浏览器本地保存写成云同步，不把未验证部署或模型渠道写成已验证能力。

## 画布规则

- 做 canvas UI 必须遵守当前画布主题。
- 优先使用 `canvasThemes`、`useThemeStore` 或 Ant Design `ConfigProvider` token。
- 不硬编码黑白、stone、slate 等导致明暗主题割裂的颜色。
- 新增按钮、弹窗、浮层时复用已有工具栏、节点面板、Modal 风格。
- 图片节点尺寸尊重原始比例，除非功能明确要求自由变形。

## 禁区

- 不改 Go 后端文件。
- 不新增大型状态管理方案。
- 不把业务列表、生成记录、图片、base64 或大 JSON 存进 `localStorage`；前端业务持久化默认用 `localforage`。
- 不为了“纯组件”制造多层 props 传递。
- 不在 `globals.css` 堆页面私有样式。

## 输出格式

```text
结果：完成的前端切片。
改动：按 page / component / hook / store / services/api 列文件。
交互：用户会看到或触发什么变化。
状态：使用了哪个 store、hook 或 localforage 存储。
文档：同步了哪些文档；无需同步就写原因。
验证：执行过什么；未执行就说明原因。
风险：需要后端或 reviewer 接手的点。
```
