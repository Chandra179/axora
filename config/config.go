package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	AppPort int

	SerpApiKey       string
	AllMinilmL6V2URL string

	QdrantHost string
	QdrantPort int

	ChunkingURL string
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
		AppPort: appPort,

		SerpApiKey:       getEnv("SERP_API_KEY"),
		AllMinilmL6V2URL: getEnv("MPNETBASEV2_URL"),

		QdrantPort: qdrantPort,
		QdrantHost: getEnv("QDRANT_HOST"),

		ChunkingURL: getEnv("CHUNKING_URL"),
	}, nil
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return value
}
