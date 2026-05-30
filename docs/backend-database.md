# 后端数据库说明

本文档只记录后端当前已经使用的主要数据表。

## 数据库

后端使用 GORM 管理数据库连接和表结构迁移。

支持的存储驱动：

- `sqlite`
- `mysql`
- `postgres`
- `postgresql`

数据库迁移由显式命令执行，当前迁移维护以下表：

- `users`
- `credit_logs`
- `prompts`
- `assets`
- `settings`

默认 `DB_AUTO_MIGRATE=false`，服务启动只打开数据库连接，不主动改表结构。发布或初始化数据库时执行迁移命令：

```bash
go run . migrate
```

Docker 镜像内等价命令是：

```bash
/app/server migrate
```

开发时如果想省掉显式迁移步骤，可以临时设置 `DB_AUTO_MIGRATE=true`，让服务启动时执行 AutoMigrate。受控部署不要依赖这个开关；否则应用启动和 schema 变更会重新绑死。

关闭启动迁移时必须先保证目标数据库已经完成迁移；普通服务启动会先检查当前应用表和字段是否存在，缺表或旧 schema 缺字段时直接失败并提示执行显式迁移。这个检查不依赖默认管理员初始化，即使禁用了默认管理员，也不会让服务在空库或明显过期的 schema 上继续启动。

后续新增表时再同步补充本文档，未实际使用的规划表不提前写入。

### users

系统用户表。用户基础信息、角色、算力点余额和第三方登录标识放在该表中。

| 字段              | 类型     | 说明                       |
|-----------------|--------|--------------------------|
| `id`            | string | 主键                       |
| `username`      | string | 用户名，唯一索引                 |
| `password`      | string | 密码哈希                     |
| `email`         | string | 邮箱                       |
| `display_name`  | string | 昵称                       |
| `avatar_url`    | string | 头像地址                     |
| `role`          | string | 角色：`user`、`admin`        |
| `credits`       | number | 算力点余额                    |
| `aff_code`      | string | 用户自己的邀请码，唯一索引            |
| `aff_count`     | number | 已邀请用户数量，冗余统计字段           |
| `inviter_id`    | string | 邀请人用户 ID                 |
| `github_id`     | string | GitHub 用户 ID               |
| `linux_do_id`   | string | Linux.do 用户 ID            |
| `wechat_id`     | string | 微信用户 ID                   |
| `status`        | string | 用户状态：`active`、`ban`       |
| `last_login_at` | string | 最近登录时间                   |
| `extra`         | json   | 扩展信息，第三方资料按平台命名空间保存，如 `linuxDo` |
| `created_at`    | string | 创建时间                     |
| `updated_at`    | string | 更新时间                     |

### prompts

提示词表。用于保存公开提示词、内置 GitHub 系统提示词、分类和预览内容。

| 字段           | 类型     | 说明                           |
|--------------|--------|------------------------------|
| `id`         | string | 主键                           |
| `title`      | string | 标题                           |
| `cover_url`  | string | 封面图                          |
| `prompt`     | string | 提示词内容                        |
| `tags`       | json   | 标签列表                         |
| `category`   | string | 分类标识                         |
| `preview`    | text   | Markdown 展示内容，可包含文本、图片、视频链接等 |
| `created_at` | string | 创建时间                         |
| `updated_at` | string | 更新时间                         |

`github_url` 仅用于接口返回，不写入数据库。

### assets

素材表。当前用于后台素材库。

| 字段               | 类型     | 说明                            |
|------------------|--------|-------------------------------|
| `id`             | string | 主键                            |
| `title`          | string | 标题                            |
| `type`           | string | 当前后台素材库支持的素材类型：`text`、`image` |
| `cover_url`      | string | 封面图                           |
| `tags`           | json   | 标签列表                          |
| `category`       | string | 分类标识                          |
| `description`    | string | 描述                            |
| `content`        | text   | 文本或 Markdown 内容               |
| `url`            | string | 图片等媒体地址                       |
| `created_at`     | string | 创建时间                          |
| `updated_at`     | string | 更新时间                          |

### settings

系统配置表，只保存两行数据：`public` 放前端可读取的公开配置，`private` 放仅后端和管理员可读取的私有配置，配置值都用 JSON。

| 字段           | 类型     | 说明                    |
|--------------|--------|-----------------------|
| `key`        | string | 主键：`public`、`private` |
| `value`      | json   | 配置内容                  |
| `created_at` | string | 创建时间                  |
| `updated_at` | string | 更新时间                  |

