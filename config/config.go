package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	MongoUsername     string
	MongoPassword     string
	MongoDatabaseName string
	MongoPort         int
	MongoURL          string

	MongoExpressUsername string
	MongoExpressPassword string
	MongoExpressPort     int

	AppPort int

	SerpApiKey        string
	TEIModelClientURL string

	MilvusHost string
	MilvusPort string
}

func Load() (*Config, error) {
	mongoPort, err := strconv.Atoi(getEnv("MONGO_PORT"))
	if err != nil {
		return nil, err
	}

	mongoExpressPort, err := strconv.Atoi(getEnv("MONGO_EXPRESS_PORT"))
	if err != nil {
		return nil, err
	}

	appPort, err := strconv.Atoi(getEnv("APP_PORT"))
	if err != nil {
		return nil, err
	}

	return &Config{
		MongoUsername:     getEnv("MONGO_USERNAME"),
		MongoPassword:     getEnv("MONGO_PASSWORD"),
		MongoDatabaseName: getEnv("MONGO_DATABASE"),
		MongoPort:         mongoPort,
		MongoURL:          getEnv("MONGO_URL"),

		MongoExpressUsername: getEnv("MONGO_EXPRESS_USERNAME"),
		MongoExpressPassword: getEnv("MONGO_EXPRESS_PASSWORD"),
		MongoExpressPort:     mongoExpressPort,

		AppPort: appPort,

		SerpApiKey:        getEnv("SERP_API_KEY"),
		TEIModelClientURL: getEnv("TEI_MODEL_CLIENT_URL"),

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
