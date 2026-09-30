package engine_test

import (
	"context"
	"testing"

	"github.com/xraph/sentinel/engine"
	"github.com/xraph/sentinel/scorer"
	"github.com/xraph/sentinel/target"
)

func echo(name string) target.Target {
	return target.FromFunc(name, func(_ context.Context, in string) (string, error) { return in, nil })
}

func TestTargetsAreNamed(t *testing.T) {
	e, _ := newEngine(t, engine.WithTarget("b", "second", echo("b")), engine.WithTarget("a", "first", echo("a")))
	got := e.Targets()
	if len(got) != 2 || got[0].Name != "a" || got[0].Description != "first" || got[1].Name != "b" {
		t.Fatalf("targets: %+v", got)
	}
	if rt, ok := e.Target("a"); !ok || rt.Target == nil {
		t.Fatal("target a not found")
	}
	if _, ok := e.Target("nope"); ok {
		t.Fatal("unknown target found")
	}
}

func TestDuplicateTargetIsRefused(t *testing.T) {
	if _, err := engine.New(engine.WithTarget("a", "", echo("a")), engine.WithTarget("a", "", echo("a"))); err == nil {
		t.Fatal("registering a target name twice must fail")
	}
	if _, err := engine.New(engine.WithTarget("", "", echo("a"))); err == nil {
		t.Fatal("a target needs a name")
	}
}

func TestScorersIncludeApplicationScorers(t *testing.T) {
	e, _ := newEngine(t, engine.WithScorer(scorer.Descriptor{Name: "judge", Description: "LLM judge", UsesLLM: true},
		func(map[string]any) (scorer.Scorer, error) { return okScorer("judge", 1, ""), nil }))
	if !e.Scorers().Has("judge") || !e.Scorers().Has("exact") {
		t.Fatal("the registry should hold built-ins and application scorers")
	}
}
