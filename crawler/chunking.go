package crawler

import (
	"axora/pkg/embedding"
	"context"
	"fmt"
	"strings"

	"github.com/daulet/tokenizers"
	"github.com/tmc/langchaingo/textsplitter"
	"go.uber.org/zap"
)

type ChunkOutput struct {
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

type ChunkingClient interface {
	ChunkText(text string, chunkType string, ch chan<- ChunkOutput)
}

type Chunker struct {
	tokenizer       *tokenizers.Tokenizer
	maxTokens       int
	minTokens       int
	embeddingClient embedding.Client
	maxBatchSize    int
	logger          *zap.Logger
}

func NewChunker(maxTokens int, embed embedding.Client, logger *zap.Logger,
	tokenizerFilePath string) (*Chunker, error) {
	tokenizer, err := tokenizers.FromFile(tokenizerFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer from pretrained or local files: %w", err)
	}
	return &Chunker{
		tokenizer:       tokenizer,
		maxTokens:       maxTokens,
		embeddingClient: embed,
		maxBatchSize:    32,
		logger:          logger,
		minTokens:       75,
	}, nil
}

func (sc *Chunker) ChunkText(text string, chunkType string, ch chan<- ChunkOutput) {
	defer close(ch)

	var chunks []string
	var err error

	switch chunkType {
	case "md":
		chunks, err = sc.chunkMarkdown(text)
	case "sen":
		chunks, err = sc.chunkSentence(text)
	default:
		sc.logger.Error("unsupported chunk type", zap.String("type", chunkType))
		return
	}

	if err != nil {
		sc.logger.Error("failed to chunk text", zap.Error(err))
		return
	}

	if len(chunks) == 0 {
		return
	}

	for i := 0; i < len(chunks); i += sc.maxBatchSize {
		end := i + sc.maxBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		embeddings, err := sc.embeddingClient.GetEmbeddings(context.Background(), batch)
		if err != nil {
			sc.logger.Error("failed to get embeddings for batch",
				zap.Int("start", i),
				zap.Int("end", end),
				zap.Error(err))
			continue
		}

		for j, chunk := range batch {
			ch <- ChunkOutput{
				Text:   chunk,
				Vector: embeddings[j],
			}
		}
	}
}

func (sc *Chunker) chunkMarkdown(text string) ([]string, error) {
	splitter := textsplitter.NewMarkdownTextSplitter(
		textsplitter.WithHeadingHierarchy(true),
		textsplitter.WithChunkOverlap(50),
	)

	c, err := splitter.SplitText(text)
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown: %w", err)
	}
	return sc.doChunk(c)
}

func (sc *Chunker) chunkSentence(text string) ([]string, error) {
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators([]string{"\n\n", "\n", ".", "!", "?", " ", ""}),
		textsplitter.WithKeepSeparator(true),
		textsplitter.WithChunkOverlap(50),
	)

	c, err := splitter.SplitText(text)
	if err != nil {
		return nil, fmt.Errorf("failed to split text: %w", err)
	}

	return sc.doChunk(c)
}

func (sc *Chunker) doChunk(chunks []string) ([]string, error) {
	var validChunks []string
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			continue
		}

		ids, _ := sc.tokenizer.Encode(trimmed, false)
		tokenCount := len(ids)
		sc.logger.Info("token_count", zap.Int("count", tokenCount))

		if tokenCount < 75 {
			continue
		}
		if tokenCount <= sc.maxTokens {
			validChunks = append(validChunks, trimmed)
		} else {
			// TODO: use something
		}
	}

	return validChunks, nil
}
