# Consensus Protocol Example

This example demonstrates the **consensus protocol** where multiple agents reach agreement through structured round-robin discussion with explicit position signaling.

## What It Does

Three agents (Development, Operations, Security) discuss and reach consensus on whether to allow production releases on Fridays:
- **Round-robin discussion**: Agents take turns in order
- **Position signaling**: Each agent signals AGREE, CONDITIONAL, BLOCK, or ABSTAIN via tool
- **Scope enforcement**: Moderator keeps discussion at policy level, prevents implementation rabbit holes
- **Moderator facilitation**: Interventions at 50%, 80%, and 90% checkpoints
- **Structured output**: Complete consensus result with state, reasoning, positions, and full thread

## The Key Innovation: Scope Enforcement

The protocol **automatically refines vague user questions** into clear, bounded scopes:

**User asks:** "Should we allow Friday deploys?"

**Protocol clarifies:**
```
DECISION SCOPE:
What we're deciding: YES/NO policy decision on Friday releases
What we're NOT deciding: Implementation details, tooling, timelines, procedures

STAY HIGH-LEVEL. Agree on policy first. Implementation comes after.
```

**Moderator actively enforces scope:**
- Detects when agents dive into CSV formats, API specs, or delivery timelines
- Redirects: "@Ops - let's pause. Can you restate at the policy level?"
- Keeps discussion focused on WHETHER, not HOW

This prevents the common failure mode where consensus discussions spiral into implementation debates before agreeing on the core decision.

## The Scenario

"Should we allow production releases on Fridays?"

### The Agents

**Development** cares about:
- Shipping quickly when customers need fixes
- Not blocking the team with bureaucracy
- Keeping morale high (burned out teams ship worse code)
- Moving fast but not breaking things

**Operations** cares about:
- Not spending weekends fixing avoidable incidents
- Predictable changes (not surprises at 4:45pm Friday)
- Having enough coverage when deployments happen
- Sleep (been paged at 3am too many times)

**Security** cares about:
- Protecting customer data (that's the job)
- Making sure changes get reviewed (not rubber-stamped)
- Not being the team everyone works around
- Practical security that actually gets followed

Each agent has defined red lines but is pragmatic and solution-oriented.

## Running the Example

```bash
export OPENAI_API_KEY=your_key_here
cd examples/06-protocol-consensus
go run main.go
```

## Expected Output

The example produces a structured `consensus.Result`:

```
CONSENSUS OUTCOME
================================================================
State: conditional_agreement
Rounds Used: 5/8

QUICK REASONING
----------------------------------------------------------------
Conditional consensus achieved. All agents agree with the following conditions:
- Must have on-call rotation with at least 2 engineers
- No deployments after 2pm on Fridays
- Security-sensitive changes require explicit approval
- Rollback plan must be documented and tested

AGENT POSITIONS
----------------------------------------------------------------
Development: conditional
  Reasoning: I support Friday deploys with reasonable safeguards...
  Conditions:
    - Emergency hotfix process must remain fast
    - No bureaucratic overhead for minor changes

Operations: conditional
  Reasoning: Acceptable if we have proper coverage and timing limits...
  Conditions:
    - Must have on-call rotation with at least 2 engineers
    - No deployments after 2pm Friday
    - Rollback plan required

Security: conditional
  Reasoning: Can work if security review isn't bypassed...
  Conditions:
    - Security-sensitive changes need explicit approval
    - Audit trail for all Friday deploys
```

## Consensus States

- **full_agreement**: All agents agree without conditions
- **conditional_agreement**: Agreement with conditions that must be met
- **blocked**: One or more agents object
- **unresolved**: Time budget exhausted before consensus

## Configuration Options

```go
consensus.Consensus(
    consensus.WithMaxRounds(8),                      // Discussion budget
    consensus.WithScope("Decide whether..."),        // Clear scope
    consensus.WithModeratorInterventions(0.5, 0.8), // Intervention points
)
```

## Position Signals

Agents signal their position by calling the `signal_position` tool:

```json
{
  "position": "agree" | "conditional" | "block" | "abstain",
  "reasoning": "Explanation of position",
  "conditions": ["condition 1", "condition 2"]  // Only for conditional
}
```

The tool is automatically registered on the session, so all agents have access to it.

**Position Types:**
- `agree` - Full agreement
- `conditional` - Agreement with specific conditions (must provide conditions array)
- `block` - Cannot agree (explain in reasoning)
- `abstain` - Choosing not to take a position

## Moderator Role

The facilitator injects guidance at configured checkpoints:
- **50%**: "Focus on key differences..."
- **80%**: "Any unresolved blockers?"
- **90%**: "Signal your position now"

## API Structure

The `ConsensusResult` provides:
- `State`: Current consensus state
- `Reasoning`: Quick summary of outcome
- `Positions`: Array of each agent's position with reasoning
- `Conditions`: Aggregated conditions (if conditional agreement)
- `BlockingIssues`: List of blocking concerns (if blocked)
- `RoundsUsed`: Rounds consumed
- `MessageThread`: Complete discussion history

## Use Cases

- Policy decisions requiring stakeholder buy-in
- Technical approach selection among competing options
- Risk assessment requiring diverse viewpoints
- Standard definition across team boundaries
- Strategic direction-setting with multiple considerations
