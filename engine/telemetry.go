package engine

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

// InstrumentationName is the tracer name on every span sentinel starts. It
// is the module path, so a trace backend shows which library a span came
// from.
const InstrumentationName = "github.com/xraph/sentinel"

// tracer looks the tracer up on every call instead of caching it, so spans
// follow whatever provider is global when the work starts. forge.New
// installs one; a test can swap in a recorder.
func tracer() trace.Tracer { return otel.Tracer(InstrumentationName) }

// Run modes, for the sentinel.run.mode span attribute and the mode label.
const (
	modeAsync = "async" // StartRun: evaluated on the engine's own context
	modeSync  = "sync"  // RunEval: evaluated on the caller's context
)

// Call results, for the result label and span attributes.
const (
	resultOK        = "ok"
	resultError     = "error"
	resultCancelled = "cancelled"
)

// callResult classifies a target or scorer call. A call that failed because
// the run's context ended (shutdown, or a RunEval caller giving up) is
// cancelled, not an error: the provider did nothing wrong.
func callResult(ctx context.Context, err error) string {
	switch {
	case err == nil:
		return resultOK
	case ctx.Err() != nil:
		return resultCancelled
	default:
		return resultError
	}
}

// Target kinds. A target the application wrote itself is custom; its Name
// can hold a model or an agent ID, so it never becomes a label.
const (
	targetLLM    = "llm"
	targetAgent  = "agent"
	targetFunc   = "func"
	targetCustom = "custom"
)

func targetKind(t target.Target) string {
	switch t.(type) {
	case *target.LLMTarget:
		return targetLLM
	case *target.AgentTarget:
		return targetAgent
	case *target.FuncTarget:
		return targetFunc
	default:
		return targetCustom
	}
}

// scorerCustom is the label for every scorer that is not one of sentinel's
// own types. Names of scorers an application registers, or a case names,
// are free text and never reach a label or a span.
const scorerCustom = "custom"

// scorerKind names a scorer by its Go type, so the value is always one of
// the built-ins below or custom. It deliberately ignores Name(): a
// scorer.FromFunc can be called anything.
func scorerKind(s scorer.Scorer) string {
	switch s.(type) {
	case *scorer.ExactScorer:
		return "exact"
	case *scorer.ContainsScorer:
		return "contains"
	case *scorer.NotContainsScorer:
		return "not_contains"
	case *scorer.RegexScorer:
		return "regex"
	case *scorer.JSONValidScorer:
		return "json_valid"
	case *scorer.JSONSchemaScorer:
		return "json_schema"
	case *scorer.LengthScorer:
		return "length"
	case *scorer.LatencyScorer:
		return "latency"
	case *scorer.CostScorer:
		return "cost"
	case *scorer.LLMJudgeScorer:
		return "llm_judge"
	case *scorer.FactualScorer:
		return "factual"
	case *scorer.HallucinationScorer:
		return "hallucination"
	case *scorer.SemanticScorer:
		return "semantic"
	case *scorer.SkillUsageScorer:
		return "skill_usage"
	case *scorer.TraitConsistencyScorer:
		return "trait_consistency"
	case *scorer.BehaviorTriggerScorer:
		return "behavior_trigger"
	case *scorer.CognitivePhaseScorer:
		return "cognitive_phase"
	case *scorer.CommunicationStyleScorer:
		return "communication_style"
	case *scorer.PerceptionFocusScorer:
		return "perception_focus"
	case *scorer.PersonaCoherenceScorer:
		return "persona_coherence"
	default:
		return scorerCustom
	}
}

// markError sets an error status with a short, fixed description and adds
// an exception event that carries only the error's type. The message never
// goes on the span: a model provider's or a driver's error text can quote
// the prompt, the output or a connection string.
func markError(span trace.Span, description string, err error) {
	span.SetStatus(codes.Error, description)
	span.AddEvent("exception", trace.WithAttributes(attribute.String("exception.type", errorType(err))))
}

// errorType names an error for exception.type: the context sentinels by
// name, anything else by the Go type of the innermost error in its chain.
func errorType(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context.Canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context.DeadlineExceeded"
	}
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return fmt.Sprintf("%T", err)
		}
		err = next
	}
}
