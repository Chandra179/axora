package config

import (
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AppPort       int
	ProxyURL      string
	PostgresDBUrl string
	DownloadPath  string
	KafkaURL      string
	QdrantHost    string
	QdrantPort    int
}

func Load() (*Config, error) {
	appPort, err := strconv.Atoi(getEnv("APP_PORT"))
	if err != nil {
		return nil, err
	}
	qdrantPort, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT"))
	if err != nil {
		return nil, err
	}

	return &Config{
		AppPort:       appPort,
		ProxyURL:      getEnv("PROXY_URL"),
		DownloadPath:  getEnv("DOWNLOAD_PATH"),
		PostgresDBUrl: getEnv("POSTGRES_DB_URL"),
		KafkaURL:      getEnv("KAFKA_URL"),
		QdrantPort:    qdrantPort,
		QdrantHost:    getEnv("QDRANT_HOST"),
	}, nil
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return value
}

type DomainConfig struct {
	Domains []string `yaml:"domains"`
}

func LoadDomains(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read YAML file: %v", err)
	}

	var cfg DomainConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse YAML: %v", err)
	}

	return cfg.Domains
}
