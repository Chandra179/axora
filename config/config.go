package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	AppPort  int
	ProxyURL string
}

func Load() (*Config, error) {
	appPort, err := strconv.Atoi(getEnv("APP_PORT"))
	if err != nil {
		return nil, err
	}

	return &Config{
		AppPort:  appPort,
		ProxyURL: getEnv("PROXY_URL"),
	}, nil
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s is required but not set", key)
	}
	return value
}