`public.value` 常放前端展示和可公开读取的配置，例如模型列表、登录开关等。
`private.value` 常放渠道密钥、登录密钥、后台内部开关等。

当前系统设置接口会按后端结构体序列化和反序列化已知字段；数据库 JSON 中额外存在的旧字段会被忽略。

`public.value` 当前字段：

| 字段                | 类型       | 说明             |
|-------------------|----------|----------------|
| `modelChannel` | object | 模型渠道公开配置组 |
| `auth` | object | 公开登录配置 |

`modelChannel` 当前字段：

| 字段                | 类型       | 说明             |
|-------------------|----------|----------------|
| `availableModels` | string[] | 系统可用模型列表       |
| `modelCosts` | object[] | 模型算力点配置       |
| `defaultModel`    | string   | 默认模型           |
| `defaultImageModel` | string | 默认图片模型         |
| `defaultVideoModel` | string | 默认视频模型         |
| `defaultTextModel` | string  | 默认文本模型         |
| `systemPrompt`    | string   | 系统提示词          |
| `allowCustomChannel` | bool    | 是否允许用户自定义渠道，默认允许，关闭后前端只提供走后端渠道的模式 |

`modelCosts` 每项字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 模型名称 |
| `credits` | number | 每次后端模型接口调用前预扣的算力点，未配置默认不扣除 |

`auth.linuxDo` 当前字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `enabled` | bool | 是否开启 Linux.do 登录 |

`private.value` 当前字段：

| 字段         | 类型       | 说明       |
|------------|----------|----------|
| `channels` | object[] | 模型渠道配置列表 |
| `promptSync` | object | GitHub 远程提示词定时同步配置 |
| `auth` | object | 私有登录配置 |

`channels` 每项字段：

| 字段       | 类型       | 说明       |
|----------|----------|----------|
| `protocol` | string | 协议，当前支持 `openai` |
| `name`   | string   | 渠道名称     |
| `baseUrl` | string  | 渠道接口地址   |
| `apiKey` | string   | 渠道密钥     |
| `models` | string[] | 渠道可用模型列表 |
| `weight` | number   | 渠道权重，同一模型命中多个渠道时按权重随机 |
| `enabled` | bool    | 是否启用     |
| `remark` | string   | 备注       |

`promptSync` 字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `enabled` | bool | 是否开启定时同步，默认开启 |
| `cron` | string | Cron 表达式，默认每 5 分钟 |

`auth.linuxDo` 当前字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `clientId` | string | Linux.do OAuth App Client ID |
| `clientSecret` | string | Linux.do OAuth App Client Secret，后台返回时隐藏 |

后端请求模型时，先按模型名筛选启用且包含该模型的渠道，再按 `weight` 加权随机选择一个渠道。

### credit_logs

用户算力点变更流水表。当前记录后台手动调整、模型调用预扣和模型调用失败返还。

| 字段           | 类型     | 说明                       |
|--------------|--------|--------------------------|
| `id`         | string | 主键                       |
| `user_id`    | string | 关联用户 ID                  |
| `type`       | string | 类型：`admin_adjust`、`ai_consume`、`ai_refund` |
| `amount`     | number | 本次变动数量，增加为正，扣减为负         |
| `balance`    | number | 变动后的用户算力点余额              |
| `related_id` | string | 关联业务 ID，可为空；视频任务消费和退款会使用上游任务 ID |
| `remark`     | string | 备注                       |
| `extra`      | json   | 扩展信息，当前记录 AI 模型、接口路径和视频任务渠道绑定信息 |
| `created_at` | string | 创建时间                     |

`type` 当前取值：

| 值 | 说明 |
| --- | --- |
| `admin_adjust` | 后台手动调整 |
| `ai_consume` | 调用后端模型接口消费 |
| `ai_refund` | 后端模型接口调用失败返还 |

`extra` 当前字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 本次 AI 消费或退款对应的模型名称 |
| `path` | string | 本次 AI 消费或退款对应的代理路径，例如 `/images/generations`、`/videos` |
| `taskId` | string | 视频任务 ID，仅视频任务相关流水使用 |
| `channelName` | string | 创建视频任务时使用的渠道名称，用于后续轮询和下载精确匹配渠道 |
| `channelBaseUrl` | string | 创建视频任务时使用的渠道 Base URL，用于后续轮询和下载定位渠道 |
