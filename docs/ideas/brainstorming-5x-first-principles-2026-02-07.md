# Brainstorming 5x Upgrade: First-Principles Findings (2026-02-07)

## Why this doc

This summarizes what we observed in current eval artifacts, then reframes brainstorming from first principles using patterns from high-performing design agencies/innovation labs (including IDEO), and proposes a concrete 5x upgrade path for Meanwhile.

## What our eval samples show right now

Reviewed artifacts:

- `artifacts/evals/brainstorming-persona-composition/20260207-095606/report.json`
- `artifacts/evals/brainstorming-persona-composition/20260207-094850/report.json`
- `artifacts/dogfood/brainstorm-dogfood-20260206-171939.json`
- `artifacts/dogfood/brainstorm-dogfood-20260206-172511.json`
- `artifacts/dogfood/brainstorm-dogfood-20260206-173034.json`
- `pkg/protocol/brainstorming.go`
- `pkg/protocol/brainstorming_prompts.go`

### Observed pattern

1. Flow quality is decent, but structurally rigid.

- Repeated 10-phase cadence: open -> warm-up -> solo ideation -> presentations -> discussion -> shortlist -> vote -> markdown report.
- In successful runs, turn counts are highly regular (often exactly 28 for 4-speaker product scenarios).

2. Strong formatting discipline, weaker true creative divergence.

- Conversations converge reliably to 3 finalists and a polished summary.
- But idea space is mostly incremental product-feature optimization, not reframing or breakthrough concept generation.

3. Over-optimization for conversational shape vs innovation output.

- Current metrics reward responsiveness/naturalness/convergence.
- They do not directly measure novelty spread, insight quality, concept portfolio diversity, or experimentability.

4. Reliability fragility still visible.

- Multiple runs fail due timeout/stream/judge deadline errors (e.g., moderator closing timeout, judge timeout, INTERNAL_ERROR in dogfood runs).

5. Protocol-level evidence of hard constraints that limit creative range.

- Fixed move cues (`build`, `pressure-test`, `workflow example`, `tradeoff`) rotate deterministically.
- Very short context windows (`recentThread(..., 6)`), reducing long-arc synthesis.
- Prompting explicitly pushes concise turns; good for readability, but can suppress deep generative exploration.

6. Signal leakage/noise still occurs.

- Dogfood transcript includes leaked internal tag text (`\u003cagent:...\u003e`) even though prompts instruct agents to ignore such tags.

## First principles: what elite design teams optimize for

If the goal is not "nice conversation" but "high-quality breakthrough concepts," the system should maximize:

1. Problem reframing before idea generation.
2. Diversity of cognitive inputs (disciplines, mental models, user vantage points).
3. Artifact-based thinking (sketches, concept cards, prototypes), not text-only discussion.
4. Fast evidence loops (prototype + user reaction), not pure speculative debate.
5. Portfolio thinking (multiple bets with explicit risk profiles), not single-thread convergence too early.
6. Decision quality with explicit criteria (desirability, feasibility, viability, learning velocity).

## How IDEO brainstorms (relevant takeaways)

IDEO methods emphasize that brainstorming is one component inside a broader human-centered cycle, not the whole process.

Key signals from IDEO sources:

1. Use brainstorming after framing prompts as HMW questions and include users/partners when possible.
   Source: https://www.designkit.org/methods/brainstorm

2. Follow explicit brainstorming rules:
   defer judgment, encourage wild ideas, build on others, stay on topic, one conversation at a time, be visual, go for quantity.
   Source: https://designthinking.ideo.com/blog/7-rules-for-brainstorming

3. Design thinking is iterative: notice -> empathize -> experiment -> refine.
   Source: https://designthinking.ideo.com/faq/what-is-design-thinking

4. Prototype early and rough; use making to learn quickly, then refine.
   Source: https://www.ideo.com/journal/4-principles-for-making-better-prototypes

Implication for Meanwhile: our current brainstorming protocol starts too late (at discussion) and stops too early (at summary), missing discovery and experiment phases that produce non-obvious ideas.

## What we are fundamentally lacking

1. No true "inspiration" phase.

- Missing: user observations, analog scans, tension mapping, contradiction harvesting.
- Result: ideas are bounded by the initial brief and panel priors.

2. No dedicated reframing mechanics.

- Missing: explicit problem decomposition and multiple HMW framings.
- Result: group converges on one framing prematurely.

3. No deliberate divergence engines.

