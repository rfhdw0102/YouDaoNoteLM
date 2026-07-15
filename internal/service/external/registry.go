package external

import (
	"fmt"
	"sync"

	"YoudaoNoteLm/pkg/logger"

	"go.uber.org/zap"
)

// FactoryFunc provider factory without type constraints.
type FactoryFunc func(cfg *ServiceConfig) (interface{}, error)

// TypedFactoryFunc typed provider factory.
type TypedFactoryFunc[T any] func(cfg *ServiceConfig) (T, error)

// ProviderInfo describes a registered provider.
type ProviderInfo struct {
	ServiceType  string            `json:"service_type"`
	Provider     string            `json:"provider"`
	DisplayName  string            `json:"display_name"`
	RequiredKeys []string          `json:"required_keys"`
	OptionalKeys []string          `json:"optional_keys"`
	Implemented  bool              `json:"implemented"`
	KeyLabels    map[string]string `json:"key_labels,omitempty"`
}

type providerEntry struct {
	info    ProviderInfo
	factory FactoryFunc
}

// Registry stores service/provider factories.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]map[string]providerEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]map[string]providerEntry)}
}

var globalRegistry = NewRegistry()

func GetGlobalRegistry() *Registry { return globalRegistry }

func (r *Registry) Register(serviceType, providerName, displayName string,
	requiredKeys, optionalKeys []string, factory FactoryFunc, keyLabels map[string]string) {

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.entries[serviceType] == nil {
		r.entries[serviceType] = make(map[string]providerEntry)
	}

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

	logger.Info("Provider registered",
		zap.String("service_type", serviceType),
		zap.String("provider", providerName),
		zap.Bool("implemented", implemented),
	)
}

func (r *Registry) RegisterAlias(serviceType, aliasName, targetName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	providers := r.entries[serviceType]
	if providers == nil {
		return
	}
	target, ok := providers[targetName]
	if !ok {
		return
	}
	info := target.info
	info.Provider = aliasName
	providers[aliasName] = providerEntry{info: info, factory: target.factory}
}

func RegisterTyped[T any](r *Registry, serviceType, providerName, displayName string,
	requiredKeys, optionalKeys []string, factory TypedFactoryFunc[T], keyLabels map[string]string) {

	wrappedFactory := func(cfg *ServiceConfig) (interface{}, error) {
		return factory(cfg)
	}
	r.Register(serviceType, providerName, displayName, requiredKeys, optionalKeys, wrappedFactory, keyLabels)
}

func (r *Registry) Create(serviceType, providerName string, cfg *ServiceConfig) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers, ok := r.entries[serviceType]
	if !ok {
		return nil, fmt.Errorf("unsupported service type: %s", serviceType)
	}

	entry, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("unsupported %s provider: %s", serviceType, providerName)
	}

	instance, err := entry.factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("create %s/%s failed: %w", serviceType, providerName, err)
	}
	return instance, nil
}

func CreateTyped[T any](r *Registry, serviceType, providerName string, cfg *ServiceConfig) (T, error) {
	var zero T
	instance, err := r.Create(serviceType, providerName, cfg)
	if err != nil {
		return zero, err
	}
	typed, ok := instance.(T)
	if !ok {
		return zero, fmt.Errorf("%s/%s returned %T, want %T", serviceType, providerName, instance, zero)
	}
	return typed, nil
}

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

func (r *Registry) HasProvider(serviceType, providerName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if providers, ok := r.entries[serviceType]; ok {
		_, exists := providers[providerName]
		return exists
	}
	return false
}

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
