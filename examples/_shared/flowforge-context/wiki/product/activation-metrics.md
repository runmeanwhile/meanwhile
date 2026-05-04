# Activation Metrics Deep Dive

**Last Updated:** January 22, 2026  
**Owner:** Analytics Team  
**Data Period:** August 2025 - January 2026 (Post-PLG Launch)

## Activation Definition

A user is considered **activated** when they have:
1. Created at least one workflow
2. Successfully executed it with real (non-test) data
3. The workflow ran at least 3 times

This typically takes 4.2 days from signup (median).

## Overall Activation Funnel (6-Month Average)

| Stage | Users | Rate | Δ from Signup |
|-------|-------|------|---------------|
| Trial Signups | 11,847 | 100% | - |
| Email Verified | 10,899 | 92.0% | -8.0% |
| First Login (Day 1) | 9,781 | 82.6% | -17.4% |
| Integration Connected | 5,869 | 49.5% | -50.5% |
| First Workflow Created | 4,147 | 35.0% | -65.0% |
| First Successful Execution | 3,317 | 28.0% | -72.0% |
| **Activated (3+ runs)** | **1,896** | **16.0%** | **-84.0%** |
| Converted to Paid | 1,303 | 11.0% | -89.0% |

## Activation by Cohort (Month-over-Month)

| Month | Signups | Activated | Rate | Notes |
|-------|---------|-----------|------|-------|
| Aug 2025 | 1,823 | 247 | 13.5% | PLG launch month |
| Sep 2025 | 1,956 | 284 | 14.5% | Initial optimizations |
| Oct 2025 | 2,104 | 315 | 15.0% | AI suggestions launched |
| Nov 2025 | 1,891 | 298 | 15.8% | Holiday slowdown |
| Dec 2025 | 1,687 | 287 | 17.0% | Template improvements |
| Jan 2026 | 2,386 | 465 | 19.5% | New year surge, experiments |

**Trend:** Improving but still below 25% target.

## Time to Activation Distribution

```
Day 0 (same day):    ██████████ 18%
Day 1:               ████████████████ 24%
Day 2-3:             ██████████████ 21%
Day 4-7:             ████████████ 19%
Day 8-14:            ████████ 12%
After trial (paid):  ████ 6%
```

**Insight:** 42% of activations happen in first 24 hours. If users don't activate Day 1, probability drops significantly.

## Activation by Signup Source

| Source | Signups | Activation Rate | Notes |
|--------|---------|-----------------|-------|
| Organic Search | 4,218 | 18.2% | Highest intent |
| G2/Capterra | 1,893 | 21.5% | Comparison shoppers but committed |
| Product Hunt | 847 | 8.3% | Lots of tire-kickers |
| LinkedIn Ads | 1,567 | 12.1% | Broad targeting |
| Google Ads | 2,103 | 14.8% | Good intent |
| Referral | 623 | 28.7% | Best performers |
| Direct | 596 | 16.4% | Brand aware |

## Activation by Company Size

| Size | % of Signups | Activation Rate |
|------|--------------|-----------------|
| 1-10 employees | 34% | 11.2% |
| 11-50 employees | 28% | 18.5% |
| 51-200 employees | 22% | 21.3% |
| 201-500 employees | 11% | 23.8% |
| 500+ employees | 5% | 19.1% |

**Insight:** Very small companies have lowest activation. Sweet spot is 51-200 employees.

## Activation by Role

| Role (Self-Reported) | % of Signups | Activation Rate |
|---------------------|--------------|-----------------|
| Operations | 24% | 22.4% |
| IT/Engineering | 18% | 19.8% |
| Marketing | 16% | 14.2% |
| Sales/RevOps | 14% | 17.6% |
| Founder/Executive | 12% | 9.8% |
| HR | 8% | 12.1% |
| Finance | 5% | 15.3% |
| Other | 3% | 8.7% |

**Insight:** Operations roles activate highest. Founders activate lowest (usually researching, not implementing).

## Activation by First Integration Connected

| Integration | % Who Connect First | Activation Rate After |
|-------------|---------------------|----------------------|
| Slack | 28% | 34.2% |
| Google Sheets | 22% | 29.1% |
| HubSpot | 12% | 41.3% |
| Salesforce | 9% | 38.7% |
| Notion | 8% | 24.6% |
| Jira | 7% | 36.8% |
| Gmail | 6% | 18.9% |
| Airtable | 5% | 31.2% |
| Other | 3% | 22.4% |

**Insight:** CRM integrations (HubSpot, Salesforce) have highest activation. These users have clear use cases. Gmail users often exploring and uncertain.

## Activation by First Workflow Type

| Template Category | % of First Workflows | Activation Rate |
|-------------------|---------------------|-----------------|
| Lead Management | 18% | 38.2% |
| Data Sync | 16% | 31.4% |
| Notifications | 15% | 22.1% |
| Task Automation | 14% | 28.9% |
| Reporting | 12% | 26.3% |
| Approvals | 9% | 35.7% |
| Customer Onboarding | 8% | 41.2% |
| HR Workflows | 5% | 24.8% |
| Custom (no template) | 3% | 12.3% |

**Insight:** Users who start from templates activate 2.8x more than blank canvas users.

## Behavioral Predictors of Activation

We built a model to predict activation based on Day 1 behavior:

| Behavior | Activation Lift |
|----------|-----------------|
| Connected 2+ integrations | +156% |
| Viewed template gallery | +89% |
| Watched help video | +67% |
| Invited teammate | +124% |
| Used search feature | +45% |
| Opened documentation | +38% |
| Clicked "Contact Sales" | -34% |
| Visited pricing page >3x | -28% |
| Session >20 min | +112% |

## Failure Analysis

### Why First Workflows Fail (n=4,847 failed executions)

| Reason | % | Example |
|--------|---|---------|
| OAuth token expired/invalid | 23% | "Salesforce connection failed" |
| Missing required fields | 19% | "Email field cannot be empty" |
| Rate limited by app | 14% | "HubSpot API limit exceeded" |
| Incorrect data mapping | 13% | "Expected number, got string" |
| Trigger misconfigured | 11% | "Webhook URL not registered" |
| App-side permission issue | 9% | "User lacks admin access" |
| Syntax error in formula | 6% | "Invalid filter expression" |
| Timeout | 3% | "Execution exceeded 30s limit" |
| Unknown | 2% | Various edge cases |

### User Reaction to First Failure

- 41% never return after first execution failure
- 34% retry within 24 hours
- 18% contact support
- 7% successfully debug independently

## Activation vs. Retention Correlation

Users who activated within:
- **Day 1:** 78% still active after 90 days
- **Day 2-3:** 62% still active after 90 days
- **Day 4-7:** 48% still active after 90 days
- **Day 8-14:** 31% still active after 90 days

**Conclusion:** Day 1 activation is critical for long-term retention.

## Recommendations from Data

1. **Focus on Day 1** - Most activation happens early or never
2. **Template-first experience** - 2.8x activation lift
3. **CRM integration users convert best** - Consider targeting
4. **Team invites matter** - +124% activation lift
5. **Reduce first-failure abandonment** - 41% never return
6. **Target 51-200 employee companies** - Highest activation
7. **Operations roles are best** - Tailor messaging
