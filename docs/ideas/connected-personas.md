# Connected Personas: Bridging Simulation and Reality

## Vision

Personas in Meanwhile can be **connected** to the real humans they represent. When agents debate using these personas and hit genuine uncertainty—information gaps, conflicting signals, high-stakes ambiguity—they can reach out to the actual person with a specific, targeted question.

The human responds asynchronously. The response flows back into the session. The debate continues with ground truth instead of assumptions.

**The result**: Pre-meeting preparation that doesn't just simulate your team—it actually consults them, surgically, without requiring everyone to be in a room.

---

## The Problem This Solves

### Current State: Simulation Without Grounding

When you run a team debate with modeled personas:
- Personas are based on documents, transcripts, and traits
- But documents are incomplete—people have opinions they've never written down
- High-confidence simulation can be confidently wrong
- You prepare for a meeting based on assumptions that might be false

### Desired State: Simulation With Selective Grounding

When agents hit genuine uncertainty:
- They recognize the gap ("We're assuming Anna prefers A over B, but we don't see that clearly")
- They propose a targeted question ("Should I ask Anna directly?")
- They reach out via Slack/email with a specific ask
- The real human responds when convenient
- The response injects into the session with attribution
- The debate continues with real data

**Key insight**: This isn't about replacing human judgment. It's about making human attention surgical. Instead of Anna sitting through a 60-minute meeting to contribute 3 key insights, she answers 3 targeted questions in 5 minutes.

---

## How It Works

### 1. Connecting a Persona

```bash
# Create persona as before
meanwhile persona create "Anna Chen" \
  --slack-export ./anna-slack.json \
  --docs ./anna-rfcs/*.md \
  --role "Head of Engineering"

# Connect to real human
meanwhile persona connect "Anna Chen" \
  --slack-user "@anna.chen" \
  --email "anna@company.com"
```

Connection requires mutual consent:
```
Meanwhile: Anna, Darko wants to connect your persona for team simulations.
           When the AI version of you is uncertain, it may ask you directly.
           You control when and how you're contacted.

           [Accept & Configure] [Decline]
```

### 2. Running a Session with Connected Personas

```bash
meanwhile session "Q2 Roadmap Review" \
  --team anna,marcus,chen \
  --protocol consensus \
  --allow-human-escalation \
  --question "Should we prioritize enterprise dashboard or mobile app?"
```

The session runs normally—agents debate, use tools, build toward consensus.

### 3. Escalation Triggers

The agent recognizes uncertainty and proposes escalation:

```
┌─ Live Deliberation ──────────────────────────────────────────┐
│ [Anna - Engineering] I think the dashboard is higher risk,   │
│ but I'm not certain about the timeline. We'd need to check   │
│ with the mobile team on their capacity.                      │
│                                                              │
│ [Marcus - Product] The mobile delay concerns me. What's      │
│ Anna's actual appetite for timeline risk here?               │
│                                                              │
│ [Chair] I'm seeing uncertainty about Anna's position on      │
│ timeline risk. Our data shows she's pushed back on tight     │
│ deadlines before, but this quarter she's mentioned wanting   │
│ to "move faster on strategic bets."                          │
│                                                              │
│ ⚡ ESCALATION PROPOSED                                        │
│ Anna (real) is connected. Should I ask her directly?         │
│                                                              │
│ Proposed question:                                           │
│ "For Q2, would you rather ship a scoped-down dashboard on    │
│  time, or take more time for the full version? Quick gut     │
│  check—Marcus and I are debating priorities."                │
│                                                              │
│ [Y] Send  [N] Skip  [E] Edit question                        │
└──────────────────────────────────────────────────────────────┘
```

### 4. Human Receives Targeted Question

Anna gets a Slack DM:

```
Meanwhile Bot (@meanwhile)

Hey Anna! Quick question from a planning session Darko is running:

"For Q2, would you rather ship a scoped-down dashboard on time,
or take more time for the full version? Quick gut check—Marcus
and I are debating priorities."

Context: Darko is preparing for tomorrow's roadmap review using
team simulation. Your persona is debating with Marcus and Chen.

[Reply here or ignore—session will continue with best guess]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
⚙️ Manage your Meanwhile preferences: /meanwhile settings
```

### 5. Response Flows Back

Anna replies: "Scoped-down, definitely. I'd rather ship something solid than slip again. But make sure we're not cutting the admin panel—that's what enterprise buyers actually need."

The session receives the response:

