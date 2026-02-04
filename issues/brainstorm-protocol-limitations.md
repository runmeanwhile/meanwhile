# Issue: Brainstorm Protocol Doesn't Reflect Real Brainstorming

**Filed**: January 31, 2026
**Severity**: High — limits protocol usefulness
**Status**: Open

---

## Problem

The current brainstorm protocol appears to run agents in parallel without visibility into each other's responses. Each agent generates ideas independently, resulting in:

1. **No building on ideas** — Agents don't riff on or extend each other's contributions
2. **No debate** — Agents don't challenge or push back on weak ideas
3. **No convergence** — Session produces parallel lists, not synthesized insights
4. **Ignores user redirection** — When pushed to answer specific questions, agents continue generating feature lists

This doesn't reflect how real brainstorming sessions work, where:
- Participants hear each other's ideas
- Ideas spark new ideas ("yes, and...")
- Bad ideas get filtered through group reaction
- The group converges toward promising directions

## Evidence

In a product architecture brainstorm (Jan 31, 2026):
- User asked a specific strategic question 5 times
- Each time, agents responded with generic feature lists
- Agents never directly answered "what requires paid infrastructure?"
- Agents never disagreed with each other
- Final summary was generic, didn't address the core question

## Root Cause Hypothesis

The protocol likely:
1. Sends the same prompt to all agents in parallel
2. Collects responses without sharing them across agents
3. Has no "react to others" phase
4. Has no facilitator steering toward the user's actual question

## Expected Behavior

A brainstorm protocol should:
1. **Sequential or reactive turns** — Each agent sees prior contributions
2. **Build and challenge** — Agents explicitly build on or challenge prior ideas
3. **Facilitator guidance** — Moderator keeps discussion on topic, redirects when needed
4. **Convergence phase** — Move from divergent ideation to convergent synthesis
5. **Question responsiveness** — When user asks specific question, agents answer it

## Suggested Fix

Consider restructuring brainstorm as:

```
Round 1: Divergent (parallel ok)
- All agents generate initial ideas independently

Round 2: React (sequential, seeing others)
- Each agent sees Round 1 output
- Agents build on, combine, or challenge ideas
- "I like X because... but Y has a problem..."

Round 3: Converge (facilitated)
- Moderator synthesizes themes
- Asks agents to vote/rank
- Identifies top directions

Round 4: Deep dive (if needed)
- Focus on top 2-3 directions
- Agents develop them further
```

Alternatively, consider whether "Brainstorm" should be split into:
- **Ideation** — Divergent, parallel, quantity over quality
- **Workshop** — Interactive, building on each other, facilitated

## Workaround

For strategic questions requiring debate and convergence, use **Consensus** or **Adversarial** protocols instead of Brainstorm.

## Related

- See product architecture brainstorm session (Jan 31, 2026) for full transcript
- Consider whether Collab Kit needs a "reaction" or "build-on" primitive

---

**Next Steps**:
1. Review current brainstorm protocol implementation
2. Determine if this is a bug or design limitation
3. Prototype improved version with sequential/reactive phases
4. Test with same strategic question to compare results
