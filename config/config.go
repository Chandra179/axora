package config

import (
	"log"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProxyURL               string
	DownloadPath           string
	QdrantHost             string
	MpnetBaseV2Url         string
	DomainWhiteListPath    string
	EmbedModelID           string
	TokenizerFilePath      string
	BoltDBPath             string
	QdrantPort             int
	MaxEmbedModelTokenSize int
	AppPort                int
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
	tokenSize, err := strconv.Atoi(getEnv("MAX_EMBED_MODEL_TOKEN_SIZE"))
	if err != nil {
		return nil, err
	}

	return &Config{
		ProxyURL:               getEnv("PROXY_URL"),
		EmbedModelID:           getEnv("EMBED_MODEL_ID"),
		DownloadPath:           getEnv("DOWNLOAD_PATH"),
		QdrantHost:             getEnv("QDRANT_HOST"),
		MpnetBaseV2Url:         getEnv("MPNET_BASEV2_URL"),
		DomainWhiteListPath:    getEnv("DOMAIN_WHITELIST_PATH"),
		TokenizerFilePath:      getEnv("TOKENIZER_FILE_PATH"),
		BoltDBPath:             getEnv("BOLTDB_PATH"),
		MaxEmbedModelTokenSize: tokenSize,
		QdrantPort:             qdrantPort,
		AppPort:                appPort,
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
