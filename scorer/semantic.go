package scorer

import (
	"context"
	"fmt"
	"math"
)

// EmbeddingClient generates vector embeddings for text.
type EmbeddingClient interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// SemanticScorer evaluates semantic similarity between expected and actual
// outputs using embedding cosine similarity.
type SemanticScorer struct {
	Client    EmbeddingClient
	Threshold float64
}

// NewSemanticScorer creates a semantic similarity scorer.
// Threshold is the minimum cosine similarity to pass (default 0.8).
func NewSemanticScorer(client EmbeddingClient, threshold float64) *SemanticScorer {
	if threshold <= 0 {
		threshold = 0.8
	}
	return &SemanticScorer{Client: client, Threshold: threshold}
}

func (s *SemanticScorer) Name() string { return "semantic" }

func (s *SemanticScorer) Score(ctx context.Context, input *Input) (*Output, error) {
	if input.Expected == "" {
		return &Output{
			Score:  0,
			Passed: false,
			Reason: "no expected output for semantic comparison",
		}, nil
	}

	embeddings, err := s.Client.Embed(ctx, []string{input.Expected, input.Actual})
	if err != nil {
		return nil, fmt.Errorf("semantic: embed: %w", err)
	}

	if len(embeddings) < 2 {
		return nil, fmt.Errorf("semantic: expected 2 embeddings, got %d", len(embeddings))
	}

	similarity := cosineSimilarity(embeddings[0], embeddings[1])
	// Normalize from [-1,1] to [0,1].
	score := (similarity + 1) / 2

	return &Output{
		Score:  score,
		Passed: similarity >= s.Threshold,
		Reason: fmt.Sprintf("cosine similarity: %.4f (threshold: %.2f)", similarity, s.Threshold),
		Details: map[string]any{
			"cosine_similarity": similarity,
			"threshold":         s.Threshold,
		},
	}, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}

	return dot / denom
}
