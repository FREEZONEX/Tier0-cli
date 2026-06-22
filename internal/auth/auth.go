package auth

// Auth provides authentication helpers.
// Tier0 CLI stores a Personal API Key (sk-per-xxx). OpenAPI calls send it via
// X-API-Key in internal/client; ResolveAuthHeaders is kept for callers that
// explicitly need Authorization: Bearer.

// ResolveAuthHeaders returns authentication headers for an API key.
func ResolveAuthHeaders(apiKey string) map[string]string {
	if apiKey == "" {
		return nil
	}
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
}
