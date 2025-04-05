package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type ProviderConfig struct {
	Name   string            `yaml:"name"`
	Path   string            `yaml:"path"`
	Config map[string]string `yaml:"config"`
}

type AuthenticatorConfig struct {
	Name   string            `yaml:"name"`
	Path   string            `yaml:"path"`
	Config map[string]string `yaml:"config"`
}

type AppConfig struct {
	Providers      []ProviderConfig      `yaml:"providers"`
	Authenticators []AuthenticatorConfig `yaml:"authenticators"`
}

func LoadConfig(filePath string) (*AppConfig, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config AppConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
