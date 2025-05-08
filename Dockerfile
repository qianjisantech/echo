FROM registry.cn-shanghai.aliyuncs.com/dev-sdk/golang1.23.8:latest

# 设置环境变量
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

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