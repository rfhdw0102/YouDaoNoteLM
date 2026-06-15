package service

import (
	"net/url"
	"sort"
	"strings"

	"YoudaoNoteLm/internal/service/external"
)

// NormalizeSearchResults 将 provider 结果清洗为统一结果。
func NormalizeSearchResults(results []external.SearchProviderResult, needContent bool, allowedDomains, blockedDomains []string) []SearchResult {
	allowed := toDomainSet(allowedDomains)
	blocked := toDomainSet(blockedDomains)
	seen := make(map[string]struct{}, len(results))
	normalized := make([]SearchResult, 0, len(results))

	for _, item := range results {
		rawURL := strings.TrimSpace(item.URL)
		title := strings.TrimSpace(item.Title)
		if rawURL == "" || title == "" {
			continue
		}

		host := hostFromURL(rawURL)
		if len(allowed) > 0 && !matchDomainSet(host, allowed) {
			continue
		}
		if matchDomainSet(host, blocked) {
			continue
		}

		key := normalizeURL(rawURL)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		content := ""
		if needContent {
			content = compactText(firstNonEmpty(item.Summary, item.Snippet), 1200)
		}

		normalized = append(normalized, SearchResult{
			Title:         compactText(title, 200),
			Snippet:       compactText(firstNonEmpty(item.Snippet, item.Summary), 500),
			URL:           rawURL,
			DisplayURL:    firstNonEmpty(strings.TrimSpace(item.DisplayURL), host),
			PublishedAt:   strings.TrimSpace(item.PublishedAt),
			SiteName:      firstNonEmpty(strings.TrimSpace(item.SiteName), host),
			Score:         item.Score,
			Content:       content,
			ProviderRawID: strings.TrimSpace(item.ID),
			Meta:          item.Meta,
		})
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Score == normalized[j].Score {
			return normalized[i].PublishedAt > normalized[j].PublishedAt
		}
		return normalized[i].Score > normalized[j].Score
	})

	return normalized
}

// BuildSearchSummary 按结果拼接简要摘要。
func BuildSearchSummary(results []SearchResult) string {
	if len(results) == 0 {
		return ""
	}

	parts := make([]string, 0, 3)
	for i := 0; i < len(results) && i < 3; i++ {
		text := strings.TrimSpace(firstNonEmpty(results[i].Content, results[i].Snippet))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func toDomainSet(domains []string) map[string]struct{} {
	if len(domains) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		host := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")))
		host = strings.TrimPrefix(host, "www.")
		host = strings.TrimSuffix(host, "/")
		if host != "" {
			result[host] = struct{}{}
		}
	}
	return result
}

func matchDomainSet(host string, domainSet map[string]struct{}) bool {
	if len(domainSet) == 0 || host == "" {
		return false
	}
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	for domain := range domainSet {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func normalizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	parsed.Fragment = ""
	if host := strings.ToLower(parsed.Hostname()); host != "" {
		parsed.Host = host
	}
	return parsed.String()
}

func compactText(value string, maxLen int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if maxLen > 0 && len(value) > maxLen {
		return value[:maxLen] + "..."
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
