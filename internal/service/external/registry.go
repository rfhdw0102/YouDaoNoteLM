// internal/service/external/registry.go
package external

import (
	"fmt"
	"sync"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// FactoryFunc provider 工厂函数签名（无类型约束版本，供 Registry 内部存储）
// 根据 ServiceConfig 创建具体的服务实例
type FactoryFunc func(cfg *ServiceConfig) (interface{}, error)

// TypedFactoryFunc 带类型约束的工厂函数签名
type TypedFactoryFunc[T any] func(cfg *ServiceConfig) (T, error)

// ProviderInfo provider 元信息（用于 API 发现）
type ProviderInfo struct {
	ServiceType  string            `json:"service_type"`
	Provider     string            `json:"provider"`
	DisplayName  string            `json:"display_name"`
	RequiredKeys []string          `json:"required_keys"`
	OptionalKeys []string          `json:"optional_keys"`
	Implemented  bool              `json:"implemented"`          // 是否已实现
	KeyLabels    map[string]string `json:"key_labels,omitempty"` // 参数中文标签
}

// providerEntry 注册表中的一个 provider
type providerEntry struct {
	info    ProviderInfo
	factory FactoryFunc
}

// Registry provider 注册表
// 线程安全，支持运行时注册和查询
type Registry struct {
	mu      sync.RWMutex
	entries map[string]map[string]providerEntry // [serviceType][providerName] → entry
}

// NewRegistry 创建空的 Registry
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]map[string]providerEntry),
	}
}

// 全局 Registry 实例
var globalRegistry = NewRegistry()

// GetGlobalRegistry 获取全局 Registry
func GetGlobalRegistry() *Registry {
	return globalRegistry
}

// Register 注册一个 provider（无类型约束，供 init() 中直接调用）
func (r *Registry) Register(serviceType, providerName, displayName string,
	requiredKeys, optionalKeys []string, factory FactoryFunc, keyLabels map[string]string) {

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.entries[serviceType] == nil {
		r.entries[serviceType] = make(map[string]providerEntry)
	}

	// 标记是否已实现（factory 不为 nil）
	implemented := factory != nil

	r.entries[serviceType][providerName] = providerEntry{
		info: ProviderInfo{
			ServiceType:  serviceType,
			Provider:     providerName,
			DisplayName:  displayName,
			RequiredKeys: requiredKeys,
			OptionalKeys: optionalKeys,
			Implemented:  implemented,
			KeyLabels:    keyLabels,
		},
		factory: factory,
	}

	logger.Info("Provider 已注册",
		zap.String("service_type", serviceType),
		zap.String("provider", providerName),
		zap.Bool("implemented", implemented),
	)

}

// RegisterTyped 注册一个带类型约束的 provider
// 编译期保证 factory 返回类型 T，调用侧无需类型断言
//
// 使用示例：
//
//	external.RegisterTyped[search.SearchEngine](r, "search", "tavily", "Tavily",
//	    []string{"api_key"}, nil,
//	    func(cfg *external.ServiceConfig) (search.SearchEngine, error) {
//	        return NewTavilyEngine(cfg.APIKey), nil
//	    }, nil)
func RegisterTyped[T any](r *Registry, serviceType, providerName, displayName string,
	requiredKeys, optionalKeys []string, factory TypedFactoryFunc[T], keyLabels map[string]string) {

	// 将类型化工厂包装为无类型工厂
	wrappedFactory := func(cfg *ServiceConfig) (interface{}, error) {
		return factory(cfg)
	}

	r.Register(serviceType, providerName, displayName, requiredKeys, optionalKeys, wrappedFactory, keyLabels)
}

// Create 根据 serviceType、providerName 和配置创建服务实例（无类型约束版本）
func (r *Registry) Create(serviceType, providerName string, cfg *ServiceConfig) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers, ok := r.entries[serviceType]
	if !ok {
		return nil, fmt.Errorf("不支持的服务类型: %s", serviceType)
	}

	entry, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("不支持的 %s provider: %s", serviceType, providerName)
	}

	instance, err := entry.factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 %s/%s 失败: %w", serviceType, providerName, err)
	}

	return instance, nil
}

// CreateTyped 根据 serviceType、providerName 和配置创建服务实例（带类型约束版本）
// 编译期保证返回类型 T，无需运行时类型断言
//
// 使用示例：
//
//	engine, err := external.CreateTyped[search.SearchEngine](registry, "search", "tavily", cfg)
//	// engine 的类型已经是 search.SearchEngine，无需断言
func CreateTyped[T any](r *Registry, serviceType, providerName string, cfg *ServiceConfig) (T, error) {
	var zero T

	instance, err := r.Create(serviceType, providerName, cfg)
	if err != nil {
		return zero, err
	}

	typed, ok := instance.(T)
	if !ok {
		return zero, fmt.Errorf("%s/%s 返回的类型 %T 不匹配期望类型 %T", serviceType, providerName, instance, zero)
	}

	return typed, nil
}

// ListProviders 列出所有已注册的 provider（或按 serviceType 过滤）
func (r *Registry) ListProviders(serviceType string) []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ProviderInfo

	if serviceType != "" {
		if providers, ok := r.entries[serviceType]; ok {
			for _, entry := range providers {
				result = append(result, entry.info)
			}
		}
		return result
	}

	for _, providers := range r.entries {
		for _, entry := range providers {
			result = append(result, entry.info)
		}
	}
	return result
}

// HasProvider 检查指定 provider 是否已注册
func (r *Registry) HasProvider(serviceType, providerName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if providers, ok := r.entries[serviceType]; ok {
		_, exists := providers[providerName]
		return exists
	}
	return false
}

// GetProviderInfo 获取指定 provider 的信息
func (r *Registry) GetProviderInfo(serviceType, providerName string) *ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if providers, ok := r.entries[serviceType]; ok {
		if entry, exists := providers[providerName]; exists {
			info := entry.info
			return &info
		}
	}
	return nil
}
