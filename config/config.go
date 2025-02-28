package config

import (
	"gopkg.in/yaml.v2"
	"os"
)

// Config 配置结构体
type Config struct {
	Algorithm      string         `yaml:"algorithm"`
	ThreadModel    string         `yaml:"thread_model"`
	DataStructure  string         `yaml:"data_structure"`
	EvictionPolicy string         `yaml:"eviction_policy"`
	Database       DatabaseConfig `yaml:"database_config"`
}

// DatabaseConfig MySQL数据库配置
type DatabaseConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	DatabaseName string `yaml:"database_name"`
}

// LoadConfig 读取并解析配置文件
func LoadConfig() (*Config, error) {
	file, err := os.Open("E:\\coder\\GoWorks\\raft_learning\\config\\config.yaml")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
