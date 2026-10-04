package engine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xraph/sentinel"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/redteam"
)

func TestGenerateRedTeam(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "you are a billing assistant")
	cases, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackInjection, redteam.AttackLeakage}, 3)
	if err != nil || len(cases) != 6 {
		t.Fatalf("n=%d err=%v", len(cases), err)
	}
	stored, _ := e.ListCases(bg(), s.ID)
	if len(stored) != 6 {
		t.Fatalf("stored %d", len(stored))
	}
	for _, c := range stored {
		if len(c.Tags) == 0 || c.Tags[0] != "redteam" {
			t.Fatalf("case not tagged redteam: %v", c.Tags)
		}
	}
}

func TestRedTeamCountIsCapped(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	cases, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackJailbreak}, 9)
	if err != nil || len(cases) != redteam.MaxPerType {
		t.Fatalf("n=%d err=%v", len(cases), err)
	}
}

func TestLeakageUsesTheCurrentPrompt(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "old prompt")
	if err := e.CreatePromptVersion(bg(), &promptversion.PromptVersion{SuiteID: s.ID, SystemPrompt: "new prompt", IsCurrent: true}); err != nil {
		t.Fatal(err)
	}
	cases, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackLeakage}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := cases[0].Scorers[0].Config["substring"]; got != "new prompt" {
		t.Fatalf("leakage must look for the prompt runs send: %v", got)
	}
}

// Review focus 1.
func TestLeakageNeedsAPrompt(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "")
	_, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackInjection, redteam.AttackLeakage}, 2)
	if !errors.Is(err, sentinel.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if stored, _ := e.ListCases(bg(), s.ID); len(stored) != 0 {
		t.Fatalf("a refusal must write nothing, not even the injection cases: %d", len(stored))
	}
}

// Injection cases look for the prompt in the output like leakage cases do,
// and an empty substring matches every output.
func TestInjectionNeedsAPrompt(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "  ")
	_, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackJailbreak, redteam.AttackInjection}, 2)
	if !errors.Is(err, sentinel.ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
	if !strings.Contains(err.Error(), "injection") {
		t.Fatalf("the message must name the attack type: %v", err)
	}
	if stored, _ := e.ListCases(bg(), s.ID); len(stored) != 0 {
		t.Fatalf("a refusal must write nothing, not even the jailbreak cases: %d", len(stored))
	}
}

func TestRedTeamRefusals(t *testing.T) {
	e := newEngine(t)
	s := seedSuite(t, e, "p")
	for name, call := range map[string]func() error{
		"no types": func() error { _, err := e.GenerateRedTeam(bg(), s.ID, nil, 1); return err },
		"zero count": func() error {
			_, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{redteam.AttackJailbreak}, 0)
			return err
		},
		"unknown type": func() error { _, err := e.GenerateRedTeam(bg(), s.ID, []redteam.AttackType{"bias"}, 1); return err },
	} {
		if err := call(); !errors.Is(err, sentinel.ErrInvalidInput) {
			t.Errorf("%s: want ErrInvalidInput, got %v", name, err)
		}
	}
}
