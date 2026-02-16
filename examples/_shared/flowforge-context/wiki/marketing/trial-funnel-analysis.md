# Trial Funnel Analysis

**Last Updated:** January 20, 2026  
**Owner:** Growth Team  
**Data Source:** Mixpanel, Stripe, HubSpot

---

## Funnel Overview (January 2026)

```
Landing Page Visitors          ████████████████████████████████████████ 48,234
       ↓ 12.1%
Trial Signups                  █████                                    5,847
       ↓ 92.0%
Email Verified                 █████                                    5,379
       ↓ 89.8%
First Login                    ████                                     4,830
       ↓ 60.2%
Integration Connected          ██                                       2,908
       ↓ 48.7%
Workflow Created               █                                        1,416
       ↓ 72.4%
Successful Execution           █                                        1,025
       ↓ 63.8%
Activated (3+ runs)            █                                          654
       ↓ 68.7%
Converted to Paid                                                         449
```

**Overall Conversion:** Landing → Paid = 0.93%  
**Trial → Paid:** 7.7% (Jan) — *Note: Some Jan trials will convert in Feb*

---

## Traffic Sources Deep Dive

### By Volume

| Source | Visitors | Signups | Rate | Quality Score |
|--------|----------|---------|------|---------------|
| Organic Search | 18,423 | 2,103 | 11.4% | 82/100 |
| Paid Search | 12,847 | 1,567 | 12.2% | 71/100 |
| Direct | 6,234 | 847 | 13.6% | 78/100 |
| Referral | 4,123 | 623 | 15.1% | 91/100 |
| Social (Organic) | 2,891 | 312 | 10.8% | 64/100 |
| Social (Paid) | 2,103 | 280 | 13.3% | 48/100 |
| Email | 1,234 | 89 | 7.2% | 85/100 |
| G2/Review Sites | 379 | 26 | 6.9% | 88/100 |

*Quality Score = weighted combination of activation rate, conversion rate, and LTV*

### By Activation Rate (Post-Signup)

| Source | Signups | Activated | Rate | Notes |
|--------|---------|-----------|------|-------|
| Referral | 623 | 181 | 29.1% | Best performers |
| G2/Capterra | 26 | 7 | 26.9% | High intent |
| Email | 89 | 21 | 23.6% | Warm audience |
| Organic Search | 2,103 | 441 | 21.0% | Quality + volume |
| Direct | 847 | 153 | 18.1% | Brand aware |
| Paid Search | 1,567 | 251 | 16.0% | Keyword dependent |
| Social (Organic) | 312 | 31 | 9.9% | Casual browsers |
| Social (Paid) | 280 | 22 | 7.9% | Low intent |

---

## Landing Page Performance

### Current Pages

