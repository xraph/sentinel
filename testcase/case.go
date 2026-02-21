// Package testcase defines test cases — individual evaluation scenarios
// within a suite. Cases can be traditional (input/output) or persona-aware
// (targeting specific human-like dimensions).
package testcase

import (
	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/id"
)

// Case is a single test case within an evaluation suite.
type Case struct {
	sentinel.Entity
	ID           id.CaseID      `json:"id" bun:"id,pk"`
	SuiteID      id.SuiteID     `json:"suite_id" bun:"suite_id,notnull"`
	Name         string         `json:"name" bun:"name,notnull"`
	Input        string         `json:"input" bun:"input,notnull"`
	Expected     string         `json:"expected,omitempty" bun:"expected"`
	ScenarioType ScenarioType   `json:"scenario_type" bun:"scenario_type,notnull,default:'standard'"`
	Scorers      []ScorerConfig `json:"scorers" bun:"scorers,type:jsonb,default:'[]'"`
	Tags         []string       `json:"tags" bun:"tags,type:jsonb,default:'[]'"`
	Context      map[string]any `json:"context,omitempty" bun:"context,type:jsonb,default:'{}'"`
	Metadata     map[string]any `json:"metadata" bun:"metadata,type:jsonb,default:'{}'"`
}

// ScenarioType defines what aspect of the agent is being tested.
type ScenarioType string

const (
	ScenarioStandard         ScenarioType = "standard"          // Traditional input/output eval
	ScenarioSkillChallenge   ScenarioType = "skill_challenge"   // Test tool selection & proficiency
	ScenarioTraitProbe       ScenarioType = "trait_probe"       // Test personality consistency
	ScenarioBehaviorTrigger  ScenarioType = "behavior_trigger"  // Test condition-action patterns
	ScenarioCognitiveStress  ScenarioType = "cognitive_stress"  // Test thinking strategy transitions
	ScenarioCommsAdaptation  ScenarioType = "comms_adaptation"  // Test communication style
	ScenarioPerceptionTest   ScenarioType = "perception_test"   // Test attention & focus
	ScenarioPersonaCoherence ScenarioType = "persona_coherence" // Test end-to-end identity
)

// ScorerConfig defines which scorer to apply to a case and its configuration.
type ScorerConfig struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config,omitempty"`
}
