# syntax=docker/dockerfile:1.7
#
# Multi-stage build for the backend API (Gin + GORM).
#
# Build context 应该是 apps/backend/ 目录本身（不是仓库根）。
# 单独 build：
#   cd apps/backend
#   docker build -t tiny-feed-backend .
# 通过 docker compose 时由 compose 设好 context。
#
# Worker stage 之前存在过，2026-08 因拆掉 redis/mq 一起删了，
# 这里不再保留对应 stage。

ARG GO_VERSION=1.24.5
ARG GOPROXY=https://goproxy.cn,direct

FROM golang:${GO_VERSION} AS build
ARG GOPROXY
ENV GOPROXY=${GOPROXY} \
    CGO_ENABLED=0
WORKDIR /src

# 单独 COPY 依赖清单以便利用构建缓存。
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

# 复制源码并编译为静态二进制。
COPY . ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd

# 运行时镜像：alpine 极简底包 + 非 root 账号。
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -s /sbin/nologin app
WORKDIR /app
COPY --from=build /src/configs ./configs
COPY --from=build /out/api /app/api
RUN mkdir -p ./.run/uploads && chown -R app:app /app
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
