# Support Tickets Summary

**Period:** January 1-20, 2026  
**Total Tickets:** 1,247  
**Average First Response:** 2.4 hours  
**Average Resolution:** 18.3 hours

---

## Ticket Volume by Category

```
Onboarding/Setup     ████████████████████████████████  412 (33%)
Integration Issues   ██████████████████████           287 (23%)
Workflow Errors      █████████████████                224 (18%)
Billing/Account      ████████████                     156 (13%)
Feature Questions    ██████████                       112 (9%)
Other                ████                              56 (4%)
```

---

## Top 20 Support Topics (by frequency)

### Onboarding & Setup Issues

#### 1. "How do I connect [app]?" — 89 tickets
**Common Apps:** Salesforce, Google Sheets, Slack, HubSpot
**Root Cause:** OAuth flow not intuitive, documentation gaps
**Typical Resolution:** Walk through connection steps, verify OAuth permissions

Sample Ticket:
> "I'm trying to connect my Salesforce account but it keeps saying 'authorization failed'. I've tried 3 times."

Resolution: Customer had popup blocker enabled. OAuth window was being blocked.

---

#### 2. "My workflow isn't triggering" — 78 tickets
**Common Causes:**
- Workflow not activated (45%)
- Trigger misconfigured (28%)
- Connection expired (15%)
- Rate limit hit (12%)

Sample Ticket:
> "Set up a webhook trigger but nothing happens when I send data to it."

Resolution: Customer was using GET instead of POST for webhook.

---

#### 3. "Where do I start?" / "Overwhelmed" — 67 tickets
**Common Patterns:**
- New trial users don't know first step
- Template library not discoverable
- No guided onboarding

Sample Ticket:
> "I just signed up and I'm staring at a blank screen. The demo video was great but now I don't know what to do."

Resolution: Directed to templates, offered onboarding call. *Note: This should be unnecessary if onboarding was better.*

---

