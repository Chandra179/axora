package crawler

import (
	"axora/pkg/embedding"
	"context"
	"fmt"
	"strings"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
	"github.com/pkoukk/tiktoken-go"
	"github.com/tmc/langchaingo/textsplitter"
)

type ChunkOutput struct {
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
}

type ChunkingClient interface {
	ChunkText(text string, chunkType string) ([]ChunkOutput, error)
}

type Chunker struct {
	tokenizer         *tiktoken.Tiktoken
	sentenceTokenizer *sentences.DefaultSentenceTokenizer
	maxTokens         int
	embeddingClient   embedding.Client
	maxBatchSize      int
}

func NewChunker(maxTokens int, embed embedding.Client) (*Chunker, error) {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	sentenceTokenizer, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get sentence tokenizer: %w", err)
	}
	return &Chunker{
		maxTokens:         maxTokens,
		embeddingClient:   embed,
		maxBatchSize:      32,
		tokenizer:         tokenizer,
		sentenceTokenizer: sentenceTokenizer,
	}, nil
}

func (sc *Chunker) ChunkText(text string, chunkType string) ([]ChunkOutput, error) {
	var chunks []string
	var err error

	switch chunkType {
	case "md":
		chunks, err = sc.chunkMarkdown(text)
	case "sen":
		chunks, err = sc.chunkSentence(text)
	case "sen2":
		chunks, err = sc.chunkSentence2(text)
	default:
		return nil, fmt.Errorf("unsupported chunk type: %s", chunkType)
	}

	if err != nil {
		return nil, err
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	var allEmbeddings [][]float32

	for i := 0; i < len(chunks); i += sc.maxBatchSize {
		end := i + sc.maxBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		embeddings, err := sc.embeddingClient.GetEmbeddings(context.Background(), batch)
		if err != nil {
			return nil, fmt.Errorf("failed to get embeddings for batch %d-%d: %w", i, end, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	result := make([]ChunkOutput, len(chunks))
	for i, chunk := range chunks {
		result[i] = ChunkOutput{
			Text:   chunk,
			Vector: allEmbeddings[i],
		}
	}

	return result, nil
}

func (sc *Chunker) chunkMarkdown(text string) ([]string, error) {
	splitter := textsplitter.NewMarkdownTextSplitter(
		// textsplitter.WithHeadingHierarchy(true),
		textsplitter.WithModelName("bge-base-en-v1.5"),
		textsplitter.WithChunkSize(sc.maxTokens),
	)

	chunks, err := splitter.SplitText(text)
	if err != nil {
		return nil, fmt.Errorf("failed to split markdown: %w", err)
	}

	// Filter out empty chunks
	var validChunks []string
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if trimmed != "" {
			validChunks = append(validChunks, trimmed)
		}
	}

	return validChunks, nil
}

func (sc *Chunker) chunkSentence(text string) ([]string, error) {
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithSeparators([]string{"\n\n", "\n", " ", "", "\n", ".\n", ".\n", "!\n", "?\n"}),
		textsplitter.WithModelName("bge-base-en-v1.5"),
		textsplitter.WithKeepSeparator(true),
		textsplitter.WithChunkSize(sc.maxTokens),
	)

	chunks, err := splitter.SplitText(text)
	if err != nil {
		return nil, fmt.Errorf("failed to split sentences: %w", err)
	}

	// Filter out empty chunks
	var validChunks []string
	for _, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if trimmed != "" {
			validChunks = append(validChunks, trimmed)
		}
	}

	return validChunks, nil
}

func (sc *Chunker) chunkSentence2(text string) ([]string, error) {
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
		} else if sentenceTokenCount > sc.maxTokens {
			// Handle case where a single sentence exceeds max tokens
			// Split it further or skip it with a warning
			continue
		} else {
			currentChunk += sentence
			currentTokens += sentenceTokenCount
		}
	}

	// Add the last chunk if it's not empty
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks, nil
}
