package target

import (
	"context"
	"time"

	"github.com/xraph/sentinel/evalrun"
)

// AgentClient is the interface for invoking a Cortex agent and capturing
// the full run trace for persona-aware evaluation.
type AgentClient interface {
	Run(ctx context.Context, agentID, personaRef, input string) (*AgentResponse, error)
}

// AgentResponse holds the response from an agent invocation.
type AgentResponse struct {
	Output string
	Tokens int
	Cost   float64
	Trace  *evalrun.RunTrace
}

// AgentTarget invokes a Cortex agent and captures the full run trace.
// This enables persona-aware scoring by providing tool calls, steps, and
// execution metadata to the scorers.
type AgentTarget struct {
	AgentID    string
	PersonaRef string
	Client     AgentClient
}

// NewAgentTarget creates a target that invokes a Cortex agent.
func NewAgentTarget(client AgentClient, agentID, personaRef string) *AgentTarget {
	return &AgentTarget{
		Client:     client,
		AgentID:    agentID,
		PersonaRef: personaRef,
	}
}

func (t *AgentTarget) Name() string { return "agent:" + t.AgentID }

func (t *AgentTarget) Call(ctx context.Context, input string) (*TargetOutput, error) {
	start := time.Now()
	resp, err := t.Client.Run(ctx, t.AgentID, t.PersonaRef, input)
	elapsed := time.Since(start)
	if err != nil {
		return nil, err
	}
	return &TargetOutput{
		Output:  resp.Output,
		Latency: elapsed,
		Tokens:  resp.Tokens,
		Cost:    resp.Cost,
		Trace:   resp.Trace,
	}, nil
}
