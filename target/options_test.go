package target

import (
	"context"
	"testing"
)

type recordingClient struct{ model, prompt string; temp float64 }

func (c *recordingClient) Complete(_ context.Context, model, systemPrompt, _ string, temperature float64) (*LLMResponse, error) {
	c.model, c.prompt, c.temp = model, systemPrompt, temperature
	return &LLMResponse{Output: "ok"}, nil
}

func TestLLMTargetUsesTheRunsChoices(t *testing.T) {
	c := &recordingClient{}
	tgt := NewLLMTarget(c, "built-model", "built-prompt", 0.9)

	ctx := WithCallOptions(context.Background(), CallOptions{SystemPrompt: "version 3", Model: "run-model", Temperature: 0.2})
	if _, err := tgt.Call(ctx, "hi"); err != nil {
		t.Fatal(err)
	}
	if c.prompt != "version 3" || c.model != "run-model" || c.temp != 0.2 {
		t.Fatalf("target ignored the run: %+v", c)
	}
}

func TestLLMTargetFallsBackWithoutOptions(t *testing.T) {
	c := &recordingClient{}
	tgt := NewLLMTarget(c, "built-model", "built-prompt", 0.9)
	if _, err := tgt.Call(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	if c.prompt != "built-prompt" || c.model != "built-model" || c.temp != 0.9 {
		t.Fatalf("without options the constructed values apply: %+v", c)
	}
}

func TestEmptyOptionFieldsKeepConstructedValues(t *testing.T) {
	c := &recordingClient{}
	tgt := NewLLMTarget(c, "built-model", "built-prompt", 0.9)
	ctx := WithCallOptions(context.Background(), CallOptions{Temperature: 0})
	if _, err := tgt.Call(ctx, "hi"); err != nil {
		t.Fatal(err)
	}
	if c.prompt != "built-prompt" || c.model != "built-model" {
		t.Fatalf("empty prompt and model must not blank the constructed ones: %+v", c)
	}
}
