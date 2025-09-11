package chunking

import (
	"axora/pkg/embedding"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/tmc/langchaingo/textsplitter"
)

type RecursiveCharacterChunking struct {
	splitter   *textsplitter.RecursiveCharacter
	embed      embedding.Client
	maxRetries int
	baseDelay  time.Duration
}

func NewRecursiveCharacterChunking(embed embedding.Client) *RecursiveCharacterChunking {
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(700),
		textsplitter.WithChunkOverlap(200),
		textsplitter.WithSeparators([]string{"\n\n", "\n", " "}),
	)
	return &RecursiveCharacterChunking{
		splitter:   &splitter,
		embed:      embed,
		maxRetries: 5,
		baseDelay:  100 * time.Millisecond,
	}
}

func (c *RecursiveCharacterChunking) ChunkText(text string) ([]ChunkOutput, error) {
	chunks, err := c.splitter.SplitText(text)
	if err != nil {
		return nil, err
	}

	result := make([]ChunkOutput, len(chunks))
	for i, chunk := range chunks {
		vec, err := c.getEmbeddingsWithRetry(context.Background(), []string{chunk})
		if err != nil {
			return nil, fmt.Errorf("embed failed after retries: %w", err)
		}
		result[i] = ChunkOutput{
			Text:   chunk,
			Vector: vec[0],
		}
	}

	return result, nil
}

func (c *RecursiveCharacterChunking) getEmbeddingsWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		vec, err := c.embed.GetEmbeddings(ctx, texts)
		if err == nil {
			return vec, nil
		}

		lastErr = err

		// Don't wait after the last attempt
		if attempt < c.maxRetries {
			delay := c.calculateBackoffDelay(attempt)

			// Check if context is cancelled during sleep
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return nil, lastErr
}

func (c *RecursiveCharacterChunking) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt with some jitter
	delay := float64(c.baseDelay) * math.Pow(2, float64(attempt))

	// Add up to 25% jitter to avoid thundering herd
	jitter := delay * 0.25 * (0.5 - (float64(time.Now().UnixNano()%1000) / 1000))

	return time.Duration(delay + jitter)
}
