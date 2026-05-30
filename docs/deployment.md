# 部署说明

本文档只说明当前仓库能看见的部署入口和边界。不要把未验证的生产能力写成已经稳定。

## 当前部署入口

- Render 一键部署：适合体验和演示。
- Docker Compose：使用已发布镜像运行。
- 本地镜像构建：用本仓库 `Dockerfile` 构建后运行。
- 源码开发启动：前后端分别启动，用于开发，不等于生产部署。

## Render 部署

点击下面按钮即可部署到 Render：

[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/basketikun/infinite-canvas)

步骤：

1. 点击 `Deploy to Render`。
2. 登录 Render，并按页面提示连接 GitHub。
3. 填写 `ADMIN_PASSWORD`，然后确认部署。

部署完成后，打开 Render 分配的 `.onrender.com` 域名访问。

免费版限制：

- 空闲约 15 分钟后会休眠，下次访问会自动唤醒。
- 免费版本地文件不是持久化存储，SQLite 数据可能在重启或重新部署后丢失。
- 适合体验和演示，不适合长期保存正式数据。

如果要长期使用，建议升级 Render 付费实例并挂载 Persistent Disk，或改用 PostgreSQL。

## Docker Compose

使用已发布镜像：

```bash
cp .env.example .env
# 修改 MYSQL_DSN，并在目标 MySQL 创建 infinite_canvas 数据库
docker compose up -d
```

本地构建镜像：

```bash
cp .env.example .env
# 修改 MYSQL_DSN，并在目标 MySQL 创建 infinite_canvas 数据库
docker compose -f docker-compose.local.yml up -d --build
```

当前 Docker Compose 只启动应用容器，不再随项目启动 MySQL；应用通过 `.env` 里的 `MYSQL_DSN` 连接宿主机已有 MySQL，Docker 场景推荐使用 `host.docker.internal:3307`，`DATABASE_DSN` 保持同值用于旧镜像兼容。镜像对外暴露 `3000`，由 Next.js 提供页面入口，并把 `/api/*` 代理到容器内部 Go API。运行入口是 `node /app/scripts/docker-start.mjs`：先迁移数据库，再同时启动 Go API 和 Next.js；任一子进程退出时，脚本会终止另一个进程并让容器退出，避免半活状态。

## 前端依赖路径

本仓库当前没有 `package-lock.json`，前端锁文件是 `web/bun.lock`。

- 源码开发：按 `web/package.json` 里的脚本执行，例如 `npm run dev`、`npm run typecheck`、`npm run test`。
- Docker 构建：使用 `bun install --frozen-lockfile` 和 `bun run build`，锁定在 `web/bun.lock`。
- Docker 运行：使用 Node 镜像运行 `/app/scripts/docker-start.mjs`，脚本内部直接启动已构建的 Next.js：`node node_modules/next/dist/bin/next start`；运行阶段不执行 `npm install`，也不再通过 `npm run start` 间接启动。

这不是说 npm 和 Bun 完全等价。出构建问题时，先按 Dockerfile 的 Bun 路径复现，再看本地 npm 脚本是否只是开发入口差异。

## 管理员账号

默认管理员用户名：

```text
admin
```

管理员密码来自环境变量 `ADMIN_PASSWORD`。正式部署必须修改默认值。

## 数据库迁移

当前服务默认 `DB_AUTO_MIGRATE=false`，正常启动不再偷偷修改表结构。

- Docker 镜像启动脚本会先执行 `/app/server migrate`，再启动并监督内部 Go API 和 Next.js。
- 如果直接运行后端源码，先执行 `go run . migrate`，再执行 `go run .`。
- 普通服务启动会先检查当前应用表和字段是否存在；未迁移或旧 schema 缺字段时会失败并提示执行显式迁移，即使禁用了默认管理员初始化也一样。
- 如果临时做本地开发并想保留旧的启动即迁移体验，可以设置 `DB_AUTO_MIGRATE=true`。

受控部署不要依赖 `DB_AUTO_MIGRATE=true`，迁移应该作为发布步骤显式执行。

## 数据持久化边界

- Docker Compose 默认使用 MySQL；`STORAGE_DRIVER` 为默认值或 `mysql` 时，`MYSQL_DSN` 会覆盖 `DATABASE_DSN`。
- `data` 目录挂载仍用于提示词等文件型数据，不再作为 Docker 默认数据库文件位置。
- MySQL 连接池通过 `MYSQL_MAX_OPEN_CONNS`、`MYSQL_MAX_IDLE_CONNS`、`MYSQL_CONN_MAX_LIFETIME` 配置。
- SQLite / PostgreSQL 仍可通过 `STORAGE_DRIVER` 和 `DATABASE_DSN` 显式配置；`render.yaml` 当前仍使用 SQLite 作为演示入口。
- 画布项目和“我的素材”主要保存在浏览器本地，不会因为后端数据库持久化就自动变成账号云同步。
- AI 本地直连配置保存在浏览器本地；后端渠道密钥保存在后端私有配置里。

## 已知未验证项

- 本文不声称生产部署、长期运行、静态资源路径和公网多人使用已经完整验证。
- Docker 静态资源路径仍按项目规则视为待确认点。
- 移动端触控、多人并发和历史数据兼容都不是当前稳定承诺。
