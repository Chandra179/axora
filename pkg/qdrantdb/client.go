package qdrantdb

import (
	"github.com/qdrant/go-client/qdrant"
)

type CrawlClient struct {
	Client *qdrant.Client
}

func NewClient(host string, port int) (*CrawlClient, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port, // gRPC port
	})
	if err != nil {
		return nil, err
	}
	return &CrawlClient{Client: client}, err
}
