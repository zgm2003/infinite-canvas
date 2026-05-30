# TODO

本文档用来记录当前项目后续比较值得处理的事项。

- [ ] 扩展前端回归测试覆盖：当前已有画布媒体补水、运行态、图片/视频 API client 和本地存储 fallback 最小 Vitest；后续再补导入导出 zip、素材导入导出、关键 React 交互和更多 AI 请求封装测试。
- [ ] 接入生成任务取消链路：当前视频 API client 已支持 `AbortSignal`，但画布删除节点、清空画布、页面卸载和用户主动取消还没有统一的 `AbortController` 管理；后续应让图片、视频和文本生成任务都能按节点取消。
- [ ] 引入版本化数据库迁移：当前显式迁移仍使用 GORM `AutoMigrate`，后续生产化应增加 migration 版本记录和回滚策略。
- [ ] 继续硬化 AI 算力点结算边界：补齐 credit log 的 request/task trace id。