#### 4. "Can't find the right template" — 54 tickets
**Common Searches:**
- "Salesforce to Google Sheets" (have it, hard to find)
- "Stripe to QuickBooks" (don't have it)
- "Lead scoring automation" (too vague, no results)

Sample Ticket:
> "I want to sync my HubSpot contacts to Mailchimp but can't find a template. Do you support this?"

Resolution: Template exists but wasn't appearing in search. Keyword mismatch.

---

#### 5. "Integration connection keeps disconnecting" — 51 tickets
**Most Affected:** Google Workspace, Salesforce, HubSpot
**Root Cause:** OAuth token refresh issues (see BUG-2831)

Sample Ticket:
> "Third time this week my Google Sheets connection has died. I have to reconnect every few days."

Resolution: Known issue, engineering working on fix. Apologize and provide workaround.

---

### Workflow Errors

#### 6. "Workflow failed but no error message" — 48 tickets
**Related Bug:** BUG-2847  
**Customer Frustration Level:** Very High

Sample Ticket:
> "My workflow shows 'Failed' but when I click on it, there's no error. How am I supposed to fix it?"

Resolution: Escalate to engineering, manually review logs, explain what happened.

---

#### 7. "Wrong data is being passed between steps" — 43 tickets
**Common Causes:**
- User selected wrong field mapping
- Data type mismatch (string vs. number)
- Array handling confusion

Sample Ticket:
> "I'm trying to pass the email address but it's showing up as 'undefined' in Slack."

Resolution: Customer was referencing `{{email}}` but field was `{{contact.email}}`.

---

#### 8. "How do I handle errors/failures?" — 38 tickets
**Common Questions:**
- "Can I get notified when something fails?"
- "How do I set up retry logic?"
- "Can I add a fallback if step fails?"

Sample Ticket:
> "When my Salesforce API call fails, the whole workflow stops. Can I make it continue?"

Resolution: Explained error handling options (limited). Feature request logged.

---

#### 9. "Duplicate runs / running twice" — 34 tickets
**Related Bug:** BUG-2842

Sample Ticket:
> "Every time a lead comes in, I get two Slack messages instead of one. It's confusing my sales team."

Resolution: Known issue with webhook idempotency. Workaround: Add deduplication step.

---

#### 10. "Workflow is slow" — 31 tickets
**Common Causes:**
- Large payload processing
- External API slow to respond
- Too many sequential steps

Sample Ticket:
> "My workflow takes 45 seconds to complete. I need it to be faster for real-time updates."

Resolution: Reviewed workflow, found unnecessary steps. Optimized from 45s to 12s.

---

### Integration-Specific Issues

#### 11. Salesforce API Errors — 29 tickets
**Common Issues:**
- "FIELD_CUSTOM_VALIDATION_EXCEPTION"
- "REQUIRED_FIELD_MISSING"
- "INVALID_CROSS_REFERENCE_KEY"

#### 12. Google Sheets Formula Errors — 27 tickets
**Common Issues:**
- Writing to formula cells overwrites formulas
- Sheet references breaking
- Permissions on shared sheets

#### 13. Slack Formatting Issues — 24 tickets
**Related Bug:** BUG-2824 (now fixed)

#### 14. HubSpot Rate Limiting — 22 tickets
**Common Issue:** High-volume workflows hitting HubSpot's API limits

#### 15. Airtable Sync Delays — 19 tickets
**Common Issue:** Changes take 5-10 minutes to reflect in FlowForge

---

### Billing & Account

#### 16. "What counts as a task?" — 35 tickets
**Common Confusion:**
- Does a failed step count?
- Does testing count?
- Multi-step workflow = 1 task or multiple?

Sample Ticket:
> "I only ran 10 workflows but my dashboard shows 500 tasks. That can't be right."

Resolution: Each step = 1 task. 10 workflows × 50 steps = 500 tasks. Need clearer docs.

---

#### 17. "Can I downgrade my plan?" — 28 tickets
**Resolution:** Self-serve downgrade available in billing settings. Point to docs.

#### 18. "Why was I charged?" — 24 tickets
**Common Causes:**
- Forgot trial converted
- Misunderstood pricing
- Overages on usage

#### 19. "Can I get an extension/refund?" — 21 tickets
**Policy:** 14-day trial extension granted once. Pro-rated refunds within 30 days.

---

### Feature Questions

#### 20. "Can FlowForge do X?" — 48 tickets
**Common X Values:**
- Wait for human approval (no, on roadmap)
- Trigger on file upload (yes, with workaround)
- Call another workflow (no, on roadmap)
- Process PDFs (limited)

---

## Sentiment Analysis

**Positive Tickets (praise, success stories):** 8%
**Neutral Tickets (questions, how-to):** 61%
**Negative Tickets (complaints, frustration):** 31%

### Sample Positive Feedback:
> "Just wanted to say the Slack + Google Sheets integration saved me hours. Loving this tool!"

> "Your support team is amazing. Marcus resolved my issue in 10 minutes."

### Sample Negative Feedback:
> "This is the third time my workflow has broken for no reason. I'm seriously considering switching to Zapier."

> "The documentation says one thing, the product does another. Very frustrating."

---

## Support Load by Customer Segment

| Segment | Tickets | % of Segment | Avg Tickets/User |
|---------|---------|--------------|------------------|
| Trial Users | 487 | 39% | 0.23 |
| Starter | 398 | 32% | 0.18 |
| Pro | 312 | 25% | 0.15 |
| Enterprise | 50 | 4% | 0.08 |

**Insight:** Trial users generate 2.9x more support load per user than Enterprise.

---

## Response Time Performance

| Priority | Target | Actual | Status |
|----------|--------|--------|--------|
| P1 (Critical) | 1 hour | 0.8 hours | ✅ |
| P2 (High) | 4 hours | 3.2 hours | ✅ |
| P3 (Normal) | 8 hours | 6.4 hours | ✅ |
| P4 (Low) | 24 hours | 18.3 hours | ✅ |

---

## Self-Service Deflection Opportunities

Tickets that could have been self-served with better docs/UX:

| Topic | Tickets | % Deflectable | Fix |
|-------|---------|---------------|-----|
| How to connect app | 89 | 70% | In-app guide |
| Task counting confusion | 35 | 90% | Clearer UI |
| Workflow not triggering | 78 | 40% | Better error states |
| Where to start | 67 | 80% | Onboarding flow |
| Template search | 54 | 60% | Better search |

**Estimated Deflectable:** ~35% of tickets (440+ per month)

---

## Escalation Summary

| Type | Count | Avg Resolution |
|------|-------|----------------|
| Escalated to Engineering | 34 | 3.2 days |
| Escalated to Product | 12 | N/A (feature requests) |
| Escalated to Management | 3 | 1.1 days |
| Refund Requests | 28 | 0.5 days |

---

## Top Support Agents (January)

| Agent | Tickets Resolved | CSAT | Avg Resolution |
|-------|-----------------|------|----------------|
| Marcus L. | 187 | 94% | 14.2 hours |
| Sarah T. | 156 | 91% | 16.8 hours |
| James K. | 142 | 89% | 19.1 hours |
| Amanda R. | 128 | 92% | 17.4 hours |

---

## Action Items

1. **Create in-app connection troubleshooting** — Deflect 70+ tickets/month
2. **Add "activation checklist" for new users** — Reduce "where to start" tickets
3. **Improve task counting visibility** — Add tooltip, FAQ link in dashboard
4. **Better workflow error messages** — Show actionable next steps
5. **Template search improvements** — Better keywords, filters, suggestions
