# 使用 Go 1.23.8 官方镜像
FROM golang:1.23.8-bookworm

# 设置环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=1   \
    GOOS=linux

# 安装编译依赖
RUN apt-get update && \
    apt-get install -y \
    libpcap-dev  \
    gcc         \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*
# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制全部源代码（修正COPY语法）
COPY . .

# 构建应用程序
RUN go build -v -o gosmo-agent ./cmd/gor.go

# 验证构建结果
RUN ldd gosmo-agent 2>/dev/null || echo "静态编译验证"

# 暴露端口
EXPOSE 8888

# 运行程序
CMD ["./gosmo-agent", \
    "--input-raw=:8888", \
    "--input-raw-track-response", \
    "--output-elasticsearch-host=http://47.94.96.190:9200", \
    "--output-elasticsearch-index=gosmo", \
    "--output-stdout"]