- Missing: structured ideation techniques (brainwriting, forced analogies, anti-goal inversion, mashups, constraint flips).
- Result: strong conversation but narrow concept entropy.

4. No artifact layer.

- Missing: concept posters, journey sketches, storyboarded flows, fake-door copy, prototype scripts.
- Result: text-heavy outputs that are hard to test.

5. No evidence gate before convergence.

- Missing: requirement that each finalist include falsifiable assumptions, test design, and kill criteria.
- Result: polished recommendations with weak empirical grounding.

6. No portfolio strategy.

- Missing: balanced set of bets (safe/adjacent/bold) and option value reasoning.
- Result: likely local maxima and homogeneous shortlist.

7. No cumulative learning memory across sessions.

- Missing: searchable idea lineage, prior failed hypotheses, recurring insight clusters.
- Result: reinvention and repeated ideas over time.

8. Eval is quality-of-conversation heavy, innovation-outcome light.

- Missing metrics for novelty spread, insight depth, prototype test readiness, assumption clarity.

## 5x upgrade: a new brainstorming protocol model

### Proposed protocol arc (agency/lab style)

1. **Inspiration Intake (Diverge Inputs)**
- Pull evidence snippets, user signals, constraints, analog examples.
- Output: `Insight Pack` (top tensions, contradictions, blind spots).

2. **Reframe (Diverge Frames)**
- Generate 10-20 HMW variants from the same brief.
- Force framing diversity (operational, behavioral, emotional, economic, system-level).
- Output: `Frame Set` + selected framing portfolio.

3. **Idea Burst (Diverge Concepts)**
- Parallel rounds using different ideation operators per subgroup:
  - analogy transfer
  - inversion (worst possible idea -> flip)
  - resource constraint remix
  - adjacent-industry import
- Output: high-volume concept pool with tags.

4. **Synthesis (Converge Concepts)**
- Cluster concepts into themes; merge complements; identify 3-5 distinct bets.
- Output: `Concept Portfolio` (safe / adjacent / bold).

5. **Proto-Test Design (Evidence Gate)**
- Each concept must include:
  - core assumption
  - cheapest prototype
  - target user signal
  - success/fail threshold
  - time-to-learn
- Output: `Experiment Cards`.

6. **Decision + Commitment**
- Rank by learning-adjusted ROI, not preference votes alone.
- Output: one ship-now concept + one risky option + one hedge experiment.

### Why this is 5x better

- Better ideas: broader search space before convergence.
- Better decisions: explicit evidence plans, not rhetorical wins.
- Better speed: prototype-ready output for immediate execution.
- Better resilience: avoids overfitting to one framing.

## Concrete protocol/collab-kit additions

1. `collab/insightpack`: build evidence packets from brief + artifacts.
2. `collab/reframer`: generate and score diverse HMW frames.
3. `collab/ideationops`: plug-in divergence operators (analogy, inversion, remix).
4. `collab/conceptboard`: structured concept cards (problem, mechanism, value, risk).
5. `collab/evidencegate`: require experiment card before shortlist eligibility.
6. `collab/portfolio`: enforce bet diversity in finalists.
7. `collab/lineage`: persistent idea memory across sessions.

These belong in collab kit first, then composed into a new protocol variant (for example: `protocol.brainstorming_lab`).

## Evaluation upgrades required

Add outcome metrics beyond current conversation proxies:

1. `frame_diversity_score`
2. `concept_novelty_entropy`
3. `portfolio_balance_score` (safe/adjacent/bold distribution)
4. `assumption_clarity_score`
5. `prototype_readiness_score`
6. `learning_velocity_score` (time-to-first-test)
7. `idea_lineage_reuse_rate` (across sessions)

## Fast 2-week experiment plan

1. Build MVP of `reframer` + `evidencegate` only.
2. Run A/B against current protocol on existing datasets.
3. Keep same judge model, but add two extra judged dimensions:
- reframing quality
- experimentability
4. Success criteria:
- +0.4 on idea quality OR convergence quality while preserving completion rate
- >=80% finalists include falsifiable assumption + test plan
- no >15% latency increase.

## Bottom line

Current brainstorming is good at producing coherent discussion transcripts and polished summaries. It is still primitive relative to top design-agency/innovation-lab practice because it lacks upstream inspiration, reframing diversity, artifact-based thinking, and downstream evidence loops. The biggest unlock is to evolve from **conversation protocol** to **innovation pipeline protocol**.
