package crawler

import (
	"axora/pkg/embedding"
	"context"
	"fmt"

	"github.com/neurosnap/sentences"
	"github.com/pkoukk/tiktoken-go"
)

type ChunkOutput struct {
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

type ChunkingClient interface {
	ChunkText(text string) ([]ChunkOutput, error)
}

type SentenceChunker struct {
	tokenizer         *tiktoken.Tiktoken
	sentenceTokenizer *sentences.DefaultSentenceTokenizer
	maxTokens         int
	embeddingClient   embedding.Client
}

func NewSentenceChunker(maxTokens int, embed embedding.Client) (*SentenceChunker, error) {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	sentenceTokenizer := sentences.NewSentenceTokenizer(nil)

	return &SentenceChunker{
		tokenizer:         tokenizer,
		sentenceTokenizer: sentenceTokenizer,
		maxTokens:         maxTokens,
		embeddingClient:   embed,
	}, nil
}

func (sc *SentenceChunker) ChunkText(text string) ([]ChunkOutput, error) {
	sentenceObjs := sc.sentenceTokenizer.Tokenize(text)

	if len(sentenceObjs) == 0 {
		return nil, nil
	}

	var chunks []string
	var currentChunk string
	var currentTokens int

	for _, sentenceObj := range sentenceObjs {
		sentence := sentenceObj.Text

		tokens := sc.tokenizer.Encode(sentence, nil, nil)
		sentenceTokenCount := len(tokens)

		// If adding this sentence would exceed max tokens, save current chunk
		if currentTokens+sentenceTokenCount > sc.maxTokens && currentChunk != "" {
			chunks = append(chunks, currentChunk)
			currentChunk = sentence
			currentTokens = sentenceTokenCount
		} else {
			currentChunk += sentence
			currentTokens += sentenceTokenCount
		}
	}

	// Add the last chunk if it's not empty
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	embeddings, err := sc.embeddingClient.GetEmbeddings(context.Background(), chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	// Create chunk outputs
	result := make([]ChunkOutput, len(chunks))
	for i, chunk := range chunks {
		result[i] = ChunkOutput{
			Text:   chunk,
			Vector: embeddings[i],
		}
	}

	return result, nil
}
