package milvusdb

import (
	"context"
	"fmt"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
)

func NewClient(host string, port string) (client.Client, error) {
	ctx := context.Background()

	milvusClient, err := client.NewGrpcClient(ctx, fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	return milvusClient, nil
}
