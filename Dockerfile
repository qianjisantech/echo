FROM registry.cn-shanghai.aliyuncs.com/dev-sdk/golang1.23.8:latest

# 切换到root用户（关键修复）
USER root

# 替换为阿里云Debian镜像源（适用于bookworm）
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

# 2. 强制启用CGO
ENV CGO_ENABLED=1

# 3. 设置Go环境
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 4. 编译时添加netgo标签（可选但推荐）
RUN go build -tags netgo -o gosmo-agent ./cmd/gor.go

RUN ls -lh gosmo-agent || echo "构建失败"
EXPOSE 8888
CMD ["./gosmo-agent", \
    "--input-raw=:8888", \
    "--input-raw-track-response", \
    "--output-elasticsearch-host=http://47.94.96.190:9200", \
    "--output-elasticsearch-index=gosmo", \
    "--output-stdout"]