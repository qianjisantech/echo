
FROM registry.cn-shanghai.aliyuncs.com/vue-gin-devops/golang1.21.1:latest

# 设置环境变量
ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    CGO_ENABLED=0 \
    GOOS=linux

# 设置工作目录
WORKDIR /app

# 先复制依赖文件以利用缓存层
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制项目所有文件
COPY .. .

# 安装CA证书
RUN apt-get update && apt-get install -y ca-certificates

# 4. 关键修正：构建主程序（cmd/gor.go）
RUN go build -o gosmo-agent ./cmd/gor.go  # 注意路径变化

# 5. 验证构建结果
RUN ls -lh gosmo-agent || echo "构建失败"
# 暴露应用程序的端口
EXPOSE 8888

# 运行应用程序
CMD ["./gosmo-agent", \
    "--input-raw=:8888", \
    "--input-raw-track-response", \
    "--output-elasticsearch-host=http://47.94.96.190:9200", \
    "--output-elasticsearch-index=gosmo", \
    "--output-stdout"]