```
┌─ Live Deliberation ──────────────────────────────────────────┐
│ ✉️ RESPONSE FROM ANNA (REAL)                                  │
│ "Scoped-down, definitely. I'd rather ship something solid    │
│ than slip again. But make sure we're not cutting the admin   │
│ panel—that's what enterprise buyers actually need."          │
│                                                              │
│ [Anna - Engineering] OK, so with that clarity—I'm firmly     │
│ in favor of dashboard, scoped to core + admin panel. We      │
│ can add the analytics views in a fast-follow.                │
│                                                              │
│ [Marcus - Product] That actually resolves my concern. If     │
│ we protect the admin panel, enterprise buyers get value.     │
│ I can live with deferring analytics.                         │
│                                                              │
│ [Chair] Consensus check: Dashboard (scoped), protecting      │
│ admin panel, analytics in fast-follow. All aligned?          │
└──────────────────────────────────────────────────────────────┘
```

### 6. Session Completes with Real Input

Final output includes attribution:

```markdown
## Session Summary: Q2 Roadmap Review

**Consensus**: Strong (3-0 after human input)

**Decision**: Prioritize enterprise dashboard, scoped to core + admin panel.
Analytics views deferred to fast-follow.

**Key Input**:
- Anna (real, via Slack): Confirmed preference for scoped-down approach.
  Flagged admin panel as non-negotiable for enterprise buyers.

**Preparation Notes for Tomorrow**:
- Anna is aligned on scope-down—don't relitigate
- Protect admin panel in any scoping discussion
- Analytics deferral is accepted; don't oversell fast-follow timeline
```

---

## Escalation Intelligence

### When Should Agents Escalate?

Not every uncertainty should trigger a human ping. Agents should escalate when:

| Trigger | Example |
|---------|---------|
| **Missing critical information** | "We don't know Anna's budget authority for this quarter" |
| **Conflicting signals in data** | "Anna's RFC says X, but her Slack messages suggest Y" |
| **High-stakes ambiguity** | "This decision blocks three other workstreams" |
| **Explicit protocol config** | "Always verify technical feasibility with Anna" |
| **Low simulation confidence** | Model uncertainty score below threshold |
| **Time-sensitive context** | "Her opinion may have changed since last data" |

### When Should Agents NOT Escalate?

| Scenario | Behavior |
|----------|----------|
| **Stylistic preference** | Use best guess from data |
| **Already answered recently** | Cache responses for session duration |
| **Human is unavailable** | Continue with simulation, note uncertainty |
| **Question is too broad** | Refine internally before asking |
| **Rate limit exceeded** | Queue for later or skip |

### Escalation Quality

The agent should formulate good questions:

**Bad**: "What do you think about the roadmap?"
**Good**: "For Q2, dashboard vs mobile—which would you prioritize if we can only do one well?"

**Bad**: "Are you concerned about the timeline?"
**Good**: "If dashboard slips to April, is that acceptable or a dealbreaker?"

The system can include question quality checks:
- Is it specific enough to answer in 1-2 sentences?
- Does it have clear options or a concrete ask?
- Is it something the persona genuinely can't answer from data?

---

## Human Controls

### Availability Configuration

Real humans control when and how they're contacted:

```bash
# Anna configures her preferences
/meanwhile settings

┌─ Meanwhile Preferences ───────────────────────────────────────┐
│                                                              │
│ Availability                                                 │
│ ├─ Slack: Mon-Fri 9am-6pm PST                               │
│ ├─ Email: Anytime (batch digest)                            │
│ └─ Urgent (SMS): Never                                       │
│                                                              │
│ Question Filters                                             │
│ ├─ ✓ Technical decisions                                     │
│ ├─ ✓ Timeline/capacity questions                            │
│ ├─ ✗ Budget/financial (route to my manager)                 │
│ └─ ✓ Team dynamics                                          │
│                                                              │
│ Rate Limits                                                  │
│ ├─ Max questions per day: 5                                  │
│ ├─ Max per session: 2                                        │
│ └─ Cooldown between questions: 30 min                        │
│                                                              │
│ Approval Mode                                                │
│ ├─ ○ Auto-send (within filters)                             │
│ └─ ● Queue for my approval                                   │
│                                                              │
│ [Save] [Preview what I'd receive]                            │
└──────────────────────────────────────────────────────────────┘
```

### Approval Queue (Optional)

For sensitive contexts, humans can review before responding:

```
Meanwhile: New question queued for your approval

Session: "Q2 Roadmap Review" (started by Darko)
Participants: Anna (you), Marcus, Chen

Question: "For Q2, would you rather ship a scoped-down
dashboard on time, or take more time for the full version?"

[Approve & Answer] [Decline] [Block this session]
```

### Response Templates

For common questions, humans can set templates:

