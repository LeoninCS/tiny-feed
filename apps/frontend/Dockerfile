# syntax=docker/dockerfile:1.7
#
# Multi-stage build for the Vue 3 + Vite frontend, served by Nginx.
#
# Build context 应该是 apps/frontend/ 目录本身（不是仓库根）。
# 单独 build：
#   cd apps/frontend
#   docker build -t tiny-feed-frontend .
# 通过 docker compose 时由 compose 设好 context。

FROM node:24-alpine AS build
WORKDIR /src

# 单独 COPY 依赖清单以便利用构建缓存。
COPY package.json package-lock.json ./
RUN npm ci

COPY . ./
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
