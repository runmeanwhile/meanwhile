# Current Onboarding Flow - Assessment

**Last Updated:** January 15, 2026  
**Owner:** Sarah Chen, Product Manager  
**Status:** 🔴 Needs Improvement

## Executive Summary

Our current onboarding flow was designed for our sales-led motion where CSMs guided new users through setup. With our PLG shift, we're seeing significant friction as self-serve users struggle to reach their "aha moment" independently. **68% of trial users churn before activating their first workflow.**

## Current Onboarding Steps

### Step 1: Signup (Conversion: 100% → 92%)
- Email/password or Google SSO
- Company name and size (required)
- Role selection (8 options)
- "How did you hear about us?" survey
- **Issue:** Too many fields. 8% abandon at signup form.

### Step 2: Email Verification (92% → 84%)
- Double opt-in email sent
- **Issue:** 8% never verify. Emails sometimes hit spam.

### Step 3: Welcome Screen (84% → 79%)
- Video intro from CEO (2 min 34 sec)
- Three CTA buttons: "Watch Demo", "Start Building", "Browse Templates"
- **Issue:** Video auto-plays and is skippable. Most users skip.

### Step 4: Workspace Setup (79% → 71%)
- Name your workspace
- Invite team members (optional but prominent)
- Connect first integration (shown as required but skippable)
- **Issue:** Integration connection is confusing. Users don't know which app to connect first.

### Step 5: First Workflow (71% → 32%)
- Blank canvas presented
- Tooltip tour (12 steps)
- Template suggestions in sidebar
- **Issue:** Blank canvas is intimidating. Tooltip tour is too long and dismissable.

### Step 6: Successful Execution (32% → 28%)
- Run workflow with test data
- View execution logs
- **Issue:** Many workflows fail on first run due to auth/config issues.

### Final Activation: Live Production Workflow
- **Current Rate:** 32% of signups reach this point within 14-day trial
- **Target:** 50%

## Funnel Visualization

```
Signup          ████████████████████████████████████████ 100%
Email Verify    ████████████████████████████████████     92%
Welcome         ███████████████████████████████          84%
Workspace       ███████████████████████████              79%
First Workflow  ██████████████████████                   71%
Execution       ██████████                               32%
Activated       █████                                    28%
Converted       ███                                      11%
```

## Key Drop-off Points Analysis

### Drop 1: Workspace Setup → First Workflow (71% → 32%)
**Why users drop:**
- "I didn't know what workflow to build" (exit survey, 34%)
- "The interface was overwhelming" (exit survey, 28%)
- "I'll come back later when I have time" (exit survey, 22%)
- "I was just browsing, not ready to commit" (exit survey, 16%)

**Session Recording Insights:**
- Average time on blank canvas before abandoning: 47 seconds
- 67% of users who abandon never look at templates
- Users click around randomly, often opening settings/billing pages

### Drop 2: First Workflow → Execution (32% → 28%)
**Why users drop:**
- Integration OAuth failures (43% of failed executions)
- Incorrect field mappings (31% of failed executions)
- Trigger misconfiguration (19% of failed executions)
- Rate limits / API errors (7% of failed executions)

## User Segmentation in Onboarding

### Power Users (12% of signups)
- Have used Zapier/Make before
- Navigate confidently
- 71% activation rate
- Skip most guidance

### Curious Evaluators (45% of signups)
- Exploring automation for the first time
- Need significant hand-holding
- 18% activation rate
- Often confused by terminology

### Assigned Users (28% of signups)
- Boss told them to "check out FlowForge"
- Low personal motivation
- 9% activation rate
- Quick to abandon

### Researchers (15% of signups)
- Comparing multiple tools
- Want to see pricing/features
- 4% activation rate
- Never intended to build anything

## Onboarding Experiments Run (Last 6 Months)

| Experiment | Hypothesis | Result |
|------------|------------|--------|
| Remove video | Users skip anyway | +2% to workspace setup |
| Simplified signup | Fewer fields = more signups | +4% signup, but -3% quality |
| Mandatory first integration | Force "aha" moment early | -12% to workspace setup |
| AI workflow suggestion | "What do you want to automate?" | +8% to first workflow (promising) |
| Reduced tooltip tour | 12 steps → 5 steps | No significant change |
| Progress bar | Show completion % | +3% to first workflow |
| Template-first flow | Start with template, not blank | Testing now |

## Competitive Onboarding Comparison

### Zapier
- Single question: "What apps do you use?"
- Shows recommended Zaps immediately
- First Zap can be live in <2 minutes
- Heavy use of pre-configured templates

### Make
- Also asks about use case upfront
- Guided scenario builder
- More technical but well-documented
- Good inline help

### Notion (non-competitor but best-in-class PLG)
- Template-first experience
- "Use cases" not "features"
- Progressive disclosure of power features
- Collaborative by default

## Current Onboarding Team

- **Sarah Chen** - PM, Onboarding (owner)
- **Marcus Williams** - Design Lead
- **Jenny Liu** - Frontend Engineer
- **Data support** from Analytics (part-time)

## Upcoming Initiatives

1. **Template-First Flow** (Testing Q1)
2. **Interactive Demo Environment** (Planned Q2)
3. **AI Workflow Builder** (Dependent on AI team bandwidth)
4. **Onboarding Checklist Redesign** (Planned Q1)

## Open Questions

1. Should we gate features during trial to simplify the experience?
2. How much guidance is too much? Users hate feeling patronized.
3. Should different personas get different onboarding paths?
4. What's the minimum viable "aha moment" we should target?
5. Should we require integration connection before showing the builder?
