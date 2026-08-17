# ---- 构建阶段 ----
# 注意：go.mod 声明 go 1.26.5，请使用 golang:1.26 及以上基础镜像。
# 若当前镜像仓库无 1.26 标签，可改为更大的版本号（如 golang:1.27-alpine），
# Go 会自动按需下载对应工具链。
FROM golang:1.26-alpine AS builder

WORKDIR /src

# 国内服务器拉取依赖加速（中国大陆环境必备）
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off
# modernc.org/sqlite 为纯 Go 实现，关闭 CGO 以产出可移植的静态二进制
ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local

# 先拉取依赖，利用 Docker 层缓存加速重复构建
COPY go.mod go.sum* ./
RUN go mod download

# 再拷贝源码并编译
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/private-chat ./cmd/server

# ---- 运行阶段 ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /out/private-chat /app/private-chat
COPY configs/config.yaml /app/config.yaml

EXPOSE 8080
# /data 通过卷挂载持久化（数据库 + 上传文件）
VOLUME ["/data"]

ENTRYPOINT ["/app/private-chat", "-config", "/app/config.yaml"]
