package weaviatedb

import (
	"context"
	"errors"

	"github.com/weaviate/weaviate-go-client/v5/weaviate"
)

func NewClient(url string) (*weaviate.Client, error) {
	cfg := weaviate.Config{
		Host:   url,
		Scheme: "http",
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, errors.New("weaviate: error creating client: " + err.Error())
	}

	ready, err := client.Misc().ReadyChecker().Do(context.Background())
	if err != nil {
		return nil, errors.New("weaviate: error checking client ready")
	}
	if !ready {
		return nil, errors.New("weaviate: client not ready")
	}

	return client, nil
}
