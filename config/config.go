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

	MilvusHost string
	MilvusPort string
}

func Load() (*Config, error) {
	appPort, err := strconv.Atoi(getEnv("APP_PORT"))
	if err != nil {
		return nil, err
	}

	return &Config{
		AppPort: appPort,

		SerpApiKey:       getEnv("SERP_API_KEY"),
		AllMinilmL6V2URL: getEnv("MINILML6V2_URL"),

		MilvusPort: getEnv("MILVUS_PORT"),
		MilvusHost: getEnv("MILVUS_HOST"),
	}, nil
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return value
}
