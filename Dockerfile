# 构建 Next.js 前端产物。
FROM oven/bun:1 AS web-build

WORKDIR /app/web
COPY web/package.json web/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile --registry=https://registry.npmmirror.com --cache-dir=/root/.bun/install/cache
COPY VERSION /app/VERSION
COPY web ./
RUN bun run build

# 构建 Go 后端入口。
FROM golang:1.25-alpine AS api-build

WORKDIR /app
COPY go.mod go.sum ./
COPY config ./config
COPY handler ./handler
COPY middleware ./middleware
COPY model ./model
COPY repository ./repository
COPY router ./router
COPY service ./service
COPY main.go ./
RUN go build -o /server .

# 运行镜像：Go 对外监听 3000，Next.js 只在容器内部监听 3001。
FROM oven/bun:1

WORKDIR /app
COPY VERSION /app/VERSION
COPY --from=api-build /server /app/server
COPY --from=web-build /app/web /app/web
ENV PROMPT_DATA_DIR=/app/data/prompts
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /app/data/prompts

EXPOSE 3000
# 先启动内部 Next.js，再由 Go 统一处理 /api/* 和页面反代。
CMD ["sh", "-c", "cd /app/web && HOSTNAME=0.0.0.0 PORT=3001 bun run start & PORT=3000 /app/server"]
