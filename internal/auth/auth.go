package auth

// Auth provides authentication helpers.
// Tier0 CLI stores a Personal API Key (sk-per-xxx). OpenAPI calls send it via
// X-API-Key in internal/client; ResolveAuthHeaders is kept for callers that
// explicitly need Authorization: Bearer.

// ResolveAuthHeaders 根据 API Key 返回认证请求头
func ResolveAuthHeaders(apiKey string) map[string]string {
	if apiKey == "" {
		return nil
	}
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
}
