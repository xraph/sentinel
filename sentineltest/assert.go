// Package sentineltest provides test assertions for evaluation results.
// It lives in a separate package to avoid import cycles between the
// root sentinel package and the evalrun package.
package sentineltest

import (
	"fmt"
	"testing"

	"github.com/xraph/sentinel/evalrun"
)

// AssertCheck is a function that validates run result statistics.
// It returns an error message if the check fails, nil if it passes.
type AssertCheck func(stats *evalrun.ResultStats) error

// Assert runs all checks against the result stats and fails the test
// if any check fails.
func Assert(t *testing.T, stats *evalrun.ResultStats, checks ...AssertCheck) {
	t.Helper()
	for _, check := range checks {
		if err := check(stats); err != nil {
			t.Error(err)
		}
	}
}

// MinPassRate requires the pass rate to be at least the given value.
func MinPassRate(rate float64) AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		if stats.PassRate < rate {
			return fmt.Errorf("pass rate %.3f < minimum %.3f", stats.PassRate, rate)
		}
		return nil
	}
}

// MinAverageScore requires the average score to be at least the given value.
func MinAverageScore(score float64) AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		if stats.AvgScore < score {
			return fmt.Errorf("avg score %.3f < minimum %.3f", stats.AvgScore, score)
		}
		return nil
	}
}

// NoRegressions checks that there are no failed cases.
func NoRegressions() AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		if stats.Failed > 0 {
			return fmt.Errorf("%d cases failed (regression detected)", stats.Failed)
		}
		return nil
	}
}

// MaxLatency requires the average latency to be at most the given value in ms.
func MaxLatency(ms int) AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		if stats.AvgLatencyMs > ms {
			return fmt.Errorf("avg latency %dms > max %dms", stats.AvgLatencyMs, ms)
		}
		return nil
	}
}

// MaxCost requires the total cost to be at most the given value.
func MaxCost(cost float64) AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		if stats.TotalCost > cost {
			return fmt.Errorf("total cost $%.4f > max $%.4f", stats.TotalCost, cost)
		}
		return nil
	}
}

// SkillProficiency requires the skill dimension score to be at least the given value.
func SkillProficiency(minScore float64) AssertCheck {
	return dimensionCheck("skill", minScore)
}

// TraitConsistency requires the trait dimension score to be at least the given value.
func TraitConsistency(minScore float64) AssertCheck {
	return dimensionCheck("trait", minScore)
}

// BehaviorTriggerRate requires the behavior dimension score to be at least the given value.
func BehaviorTriggerRate(minScore float64) AssertCheck {
	return dimensionCheck("behavior", minScore)
}

// CognitivePhaseCompliance requires the cognition dimension score to be at least the given value.
func CognitivePhaseCompliance(minScore float64) AssertCheck {
	return dimensionCheck("cognition", minScore)
}

// CommunicationMatch requires the communication dimension score to be at least the given value.
func CommunicationMatch(minScore float64) AssertCheck {
	return dimensionCheck("communication", minScore)
}

// PersonaCoherence requires the persona dimension score to be at least the given value.
func PersonaCoherence(minScore float64) AssertCheck {
	return dimensionCheck("persona", minScore)
}

func dimensionCheck(dimension string, minScore float64) AssertCheck {
	return func(stats *evalrun.ResultStats) error {
		score, ok := stats.DimensionScores[dimension]
		if !ok {
			return fmt.Errorf("dimension %q not present in results", dimension)
		}
		if score < minScore {
			return fmt.Errorf("dimension %q score %.3f < minimum %.3f", dimension, score, minScore)
		}
		return nil
	}
}