```
/meanwhile templates

"Timeline risk tolerance" →
  "I prefer shipping something solid over slipping. Scope down if needed."

"Budget authority" →
  "Anything under $50k I can approve. Over that, loop in Marcus."
```

When a question matches a template pattern, the agent can use it without pinging (or ping for confirmation).

---

## Session Modes

### Mode 1: Simulation Only (Default)

Personas debate using modeled knowledge. No human contact. Fast, fully async.

```bash
meanwhile session "..." --team anna,marcus --mode simulation
```

### Mode 2: Human-Assisted

Personas can escalate to connected humans when uncertain. Session may pause waiting for responses.

```bash
meanwhile session "..." --team anna,marcus --mode assisted
```

### Mode 3: Human-Required

Critical questions MUST get human confirmation before proceeding. For high-stakes decisions.

```bash
meanwhile session "..." --team anna,marcus --mode verified \
  --require-human "anna" \
  --require-topic "budget,timeline"
```

### Mode 4: Live Collaboration

Real humans join the session directly (from human-participation.md). Agents and humans in the same deliberation.

```bash
meanwhile session "..." --team anna,marcus --mode live \
  --human-participants "anna,darko"
```

---

## Persona Learning

When real humans provide input, the persona can improve:

### Explicit Feedback

After a session, the real human can review their persona's performance:

```
Meanwhile: Session complete! Your persona debated for 12 turns.

How did AI-Anna do?
├─ Turn 3: "I'd push back on the timeline" — [👍 Accurate] [👎 Not me]
├─ Turn 7: "Let's check mobile capacity" — [👍 Accurate] [👎 Not me]
└─ Turn 11: "I'm firmly in favor of dashboard" — [👍 Accurate] [👎 Not me]

Any corrections?
[Add feedback] [Looks good] [Skip]
```

### Implicit Learning

Responses to escalation questions become training data:

```
New data point for Anna persona:
Q: "Scoped-down vs full version?"
A: "Scoped-down, definitely. Ship something solid."
Context: Q2 planning, dashboard decision

[Auto-incorporated into persona model]
```

### Drift Detection

If real human responses consistently diverge from persona predictions:

```
⚠️ Persona drift detected for Anna

Recent responses suggest:
- Higher risk tolerance than modeled
- Less concerned about technical debt than before
- New priority: "enterprise buyers"

Suggested persona update:
- Increase risk_tolerance: 0.4 → 0.6
- Add trait: "focused on enterprise value"

[Apply update] [Review details] [Dismiss]
```

---

## Privacy & Security

### Data Handling

| Data Type | Storage | Access |
|-----------|---------|--------|
| Persona models | Local (`~/.meanwhile/`) | Owner only |
| Escalation questions | Encrypted in transit | Session participants |
| Human responses | Session transcript | Session participants |
| Learning updates | Local persona file | Owner only |

### Consent Requirements

| Action | Consent Needed |
|--------|---------------|
| Create persona from public data | None |
| Create persona from private data | Data owner |
| Connect persona to real human | Explicit mutual consent |
| Send escalation question | Within configured preferences |
| Use response for learning | Opt-in (default off) |

### Audit Trail

For enterprise/compliance:

```bash
meanwhile audit --persona "Anna Chen" --last 30d

Escalations sent: 12
Responses received: 9
Learning updates: 3
Sessions participated: 7
Preference changes: 1 (rate limit increased)
```

---

## Integration Points

### Slack (Primary)

- DM for questions
- Thread replies for context
- Slash commands for preferences
- App home for dashboard

### Email

- Digest mode (batch questions)
- Inline reply to respond
- Good for async/low-urgency

### Discord

- Similar to Slack
- Good for communities/open teams

### Microsoft Teams

- Enterprise integration
- Compliance/audit support

### Custom Webhooks

```bash
meanwhile persona connect "Anna Chen" \
  --webhook "https://company.com/meanwhile/anna" \
  --webhook-secret $SECRET
```

For custom integrations (internal tools, mobile apps, etc.)

---

## Example Flows

### Flow 1: Pre-Meeting Preparation

**Scenario**: You have a roadmap review tomorrow with Anna, Marcus, and Chen.

**Tonight**:
```bash
meanwhile session "Tomorrow's roadmap review" \
  --team anna,marcus,chen \
  --protocol consensus \
  --mode assisted \
  --question "What's our Q2 priority: dashboard or mobile?"
```

**What happens**:
1. Agents debate for 10 minutes
2. Marcus-persona raises concern about Anna's timeline risk appetite
3. Agent escalates to real Anna via Slack
4. Anna responds in 2 minutes while watching TV
5. Debate continues with real input
6. Session concludes with recommendation + preparation notes

