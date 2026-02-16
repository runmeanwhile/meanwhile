# Sales Call Notes: TechStart Inc

**Company:** TechStart Inc  
**Industry:** B2B SaaS (HR Tech)  
**Company Size:** 28 employees  
**Current Stage:** Trial (Day 4)  
**ACV Potential:** $4,800/year (Starter or Pro)

---

## Meeting Details

**Date:** January 17, 2026  
**Attendees:**  
- Alex Rivera, Head of Growth (TechStart)
- David Rodriguez, AE (FlowForge)

**Meeting Type:** Trial Check-in (inbound request)

---

## Company Background

TechStart is a Series A HR tech startup that helps companies manage employee onboarding. Ironically, they're struggling with their own internal automation.

**Current Situation:**
- 28 employees, growing fast (hiring 2-3/month)
- Raised $4M Series A in September 2025
- Marketing team of 3, no dedicated ops
- Using Zapier currently, hitting frustrations

**Current Stack:**
- HubSpot (CRM + Marketing)
- Intercom (Support)
- Slack (Internal comms)
- Notion (Wiki/docs)
- Stripe (Billing)
- Segment (Data)

---

## Why They're Evaluating FlowForge

### Zapier Pain Points
> "We're paying $300/month for Zapier and constantly hitting limits. Every time I want to do something slightly complex, I need three zaps chained together."

> "Zapier's UI feels like it's from 2015. I want something that feels modern."

> "Our most critical workflow — lead routing — breaks every few weeks. No one notices until a lead complains."

### What They Found Appealing
- Saw our "vs Zapier" comparison page (converted from there)
- Pricing looked more favorable at their usage level
- Liked that we have HubSpot-specific features

---

## Current Trial Status

**Day 4 Progress:**
- ✅ Connected HubSpot
- ✅ Connected Slack
- ❌ Attempted to set up lead routing — got stuck
- ❌ Haven't connected Segment yet

**Trial Blocker:**
> "I tried to recreate our lead routing workflow but I couldn't figure out how to do conditional logic based on lead score AND company size. In Zapier I use Paths but it's clunky. Is there a better way here?"

---

## Pain Points Deep Dive

### 1. Lead Routing Complexity
Current manual process:
1. Lead comes in from website
2. Check lead score (HubSpot)
3. Check company size (HubSpot)
4. If enterprise → route to AE team Slack channel
5. If SMB + high score → route to SDR with @mention
6. If SMB + low score → add to nurture sequence
7. Update HubSpot with routing decision

> "This sounds simple but in Zapier it's 4 different zaps and they constantly get out of sync."

**FlowForge Opportunity:** Single workflow with conditions. Showed Alex a mockup during call.

### 2. Customer Lifecycle Events
> "When a customer churns or upgrades in Stripe, we need to update HubSpot, notify the team in Slack, and log it in Notion. Right now someone does this manually and forgets half the time."

### 3. Content Syndication
> "We publish a blog post and want it to automatically go to LinkedIn, Twitter, and our Slack #content channel with custom formatting for each. Tried this in Zapier, gave up."

---

## Demo During Call

Spent 20 minutes building their lead routing workflow live:
- Created HubSpot form submission trigger
- Added conditions for lead score and company size
- Set up Slack messages with dynamic content
- Added HubSpot property update

**Alex's Reaction:**
> "Wait, that's it? That would have been 4 zaps and an hour of work. You did it in 20 minutes."

> "The condition builder makes so much more sense than Zapier Paths."

---

## Objections & Concerns

### Concern 1: Migration Effort
> "We have 23 Zaps running. Migrating sounds painful."

**Response:** 
- Offered migration guide
- Pointed out their critical workflows (5-6) are the priority
- Suggested running both for 2 weeks during transition

**Status:** Partially addressed. Still nervous about effort.

### Concern 2: Team Buy-in
> "It's not just me using this. I need to convince our marketing manager and CEO."

**Response:**
- Offered to do a team demo
- Shared case study of similar-sized startup
- Highlighted that templates make it easier for non-power-users

**Status:** Scheduled team demo for next week

### Concern 3: Reliability
> "Zapier breaks on us. How do I know you won't?"

**Response:**
- Shared uptime stats (99.9% last 12 months)
- Explained error notifications and monitoring
- Mentioned we're smaller but more focused on reliability

**Status:** Partially addressed. Wants to test more.

---

## Pricing Discussion

**Current Zapier Spend:** $299/month (Team plan)

**FlowForge Options Discussed:**
- Starter ($49/month): 2,000 tasks — probably not enough
- Pro ($149/month): 20,000 tasks — likely fits their needs

**Savings:** $150/month = $1,800/year

> Alex: "The pricing is definitely better. Even Pro is half what we pay Zapier."

---

## Competition

**Primary:** Zapier (incumbent)  
**Secondary:** Make — "We looked at it but it seemed too complex for our team"

**Why they might stay with Zapier:**
- Familiarity
- Migration effort
- Risk of unknown

**Why they'd switch to FlowForge:**
- Price (50% savings)
- Better conditional logic
- Modern UI
- Single workflow instead of multiple zaps

---

## Buying Process

**Decision Makers:**
- Alex Rivera (Head of Growth) — Champion, likely has budget authority
- CEO — Final approval on new tools (but Alex says he's hands-off)

**Timeline:**
- Trial ends: January 27
- Wants to decide by end of trial
- If yes, would migrate in February

**Budget:** Not a major concern. Already spending $300/month.

---

## Next Steps

| Action | Owner | Due Date |
|--------|-------|----------|
| Send Zapier migration guide | David | Jan 18 |
| Schedule team demo | David | Jan 22 |
| Alex to complete lead routing workflow | Alex | Jan 20 |
| Check in on trial progress | David | Jan 22 |
| Follow up for decision | David | Jan 28 |

---

## Forecast

**Stage:** Trial (Active)  
**Probability:** 50%  
**Expected Close:** January 2026  
**Expected ACV:** $1,788 (Pro annual, 10% discount)

**Risk Factors:**
- Migration effort might stall them
- If reliability issues during trial, could lose them
- CEO could push back (low probability)

**Positive Signals:**
- Strong pain with current solution
- Pricing works in our favor
- Alex is motivated and engaged

---

## Follow-up Notes

**Jan 22 Update:**  
Team demo went well. Marketing manager (Jamie) was impressed with template library. CEO didn't attend but Alex said he's supportive. Lead routing workflow is now live and working.

**Remaining Blocker:** They want to run FlowForge parallel to Zapier for a week before fully committing. Extended trial to Jan 31.

---

## Potential Expansion

If they convert:
- Segment integration (data warehouse sync)
- Intercom integration (support automation)
- As team grows, could move to Team/Enterprise plan
- Strong referral potential (Alex is active in SaaS founder communities)

**12-month ACV potential:** $5-8K
