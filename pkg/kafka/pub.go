package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaClient struct {
	writer *kafka.Writer
	url    string
}

// NewClient creates a new Kafka client with the given broker URL
func NewClient(url string) (*KafkaClient, error) {
	if url == "" {
		return nil, fmt.Errorf("kafka URL cannot be empty")
	}

	// Create a new writer without specifying a topic yet
	// We'll set the topic per message in the Publish method
	writer := &kafka.Writer{
		Addr:         kafka.TCP(url),
		Balancer:     &kafka.LeastBytes{},
		MaxAttempts:  3,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Compression:  kafka.Snappy,
		Async:        false, // Set to true for fire-and-forget
	}

	client := &KafkaClient{
		writer: writer,
		url:    url,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := kafka.DialContext(ctx, "tcp", url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to kafka: %w", err)
	}
	conn.Close()

	return client, nil
}

// Publish sends a message to the specified Kafka topic
func (k *KafkaClient) Publish(topic string, msg []byte) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	message := kafka.Message{
		Topic: topic,
		Value: msg,
		Time:  time.Now(),
	}

	err := k.writer.WriteMessages(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	return nil
}

// PublishWithKey sends a message with a key to the specified Kafka topic
// The key is used for partitioning
func (k *KafkaClient) PublishWithKey(topic string, key []byte, msg []byte) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	message := kafka.Message{
		Topic: topic,
		Key:   key,
		Value: msg,
		Time:  time.Now(),
	}

	err := k.writer.WriteMessages(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	return nil
}

// Close gracefully closes the Kafka writer
func (k *KafkaClient) Close() error {
	if k.writer != nil {
		return k.writer.Close()
	}
	return nil
}
