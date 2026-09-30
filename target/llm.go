package target

import (
	"context"
	"time"
)

// LLMClient is the interface for calling a language model.
// Implementations bridge to specific LLM providers.
type LLMClient interface {
	Complete(ctx context.Context, model, systemPrompt, userInput string, temperature float64) (*LLMResponse, error)
}

// LLMResponse holds the response from an LLM call.
type LLMResponse struct {
	Output string
	Tokens int
	Cost   float64
}

// LLMTarget calls a language model directly (traditional eval).
type LLMTarget struct {
	Model        string
	SystemPrompt string
	Temperature  float64
	Client       LLMClient
}

// NewLLMTarget creates a target that calls an LLM directly.
func NewLLMTarget(client LLMClient, model, systemPrompt string, temperature float64) *LLMTarget {
	return &LLMTarget{
		Client:       client,
		Model:        model,
		SystemPrompt: systemPrompt,
		Temperature:  temperature,
	}
}

func (t *LLMTarget) Name() string { return "llm:" + t.Model }

func (t *LLMTarget) Call(ctx context.Context, input string) (*Output, error) {
	start := time.Now()
	model, prompt, temperature := t.Model, t.SystemPrompt, t.Temperature
	if o, ok := CallOptionsFrom(ctx); ok {
		if o.SystemPrompt != "" {
			prompt = o.SystemPrompt
		}
		if o.Model != "" {
			model = o.Model
		}
		temperature = o.Temperature
	}
	resp, err := t.Client.Complete(ctx, model, prompt, input, temperature)
	elapsed := time.Since(start)
	if err != nil {
		return nil, err
	}
	return &Output{
		Output:  resp.Output,
		Latency: elapsed,
		Tokens:  resp.Tokens,
		Cost:    resp.Cost,
	}, nil
}
