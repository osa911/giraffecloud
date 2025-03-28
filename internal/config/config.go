package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token     string            `yaml:"token"`
	Endpoints []EndpointConfig  `yaml:"endpoints"`
	Server    ServerConfig      `yaml:"server"`
}

type EndpointConfig struct {
	Name     string `yaml:"name"`
	Local    string `yaml:"local"`
	Remote   string `yaml:"remote"`
	Protocol string `yaml:"protocol"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

var DefaultConfig = Config{
	Server: ServerConfig{
		Host: "api.giraffecloud.com",
		Port: 443,
	},
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".giraffecloud", "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return nil, err
	}

	// If config file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig
		data, err := yaml.Marshal(config)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(configPath, data, 0600); err != nil {
			return nil, err
		}
		return &config, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(homeDir, ".giraffecloud", "config.yaml")
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}