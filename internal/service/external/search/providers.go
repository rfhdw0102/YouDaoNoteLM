// internal/service/external/search/providers.go
package search

import (
	"YoudaoNoteLm/internal/service/external"
)

const ServiceType = "search"

func init() {
	r := external.GetGlobalRegistry()

	// SearXNG（自部署）
	r.Register(ServiceType, "searxng", "SearXNG（自部署）",
		[]string{"api_url"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewSearXNGEngine(cfg.APIURL), nil
		}, map[string]string{
			"api_url": "API 地址",
		})

	// Tavily（AI 搜索）
	r.Register(ServiceType, "tavily", "Tavily（AI 搜索）",
		[]string{"api_key"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewTavilyEngine(cfg.APIKey), nil
		}, map[string]string{
			"api_key": "API Key",
		})

	// Serper（Google 搜索代理）
	r.Register(ServiceType, "serper", "Serper（Google 搜索）",
		[]string{"api_key"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewSerperEngine(cfg.APIKey), nil
		}, map[string]string{
			"api_key": "API Key",
		})

	// Bing（微软搜索）
	r.Register(ServiceType, "bing", "Bing 搜索",
		[]string{"api_key"}, nil,
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewBingEngine(cfg.APIKey), nil
		}, map[string]string{
			"api_key": "API Key",
		})

	// Bocha（博查搜索）
	r.Register(ServiceType, "bocha", "博查搜索",
		[]string{"api_key"}, []string{"api_url"},
		func(cfg *external.ServiceConfig) (interface{}, error) {
			return NewBochaEngine(cfg.APIURL, cfg.APIKey), nil
		}, map[string]string{
			"api_key": "API Key",
			"api_url": "API 地址（可选）",
		})

	// Custom（自定义 POST JSON）
	r.Register(ServiceType, "custom", "自定义搜索 API",
		[]string{"api_url"}, []string{"api_key"},
		func(cfg *external.ServiceConfig) (interface{}, error) {
			name := cfg.GetExtraString("name")
			if name == "" {
				name = "custom"
			}
			return NewCustomEngine(name, cfg.APIURL, cfg.APIKey), nil
		}, map[string]string{
			"api_url": "API 地址",
			"api_key": "API Key（可选）",
		})
}
