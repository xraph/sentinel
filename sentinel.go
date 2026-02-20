// Package sentinel provides a composable AI evaluation and testing framework
// for Go. It tests AI agents the way you'd evaluate a human — skills, traits,
// behaviors, cognition, communication, perception.
//
// Sentinel supports defining eval suites, running them across providers,
// detecting regressions, and integrating with CI. It is tenant-scoped,
// plugin-extensible, Forge-native, and Cortex-aware.
//
// # The Human-Like Testing Model
//
// Traditional AI eval: input → LLM → output → score.
//
// Sentinel's human-like eval: scenario → agent (persona) → observe skills,
// traits, behaviors, cognition, communication, perception → multi-dimensional
// score.
//
// # Evaluation Dimensions
//
//   - Skill: Can the agent do the job? Tool selection, proficiency, correctness.
//   - Trait: Who is the agent? Personality consistency across interactions.
//   - Behavior: How does it react? Trigger-action patterns fire correctly.
//   - Cognition: How does it think? Phase transitions, depth, focus.
//   - Communication: How does it talk? Tone, formality, verbosity.
//   - Perception: What does it notice? Attention focus, detail orientation.
//   - Persona: The whole person — end-to-end identity coherence.
package sentinel