| Page | Visitors | Signups | Rate | Bounce |
|------|----------|---------|------|--------|
| Homepage | 23,891 | 2,341 | 9.8% | 42% |
| /features | 8,234 | 987 | 12.0% | 38% |
| /pricing | 6,891 | 823 | 11.9% | 31% |
| /templates | 4,123 | 689 | 16.7% | 28% |
| /use-cases/* | 3,234 | 567 | 17.5% | 33% |
| /vs-zapier | 1,234 | 298 | 24.2% | 21% |
| /vs-make | 627 | 142 | 22.6% | 24% |

**Insight:** Comparison pages convert best. High intent visitors.

### Top Converting Blog Posts

| Post | Visitors | Signups | Rate |
|------|----------|---------|------|
| "10 Slack Automations for Teams" | 2,341 | 189 | 8.1% |
| "HubSpot + Sheets Integration Guide" | 1,892 | 167 | 8.8% |
| "Zapier vs FlowForge: Honest Review" | 1,234 | 298 | 24.2% |
| "Automate Your Sales Pipeline" | 987 | 78 | 7.9% |
| "Notion Workflow Templates" | 876 | 67 | 7.6% |

---

## Trial Experience Timeline

### Day 0 (Signup Day)
- **Login Rate:** 82.6%
- **Avg Time in Product:** 12.4 minutes
- **Actions Taken:** Browse templates (34%), Connect integration (28%), View docs (21%), Leave (17%)

### Day 1
- **Return Rate:** 47.3%
- **Avg Time in Product:** 18.7 minutes
- **Key Activity:** First workflow attempt (most who return)

### Day 2-3
- **Return Rate:** 31.2%
- **Activity:** Iterating on workflows, connecting more integrations

### Day 4-7
- **Return Rate:** 22.8%
- **Activity:** Expanding use cases, inviting teammates

### Day 8-13
- **Return Rate:** 12.4%
- **Activity:** Final evaluation, pricing review

### Day 14 (Trial End)
- **Conversion Decision:** 11% convert, 89% churn or extend

---

## Cohort Retention Analysis

### 7-Day Retention by Signup Week

| Week | Signups | D1 | D3 | D7 | Trend |
|------|---------|----|----|----|----|
| Dec 30 - Jan 5 | 1,234 | 48% | 29% | 19% | Baseline |
| Jan 6 - Jan 12 | 1,456 | 51% | 32% | 22% | ↑ |
| Jan 13 - Jan 19 | 1,523 | 49% | 30% | 21% | → |
| Jan 20 - Jan 26 | 1,634 | 52% | 33% | 23% | ↑ |

**Insight:** Retention improving slightly as we iterate on onboarding.

---

## Signup Form Analysis

### Current Form Fields
1. Email (required)
2. Password (required)
3. Full Name (required)
4. Company Name (required)
5. Company Size (required, dropdown)
6. Role (required, dropdown)
7. How did you hear about us? (required, dropdown)

### Field Abandonment

| Field | Start | Complete | Drop-off |
|-------|-------|----------|----------|
| Email | 100% | 98.2% | 1.8% |
| Password | 98.2% | 96.4% | 1.8% |
| Full Name | 96.4% | 95.8% | 0.6% |
| Company Name | 95.8% | 94.1% | 1.7% |
| Company Size | 94.1% | 92.3% | 1.8% |
| Role | 92.3% | 91.1% | 1.2% |
| Attribution | 91.1% | 88.7% | 2.4% |
| **Submit** | 88.7% | 86.2% | 2.5% |

**Total Form Abandonment:** 13.8%

### Experiment: Simplified Form (Jan 15-22)
Tested removing Company Size, Role, and Attribution fields.

| Metric | Control | Variant | Change |
|--------|---------|---------|--------|
| Form Completion | 86.2% | 91.8% | +6.5% |
| Day 1 Return Rate | 47.3% | 44.1% | -6.8% |
| Activation Rate | 19.5% | 16.2% | -16.9% |

**Conclusion:** Fewer fields = more signups, but lower quality. Keeping current form.

---

## Email Engagement

### Onboarding Email Sequence

| Email | Day | Open Rate | Click Rate | Unsubscribe |
|-------|-----|-----------|------------|-------------|
| Welcome | 0 | 68.2% | 23.4% | 0.3% |
| Quick Start Guide | 1 | 42.1% | 12.8% | 0.4% |
| Template Spotlight | 3 | 38.7% | 9.2% | 0.5% |
| Success Story | 5 | 31.2% | 6.1% | 0.4% |
| Trial Reminder | 10 | 44.8% | 18.3% | 0.8% |
| Last Chance | 13 | 52.3% | 21.7% | 1.2% |

**Best Performer:** Welcome email (68% open, 23% click)  
**Worst Performer:** Success Story (low relevance to non-activated)

### Behavioral Triggers (New)

| Trigger | Condition | Email | Open Rate | Impact |
|---------|-----------|-------|-----------|--------|
| Abandoned Setup | Left during onboarding | "Finish Setup" | 51.2% | +8% return |
| First Failure | Workflow failed | "Need Help?" | 62.3% | +12% retry |
| Dormant 3 Days | No login in 3 days | "We Miss You" | 38.4% | +5% return |
| Almost Activated | 1-2 executions | "One More Run" | 54.7% | +15% activation |

---

## Pricing Page Behavior

### Visitor Analysis (n=6,891 visitors)

| Behavior | % of Visitors | Conversion Rate |
|----------|---------------|-----------------|
| Viewed once, left | 34% | 2.1% |
| Viewed once, signed up | 28% | N/A (already in trial) |
| Viewed 2-3 times | 24% | 8.7% |
| Viewed 4+ times | 14% | 4.2% |

**Insight:** 2-3 pricing page views is optimal. More indicates confusion/hesitation.

### Plan Selection (Among Converters)

| Plan | % Selected | MRR/User |
|------|------------|----------|
| Starter | 62% | $29 |
| Pro | 31% | $79 |
| Enterprise | 7% | $312 (avg) |

### Common Pricing Questions (Support Tickets)
1. "What happens after 14 days?" (34%)
2. "Can I downgrade later?" (21%)
3. "What counts as a 'task'?" (19%)
4. "Do you offer annual plans?" (14%)
5. "Is there a free tier?" (12%)

---

## Recommendations

### Immediate Wins
1. **Add comparison page CTAs** - 24% conversion, underutilized
2. **Implement behavioral email triggers** - High engagement, low effort
3. **Simplify pricing explanations** - Address top questions proactively

### Experiments to Run
1. **SSO-only signup option** - Reduce friction for Google/Microsoft shops
2. **"Start with Template" CTA** - Skip signup, go straight to templates
3. **Personalized landing pages** - Based on ad keyword/UTM

### Data Gaps to Fill
1. Why do 17% of Day 0 visitors leave without any action?
2. What's the journey of the 29% referral activators?
3. Which specific templates drive highest conversion?
