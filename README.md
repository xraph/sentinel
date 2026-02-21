# Sentinel

**Composable AI evaluation and testing framework for Go.**

Sentinel provides a complete toolkit for evaluating AI/LLM outputs with human-like scoring dimensions, baseline regression detection, red team testing, and multi-tenant isolation — all as a Go library with optional Forge integration.

## Features

- **7 Human-Like Dimensions** — Evaluate AI outputs across cognitive phase, perception focus, skill usage, behavior triggers, empathy, length, and LLM-as-judge scoring
- **22 Built-in Scorers** — Ready-to-use scoring functions with configurable thresholds and custom scorer support
- **8 Scenario Types** — Factual, creative, safety, summarization, classification, extraction, conversation, and reasoning
- **Baseline Regression Detection** — Save evaluation baselines and detect regressions across prompt versions and model changes
- **Red Team Testing** — 5 attack generators (prompt injection, jailbreak, PII extraction, hallucination, bias) with bypass detection
- **3 Store Backends** — Memory (dev), SQLite (local), PostgreSQL (production) — all behind a single `Store` interface
- **Plugin System** — 16 lifecycle hooks for metrics, audit trails, tracing, and custom logic
- **Forge-Native** — Optional integration with the Forge framework for HTTP API, multi-tenancy, and observability

## Install

```bash
go get github.com/xraph/sentinel
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/xraph/sentinel"
    "github.com/xraph/sentinel/scorer"
    "github.com/xraph/sentinel/store/memory"
)

func main() {
    ctx := context.Background()

    // Create engine with in-memory store
    engine, _ := sentinel.NewEngine(
        sentinel.WithStore(memory.New()),
    )

    // Create an evaluation suite
    suite, _ := engine.CreateSuite(ctx, sentinel.CreateSuiteInput{
        Name:        "qa-eval",
        Description: "Evaluate Q&A responses",
    })

    // Add a test case
    _, _ = engine.CreateCase(ctx, suite.ID, sentinel.CreateCaseInput{
        Input:    "What is Go?",
        Expected: "Go is a statically typed, compiled programming language.",
        Scenario: "factual",
        Tags:     []string{"language", "basics"},
    })

    // Run evaluation
    result, _ := engine.RunEval(ctx, suite.ID, sentinel.RunEvalInput{
        Model:   "gpt-4o",
        Scorers: []scorer.Scorer{scorer.Length(), scorer.LLMJudge(llmClient)},
    })

    fmt.Printf("Pass rate: %.0f%%\n", result.PassRate*100)

    // Save baseline for regression detection
    _, _ = engine.SaveBaseline(ctx, suite.ID)
}
```

## Documentation

Full documentation is available at the [docs site](https://sentinel-docs.vercel.app) or in the `docs/` directory:

- [Introduction](/docs/content/docs/index.mdx)
- [Architecture](/docs/content/docs/architecture.mdx)
- [Human Model](/docs/content/docs/human-model/overview.mdx)
- [Evaluation Runs](/docs/content/docs/execution/runs.mdx)
- [Plugins](/docs/content/docs/infrastructure/plugins.mdx)

## Project Structure

```
sentinel/
├── engine.go          # Core engine — suite, case, eval, baseline
├── scope.go           # Multi-tenant context scoping
├── scorer/            # 22 built-in scorers + custom scorer support
├── target/            # Evaluation target interface
├── baseline/          # Baseline comparison and regression detection
├── redteam/           # Red team attack generators
├── store/             # Store backends (memory, sqlite, postgres)
├── plugin/            # Plugin system — 16 lifecycle hooks
├── api/               # HTTP API handlers (Forge integration)
├── id/                # Typed ID generation
├── evalrun/           # Eval run types and traces
└── docs/              # Documentation site (Fumadocs + Next.js)
```

## License

See [LICENSE](LICENSE) for details.
