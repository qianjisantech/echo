package goreplay

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/buger/goreplay/internal/byteutils"
	"github.com/buger/goreplay/proto"
	"github.com/elastic/go-elasticsearch/v8"
)

// ElasticsearchOutput 用于将数据发送到 Elasticsearch
type ElasticsearchOutput struct {
	config   *ElasticsearchConfig
	client   *elasticsearch.Client
	bulkChan chan *Message
}

// ElasticsearchConfig 包含 Elasticsearch 连接配置
type ElasticsearchConfig struct {
	Hosts    string // ES 集群地址列表
	Index    string // 索引名称
	Username string // 认证用户名
	Password string // 认证密码
	UseSSL   bool   // 是否使用 SSL
	Sniff    bool   // 是否启用节点嗅探
	BulkSize int    // 批量操作大小
}

// NewElasticsearchOutput 创建 Elasticsearch 输出插件实例
func NewElasticsearchOutput(_ string, config *ElasticsearchConfig) PluginWriter {
	cfg := elasticsearch.Config{
		Addresses: []string{config.Hosts},
	}
	log.Printf("进入NewElasticsearchOutput")
	if config.Username != "" && config.Password != "" {
		cfg.Username = config.Username
		cfg.Password = config.Password
	}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}

	// 设置默认批量大小
	if config.BulkSize <= 0 {
		config.BulkSize = 1000
	}

	o := &ElasticsearchOutput{
		config:   config,
		client:   client,
		bulkChan: make(chan *Message, config.BulkSize),
	}

	// 启动批量处理协程
	go o.bulkProcessor()

	return o
}

// bulkProcessor 处理批量索引请求
func (o *ElasticsearchOutput) bulkProcessor() {
	var buffer bytes.Buffer
	var count int

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-o.bulkChan:
			if !ok {
				if count > 0 {
					o.flush(&buffer)
				}
				return
			}

			// 获取准备后的文档数据
			prepared := o.prepareDocument(msg)
			action := prepared["action"].(string)
			id := prepared["id"].(string)
			doc := prepared["doc"].(map[string]interface{})

			// 根据操作类型构建不同的请求
			if action == "index" {
				meta := map[string]interface{}{
					"index": map[string]interface{}{ // 使用index操作
						"_index": o.config.Index,
						"_id":    id,
					},
				}
				metaBytes, _ := json.Marshal(meta)
				buffer.Write(metaBytes)
				buffer.WriteByte('\n')

				docBytes, _ := json.Marshal(doc)
				buffer.Write(docBytes)
			}
			// 在构建更新请求时修改为：
			if action == "update" {
				meta := map[string]interface{}{
					"update": map[string]interface{}{
						"_index":            o.config.Index,
						"_id":               id,
						"retry_on_conflict": 3, // 添加冲突重试
					},
				}
				metaBytes, _ := json.Marshal(meta)
				buffer.Write(metaBytes)
				buffer.WriteByte('\n')

				// 构建完整的更新体
				updateBody := map[string]interface{}{
					"doc":    doc,
					"upsert": prepared["upsert"],
				}
				docBytes, _ := json.Marshal(updateBody)
				buffer.Write(docBytes)
			}

			buffer.WriteByte('\n')
			count++

			if count >= o.config.BulkSize {
				o.flush(&buffer)
				count = 0
			}

		case <-ticker.C:
			if count > 0 {
				o.flush(&buffer)
				count = 0
			}
		}
	}
}

// flush 发送批量请求到 Elasticsearch
func (o *ElasticsearchOutput) flush(buffer *bytes.Buffer) {
	if buffer.Len() == 0 {
		return
	}

	res, err := o.client.Bulk(
		bytes.NewReader(buffer.Bytes()),
		o.client.Bulk.WithContext(context.Background()),
	)

	if err != nil {
		log.Printf("Error sending bulk request to Elasticsearch: %s", err)
		return
	}
	defer res.Body.Close()
	log.Printf("Elasticsearch 返回结果 %s", res.String())
	if res.IsError() {
		log.Printf("Error response from Elasticsearch: %s", res.String())
	}

	buffer.Reset()
}

// prepareDocument 准备要索引的文档
func (o *ElasticsearchOutput) prepareDocument(msg *Message) map[string]interface{} {
	meta := payloadMeta(msg.Meta)
	data := msg.Data
	reqID := byteutils.SliceToString(meta[1]) // 获取请求ID
	timestamp := time.Now().Format(time.DateTime)

	// 判断是请求还是响应
	isRequest := isRequestOrResponse(msg.Meta)

	if isRequest {
		log.Printf("请求数据 %s", string(data))
		// 请求数据 - 创建新文档
		doc := map[string]interface{}{
			"@timestamp":  timestamp,
			"type":        "request",
			"req_id":      reqID,
			"req_ts":      byteutils.SliceToString(meta[2]),
			"req_url":     byteutils.SliceToString(proto.Path(data)),
			"req_method":  byteutils.SliceToString(proto.Method(data)),
			"req_headers": parseHeaders(data),
			"req_body":    byteutils.SliceToString(proto.Body(data)),
			// 预置响应字段为空
			"resp_data": nil,
		}

		// 返回创建操作
		return map[string]interface{}{
			"action": "index",
			"id":     reqID,
			"doc":    doc,
		}
	} else {
		log.Printf("响应数据 %s", string(data))
		// 响应数据 - 更新现有文档
		respData := map[string]interface{}{
			"status":  extractStatus(data),
			"headers": parseHeaders(data),
			"body":    byteutils.SliceToString(proto.Body(data)),
			"ts":      byteutils.SliceToString(meta[2]),
		}

		// 返回更新操作
		return map[string]interface{}{
			"action": "update",
			"id":     reqID,
			"doc": map[string]interface{}{
				"resp_data": respData,
			},
			"upsert": map[string]interface{}{
				"@timestamp": timestamp,
				"type":       "request",
				"req_id":     reqID,
				"resp_data":  respData,
			},
		}
	}
}

// extractStatus 从HTTP响应中提取状态码
func extractStatus(data []byte) string {
	// 示例数据格式：
	// HTTP/1.1 200 OK\r\nContent-Type: application/json...

	// 找到第一行
	end := bytes.IndexByte(data, '\n')
	if end == -1 {
		end = len(data)
	}
	firstLine := data[:end]

	// 分割第一行获取状态码
	parts := bytes.Split(firstLine, []byte(" "))
	if len(parts) >= 3 {
		return string(parts[1]) // 返回状态码部分
	}
	return "000" // 默认值
}

// requestPayload 判断是请求还是响应
func isRequestOrResponse(meta []byte) bool {
	log.Printf("meta数据 %s", string(meta))
	parts := bytes.Split(meta, []byte(" "))
	if len(parts) > 0 {
		// 第一部分为1表示请求，2表示响应
		return parts[0][0] == '1'
	}
	return true // 默认为请求
}

// PluginWrite 实现 PluginWriter 接口
func (o *ElasticsearchOutput) PluginWrite(msg *Message) (n int, err error) {
	// 将消息发送到批量处理通道
	o.bulkChan <- msg
	return len(msg.Data), nil
}

// Close 关闭 Elasticsearch 客户端
func (o *ElasticsearchOutput) Close() error {
	close(o.bulkChan)
	return nil
}

// parseHeaders 解析 HTTP 头
func parseHeaders(data []byte) map[string]string {
	mimeHeader := proto.ParseHeaders(data)
	header := make(map[string]string)
	for k, v := range mimeHeader {
		header[k] = strings.Join(v, ", ")
	}
	return header
}
func (i *ElasticsearchOutput) String() string {
	return "Elasticsearch Output"
}
