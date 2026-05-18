package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile 用户配置
type Profile struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
}

// configDir 返回配置文件目录
func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tier0")
}

// configFile 返回配置文件路径
func configFile() string {
	return filepath.Join(configDir(), "config.json")
}

// SaveProfile 保存配置到文件
func SaveProfile(profile Profile) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configFile(), data, 0600); err != nil {
		return fmt.Errorf("写入配置失败: %w", err)
	}
	return nil
}

// LoadProfile 从文件加载配置
func LoadProfile() (Profile, error) {
	var profile Profile

	data, err := os.ReadFile(configFile())
	if err != nil {
		if os.IsNotExist(err) {
			return profile, nil
		}
		return profile, fmt.Errorf("读取配置失败: %w", err)
	}

	if err := json.Unmarshal(data, &profile); err != nil {
		return profile, fmt.Errorf("解析配置失败: %w", err)
	}
	return profile, nil
}
