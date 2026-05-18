package auth

// Auth 提供认证相关辅助函数
// Tier0 使用 Personal API Key（sk-per-xxx）作为 Bearer Token
// 直接通过 Authorization: Bearer <api-key> 头部进行认证

// ResolveAuthHeaders 根据 API Key 返回认证请求头
func ResolveAuthHeaders(apiKey string) map[string]string {
	if apiKey == "" {
		return nil
	}
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
	}
}