**Tomorrow**:
- You walk in knowing Anna's actual position
- You know Marcus's concerns and how to address them
- You have talking points that reflect real input
- Meeting is 30 minutes instead of 90

### Flow 2: Async Decision Making

**Scenario**: Need quick alignment on a technical decision, but team is distributed across timezones.

```bash
meanwhile session "Redis vs Postgres for session storage" \
  --team anna,bob,chen \
  --protocol adversarial \
  --mode assisted \
  --deadline "6 hours"
```

**What happens**:
1. Agents debate the technical trade-offs
2. Bob-persona is uncertain about ops capacity
3. Escalation to real Bob (he's awake in London)
4. Bob responds: "We can handle Redis, but only managed service"
5. Chen-persona is uncertain about data model fit
6. Escalation to real Chen (she's in Singapore, asleep)
7. Session continues, notes Chen's pending input
8. 4 hours later, Chen wakes up, responds via Slack
9. Session resumes, incorporates response
10. Final recommendation delivered to you before deadline

**Result**: Decision made with real input from 3 people across 3 timezones, without a single meeting.

### Flow 3: Stakeholder Simulation with Reality Check

**Scenario**: Preparing for board meeting. You've modeled board members from public talks/writings.

```bash
meanwhile session "Board pitch: Series B terms" \
  --team investor-a,investor-b,advisor \
  --protocol adversarial \
  --mode simulation  # No escalation—these aren't connected
  --question "Will they push back on our valuation?"
```

**What happens**:
1. Modeled investors debate your pitch
2. Investor-A-persona raises concern about burn rate
3. You realize you need to prepare that slide
4. Advisor-persona suggests comparable companies

**Result**: You've stress-tested your pitch. You can't escalate to real board members, but simulation reveals blind spots.

---

## Open Questions

1. **Response timeout handling**: If human doesn't respond in X hours, should session proceed with simulation? Retry? Fail?

2. **Partial responses**: What if human says "I don't know" or "ask someone else"? How does that flow back?

3. **Multi-human escalation**: Can an agent ask multiple connected personas the same question simultaneously? How are conflicting responses handled?

4. **Escalation chains**: Can one human's response trigger escalation to another? ("Anna says ask Marcus about budget")

5. **Real-time presence**: Should the system know if a human is "online" and prefer them for escalation?

6. **Escalation explanation**: Should humans see WHY the agent is asking? ("I'm asking because your RFC and Slack messages seem to conflict on this point")

7. **Group questions**: Can an agent ask a question to multiple humans at once and synthesize responses?

---

## Success Metrics

1. **Escalation precision**: >80% of escalations result in useful responses (not "I don't know" or ignored)

2. **Human burden**: <5 questions per person per day on average

3. **Session value**: Users report sessions with escalation are more actionable than simulation-only

4. **Response time**: Median response time <10 minutes during availability windows

5. **Persona accuracy**: After 10 escalations, persona predictions match human responses >70% of time

6. **Meeting reduction**: Users report fewer/shorter meetings after adopting connected personas

---

## Implementation Phases

### Phase 1: Basic Escalation (MVP)

- Connect persona to Slack user
- Manual escalation trigger (user approves each question)
- Response injection into session
- Basic availability settings (on/off)

### Phase 2: Smart Escalation

- Automatic escalation detection
- Question quality scoring
- Rate limiting and preferences
- Response caching

### Phase 3: Learning Loop

- Persona updates from responses
- Drift detection
- Feedback collection
- Template responses

### Phase 4: Enterprise

- Audit trails
- Compliance controls
- SSO integration
- Admin dashboard

---

## Relationship to Other Features

| Feature | Relationship |
|---------|-------------|
| **Team Modeling** | Connected personas are an extension—same modeling, plus real human link |
| **Human Participation** | Different: participation = human in session; escalation = human consulted |
| **Meanwhile Board** | Could offer "verified" tier where advisory personas are connected to real advisors |
| **Slack Bot** | Natural integration point for escalation delivery |

---

## The Big Picture

This feature transforms Meanwhile from "AI debates" to "AI-augmented team coordination."

The insight: **Human attention is precious. AI can do 90% of the deliberation work. Humans provide the 10% that requires ground truth.**

Instead of:
- 60-minute meeting where Anna contributes 3 key points

You get:
- 10-minute simulation that identifies the 3 questions
- 3 Slack messages that take Anna 5 minutes total
- Better outcome because questions were targeted

**Meanwhile becomes the layer that makes human collaboration efficient, not the thing that replaces it.**

---

**Status**: Proposed
**Dependencies**: Team Modeling, Human Participation (partial)
**Target**: Q3 2026
**Complexity**: High (multi-system integration)
