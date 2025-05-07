package goreplay

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// 在包级别添加插件注册表
var (
	inputPlugins  = make(map[string]func(options map[string]interface{}) PluginReader)
	outputPlugins = make(map[string]func(options map[string]interface{}) PluginWriter)
)

// RegisterInputPlugin 注册输入插件
func RegisterInputPlugin(name string, factory func(options map[string]interface{}) PluginReader) {
	inputPlugins[name] = factory
}

// RegisterOutputPlugin 注册输出插件
func RegisterOutputPlugin(name string, factory func(options map[string]interface{}) PluginWriter) {
	outputPlugins[name] = factory
}

// Engine 主接口
type Engine interface {
	// Start 启动流量重放
	Start(ctx context.Context) error
	// Stop 停止流量重放
	Stop() error
	// Status 获取运行状态
	Status() Status
	// UpdateConfig 修改运行时配置
	UpdateConfig(config Config) error
}
type EngineBuilder struct {
	config Config
}

func NewBuilder() *EngineBuilder {
	return &EngineBuilder{
		config: Config{},
	}
}

func (e *EngineBuilder) WithHTTPInput(options map[string]interface{}) *EngineBuilder {
	e.config.Inputs = append(e.config.Inputs, InputConfig{
		Type:    "http",
		Options: options,
	})
	return e
}

func (e *EngineBuilder) WithHTTPOutput(options map[string]interface{}) *EngineBuilder {
	e.config.Outputs = append(e.config.Outputs, OutputConfig{
		Type:    "http",
		Options: options,
	})
	return e
}

func (e *EngineBuilder) WithMiddleware(middleware string) *EngineBuilder {
	e.config.Middleware = middleware
	return e
}

func (e *EngineBuilder) Build() (Engine, error) {
	return NewEngine(e.config)
}

// Status 运行状态
type Status struct {
	Running   bool
	Processed uint64
	Dropped   uint64
	Errors    uint64
	LastError error
	StartedAt time.Time
}

// Config 配置结构
type Config struct {
	Inputs     []InputConfig
	Outputs    []OutputConfig
	Middleware string
	ExitAfter  time.Duration
	// 其他配置项...
}

// InputConfig 输入配置
type InputConfig struct {
	Type    string
	Options map[string]interface{}
}

// OutputConfig 输出配置
type OutputConfig struct {
	Type    string
	Options map[string]interface{}
}

// NewEngine 创建新的重放实例
func NewEngine(config Config) (Engine, error) {
	return &EngineImpl{
		config: config,
	}, nil
}

type EngineImpl struct {
	config   Config
	emitter  *Emitter
	plugins  *InOutPlugins
	running  bool
	stopChan chan struct{}
}

func (e *EngineImpl) Start(ctx context.Context) error {
	if e.running {
		return errors.New("already running")
	}

	// 初始化插件
	plugins, err := e.initPlugins()
	if err != nil {
		return err
	}
	e.plugins = plugins

	// 初始化emitter
	e.emitter = NewEmitter()
	e.stopChan = make(chan struct{})
	e.running = true

	// 启动emitter
	go e.emitter.Start(plugins, e.config.Middleware)

	// 如果设置了超时
	if e.config.ExitAfter > 0 {
		go func() {
			time.Sleep(e.config.ExitAfter)
			e.Stop()
		}()
	}
	return nil
}

func (e *EngineImpl) Stop() error {
	if !e.running {
		return nil
	}

	close(e.stopChan)
	e.emitter.Close()
	e.running = false
	return nil
}

func (e *EngineImpl) Status() Status {
	if e.emitter == nil {
		return Status{
			Running:   e.running,
			StartedAt: time.Time{},
		}
	}

	return Status{
		Running:   e.running,
		Processed: e.emitter.Processed(),
		//Dropped:   e.emitter.Dropped(),
		//Errors:    e.emitter.Errors(),
		//LastError: e.emitter.LastError(),
		//StartedAt: e.emitter.StartedAt(),
	}
}
func (e *EngineImpl) UpdateConfig(config Config) error {
	if e.running {
		return errors.New("cannot update config while running")
	}
	e.config = config
	return nil
}

func (e *EngineImpl) initPlugins() (*InOutPlugins, error) {
	plugins := new(InOutPlugins)

	// 初始化输入插件
	for _, input := range e.config.Inputs {
		switch input.Type {
		case "http":
			plugins.registerPlugin(NewHTTPInput, input.Options)
		case "tcp":
			plugins.registerPlugin(NewTCPInput, input.Options)
		// 其他输入插件...
		default:
			return nil, fmt.Errorf("unknown input type: %s", input.Type)
		}
	}

	// 初始化输出插件
	for _, output := range e.config.Outputs {
		switch output.Type {
		case "http":
			plugins.registerPlugin(NewHTTPOutput, output.Options)
		case "tcp":
			plugins.registerPlugin(NewTCPOutput, output.Options)
		// 其他输出插件...
		default:
			return nil, fmt.Errorf("unknown output type: %s", output.Type)
		}
	}

	if len(plugins.Inputs) == 0 {
		return nil, errors.New("no inputs configured")
	}

	if len(plugins.Outputs) == 0 {
		return nil, errors.New("no outputs configured")
	}

	return plugins, nil
}
