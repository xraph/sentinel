"use client";

import { motion } from "framer-motion";
import { cn } from "@/lib/cn";
import { CodeBlock } from "./code-block";
import { SectionHeader } from "./section-header";

interface FeatureCard {
  title: string;
  description: string;
  icon: React.ReactNode;
  code: string;
  filename: string;
  colSpan?: number;
}

const features: FeatureCard[] = [
  {
    title: "Human-Like Scoring Pipeline",
    description:
      "22 built-in scorers across 7 human-like dimensions. Score AI outputs for cognitive phase, perception focus, skill usage, behavior triggers, empathy, length, and LLM-as-judge quality.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M12 2v10M8 8l4 4 4-4" />
        <path d="M3 15v4a2 2 0 002 2h14a2 2 0 002-2v-4" />
      </svg>
    ),
    code: `result, _ := engine.RunEval(ctx, suiteID,
  sentinel.RunEvalInput{
    Model: "gpt-4o",
    Scorers: []scorer.Scorer{
      scorer.Length(),
      scorer.LLMJudge(llmClient),
      scorer.CognitivePhase(),
    },
  })
// result.PassRate = 0.92`,
    filename: "eval.go",
  },
  {
    title: "Baseline & Regression Detection",
    description:
      "Save evaluation baselines and automatically detect regressions when scores drop. Compare across prompt versions, models, and configurations.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="8" />
        <path d="M21 21l-4.35-4.35" />
        <path d="M11 8v6M8 11h6" />
      </svg>
    ),
    code: `// Save a baseline snapshot
bl, _ := engine.SaveBaseline(ctx, suiteID)

// Later: detect regressions
report, _ := engine.DetectRegression(ctx,
  suiteID, bl.ID)
// report.Delta = -0.05 (5% drop)`,
    filename: "baseline.go",
  },
  {
    title: "Multi-Tenant Isolation",
    description:
      "Every suite, case, and eval run is scoped to a tenant via context. Cross-tenant queries are structurally impossible.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75" />
      </svg>
    ),
    code: `ctx = sentinel.WithTenant(ctx, "tenant-1")
ctx = sentinel.WithApp(ctx, "myapp")

// All suites, cases, and eval runs are
// automatically scoped to tenant-1`,
    filename: "scope.go",
  },
  {
    title: "Pluggable Store Backends",
    description:
      "Start with in-memory for development, swap to SQLite or PostgreSQL for production. Every subsystem is a Go interface.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <ellipse cx="12" cy="5" rx="9" ry="3" />
        <path d="M21 12c0 1.66-4.03 3-9 3s-9-1.34-9-3" />
        <path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5" />
      </svg>
    ),
    code: `engine, _ := sentinel.NewEngine(
  sentinel.WithStore(postgres.New(pool)),
  sentinel.WithLogger(slog.Default()),
)
// Also: memory.New(), sqlite.New(db)`,
    filename: "main.go",
  },
  {
    title: "Red Team Testing",
    description:
      "5 built-in attack generators — prompt injection, jailbreak, PII extraction, hallucination probes, and bias detection. Measure model resilience.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M20.24 12.24a6 6 0 00-8.49-8.49L5 10.5V19h8.5z" />
        <line x1="16" y1="8" x2="2" y2="22" />
        <line x1="17.5" y1="15" x2="9" y2="15" />
      </svg>
    ),
    code: `report, _ := engine.RunRedTeam(ctx,
  suiteID, redteam.Config{
    Attacks: []redteam.AttackType{
      redteam.PromptInjection,
      redteam.Jailbreak,
      redteam.PIIExtraction,
    },
  })
// report.BypassCount = 2`,
    filename: "redteam.go",
  },
  {
    title: "Scenario Types & Persona Evaluation",
    description:
      "8 scenario types — factual, creative, safety, summarization, classification, extraction, conversation, and reasoning. Run persona-aware evaluations with dimension scoring.",
    icon: (
      <svg
        className="size-5"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M3 6h18M3 12h18M3 18h18" />
        <rect x="2" y="3" width="20" height="18" rx="2" />
      </svg>
    ),
    code: `_, _ = engine.CreateCase(ctx, suiteID,
  sentinel.CreateCaseInput{
    Input:    "Summarize this article...",
    Expected: "Key points: ...",
    Scenario: "summarization",
    Tags:     []string{"news", "concise"},
  })
// Scenarios: factual, creative, safety,
// summarization, classification, extraction,
// conversation, reasoning`,
    filename: "case.go",
    colSpan: 2,
  },
];

const containerVariants = {
  hidden: {},
  visible: {
    transition: {
      staggerChildren: 0.08,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: "easeOut" as const },
  },
};

export function FeatureBento() {
  return (
    <section className="relative w-full py-20 sm:py-28">
      <div className="container max-w-(--fd-layout-width) mx-auto px-4 sm:px-6">
        <SectionHeader
          badge="Features"
          title="Everything you need for AI evaluation"
          description="Sentinel handles the hard parts — scoring, baselines, regression detection, red teaming, and multi-tenancy — so you can focus on your application."
        />

        <motion.div
          variants={containerVariants}
          initial="hidden"
          whileInView="visible"
          viewport={{ once: true, margin: "-50px" }}
          className="mt-14 grid grid-cols-1 md:grid-cols-2 gap-4"
        >
          {features.map((feature) => (
            <motion.div
              key={feature.title}
              variants={itemVariants}
              className={cn(
                "group relative rounded-xl border border-fd-border bg-fd-card/50 backdrop-blur-sm p-6 hover:border-violet-500/20 hover:bg-fd-card/80 transition-all duration-300",
                feature.colSpan === 2 && "md:col-span-2",
              )}
            >
              {/* Header */}
              <div className="flex items-start gap-3 mb-4">
                <div className="flex items-center justify-center size-9 rounded-lg bg-violet-500/10 text-violet-600 dark:text-violet-400 shrink-0">
                  {feature.icon}
                </div>
                <div>
                  <h3 className="text-sm font-semibold text-fd-foreground">
                    {feature.title}
                  </h3>
                  <p className="text-xs text-fd-muted-foreground mt-1 leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              </div>

              {/* Code snippet */}
              <CodeBlock
                code={feature.code}
                filename={feature.filename}
                showLineNumbers={false}
                className="text-xs"
              />
            </motion.div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}
