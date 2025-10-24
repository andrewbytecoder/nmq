# 构建阶段
FROM golang:1.25.1-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制整个源码（除了 .dockerignore 中排除的）
COPY . .

# podman build -t nmq:latest .

# 编译二进制文件（静态链接，避免依赖 libc）
# CGO_ENABLED=0 表示禁用 CGO，生成纯静态二进制
# GOOS=linux 明确指定目标操作系统
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o nmq \
    ./cmd/nmq/nmq.go

# 最终运行阶段（最小化镜像）
FROM alpine:latest

# 可选：设置非 root 用户（提升安全性）
RUN adduser -D -s /bin/sh nmquser

# 安装必要依赖（如时区数据）
RUN apk --no-cache add ca-certificates tzdata

RUN mkdir -p /home/nmquser

# 设置工作目录
WORKDIR /home/nmquser

# 复制编译好的二进制文件
COPY --chmod=757 --from=builder /app/nmq /home/nmquser/

# 更改文件所有者
RUN chown nmquser:nmquser nmq

# 切换到非 root 用户
USER nmquser

# 启动命令
CMD ["/home/nmquser/nmq"]