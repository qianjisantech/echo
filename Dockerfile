FROM registry.cn-shanghai.aliyuncs.com/dev-sdk/golang1.23.8:latest

# 1. 切换到root用户
USER root

# 2. 配置阿里云镜像源
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list && \
    sed -i 's|security.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

# 3. 设置Go环境
ENV CGO_ENABLED=1 \
    GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct

# 4. 构建应用
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 关键修改：将二进制安装到标准路径
RUN go build -tags netgo -o /usr/local/bin/gosmo-agent ./cmd/gor.go && \
    chmod +x /usr/local/bin/gosmo-agent

# 5. 验证构建
RUN ls -lh /usr/local/bin/gosmo-agent || (echo "构建失败" && exit 1)

# 6. 入口配置（必须使用绝对路径）
ENTRYPOINT ["/usr/local/bin/gosmo-agent"]
CMD ["--input-raw=:8080", "--output-stdout"]