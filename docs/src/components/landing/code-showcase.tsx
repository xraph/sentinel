"use client";

import { motion } from "framer-motion";
import { CodeBlock } from "./code-block";
import { SectionHeader } from "./section-header";

const setupCode = `package main

import (
  "context"
  "log/slog"

  "github.com/xraph/sentinel"
  "github.com/xraph/sentinel/scorer"
  "github.com/xraph/sentinel/store/memory"
)

func main() {
  ctx := context.Background()

  engine, _ := sentinel.NewEngine(
    sentinel.WithStore(memory.New()),
    sentinel.WithLogger(slog.Default()),
  )

  ctx = sentinel.WithTenant(ctx, "tenant-1")
  ctx = sentinel.WithApp(ctx, "myapp")

  // Create a suite with test cases
  suite, _ := engine.CreateSuite(ctx,
    sentinel.CreateSuiteInput{
      Name: "qa-eval",
    })

  _, _ = engine.CreateCase(ctx, suite.ID,
    sentinel.CreateCaseInput{
      Input:    "What is Go?",
      Expected: "A compiled language.",
      Scenario: "factual",
    })
}`;

const evalCode = `package main

import (
  "context"
  "fmt"

  "github.com/xraph/sentinel"
  "github.com/xraph/sentinel/scorer"
)

func runEval(
  engine *sentinel.Engine,
  ctx context.Context,
  suiteID string,
) {
  ctx = sentinel.WithTenant(ctx, "tenant-1")

  // Run evaluation with multiple scorers
  result, _ := engine.RunEval(ctx, suiteID,
    sentinel.RunEvalInput{
      Model: "gpt-4o",
      Scorers: []scorer.Scorer{
        scorer.Length(),
        scorer.LLMJudge(llmClient),
      },
    })

  fmt.Printf("Pass rate: %.0f%%\\n",
    result.PassRate*100)
  // Pass rate: 92%

  // Save baseline for regression detection
  _, _ = engine.SaveBaseline(ctx, suiteID)
}`;

export function CodeShowcase() {
  return (
    <section className="relative w-full py-20 sm:py-28">
      <div className="container max-w-(--fd-layout-width) mx-auto px-4 sm:px-6">
        <SectionHeader
          badge="Developer Experience"
          title="Simple API. Powerful evaluation."
          description="Create an evaluation suite and score AI outputs in under 20 lines. Sentinel handles the rest."
        />

        <div className="mt-14 grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Setup side */}
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.1 }}
          >
            <div className="mb-3 flex items-center gap-2">
              <div className="size-2 rounded-full bg-violet-500" />
              <span className="text-xs font-medium text-fd-muted-foreground uppercase tracking-wider">
                Setup &amp; Run
              </span>
            </div>
            <CodeBlock code={setupCode} filename="main.go" />
          </motion.div>

          {/* Eval side */}
          <motion.div
            initial={{ opacity: 0, x: 20 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.2 }}
          >
            <div className="mb-3 flex items-center gap-2">
              <div className="size-2 rounded-full bg-green-500" />
              <span className="text-xs font-medium text-fd-muted-foreground uppercase tracking-wider">
                Score &amp; Verify
              </span>
            </div>
            <CodeBlock code={evalCode} filename="eval.go" />
          </motion.div>
        </div>
      </div>
    </section>
  );
}
