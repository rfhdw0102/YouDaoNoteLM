package service

type ConfigHealthChecker struct{}

func NewConfigHealthChecker() *ConfigHealthChecker {
	return &ConfigHealthChecker{}
}

func (c *ConfigHealthChecker) resolveAPIURL(provider, apiURL string) string {
	if provider == "doubao" && apiURL == "" {
		return "https://ark.cn-beijing.volces.com/api/v3"
	}
	return ""
}
