FROM registry.cn-shanghai.aliyuncs.com/dev-sdk/golang1.23.8:latest

# 设置环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=1 \
    GOOS=linux

# 修复权限并安装依赖
RUN mkdir -p /var/lib/apt/lists/partial && \
    chmod -R 0755 /var/lib/apt/lists && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    libpcap-dev \
    gcc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY .. .
RUN go build -o gosmo-agent ./cmd/gor.go
RUN ls -lh gosmo-agent || echo "构建失败"
EXPOSE 8888
CMD ["./gosmo-agent", \
    "--input-raw=:8888", \
    "--input-raw-track-response", \
    "--output-elasticsearch-host=http://47.94.96.190:9200", \
    "--output-elasticsearch-index=gosmo", \
    "--output-stdout"]