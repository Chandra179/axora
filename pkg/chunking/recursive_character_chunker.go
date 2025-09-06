package chunking

import (
	"axora/pkg/embedding"
	"context"
	"fmt"

	"github.com/tmc/langchaingo/textsplitter"
)

type RecursiveCharacterChunking struct {
	splitter *textsplitter.RecursiveCharacter
	embed    embedding.Client
}

func NewRecursiveCharacterChunking(embed embedding.Client) *RecursiveCharacterChunking {
	splitter := textsplitter.NewRecursiveCharacter()

	// Optional: Configure the splitter with custom settings
	splitter = textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(1000),
		textsplitter.WithChunkOverlap(200),
		textsplitter.WithSeparators([]string{"\n\n", "\n", " "}),
	)

	return &RecursiveCharacterChunking{
		splitter: &splitter,
		embed:    embed,
	}
}

func (c *RecursiveCharacterChunking) ChunkText(text string) ([]ChunkOutput, error) {
	chunks, err := c.splitter.SplitText(text)
	if err != nil {
		return nil, err
	}

	result := make([]ChunkOutput, len(chunks))
	for i, chunk := range chunks {
		vec, err := c.embed.GetEmbeddings(context.Background(), []string{chunk})
		if err != nil {
			return nil, fmt.Errorf("embed failed: %w", err)
		}
		result[i] = ChunkOutput{
			Text:   chunk,
			Vector: vec[0],
		}
	}

	return result, nil
}
