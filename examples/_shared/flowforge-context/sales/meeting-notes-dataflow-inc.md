# Sales Call Notes: DataFlow Inc

**Company:** DataFlow Inc  
**Industry:** Data Analytics / Consulting  
**Company Size:** 120 employees  
**Current Stage:** Negotiation  
**ACV Potential:** $36,000/year (Enterprise)

---

## Meeting Details

**Date:** January 8, 2026 (Discovery), January 15, 2026 (Technical Deep-Dive)  
**Attendees:**  
- Marcus Thompson, CTO (DataFlow)
- Jennifer Wu, Director of Engineering (DataFlow)
- Chris Anderson, VP Sales (FlowForge)
- Sarah Kim, Solutions Engineer (FlowForge)

**Meeting Type:** Enterprise Evaluation

---

## Company Background

DataFlow is a data analytics consulting firm that helps Fortune 500 companies with data infrastructure. They need internal automation to scale their consulting practice.

**Company Profile:**
- 120 employees (80 technical, 40 business)
- $18M ARR, growing 40% YoY
- 35 active client projects at any time
- Heavy Snowflake, dbt, Fivetran usage

**Tech Stack:**
- Snowflake (Data warehouse)
- dbt Cloud (Transformations)
- Fivetran (Data ingestion)
- Salesforce (CRM)
- Jira (Project management)
- Slack (Communication)
- GitHub (Code)

---

## Why They're Evaluating

### Current Pain
> Marcus: "We're a data company and we can't even keep our own data in sync. It's embarrassing. Our consultants spend 20% of their time on internal admin instead of client work."

### Key Problems
1. **Client onboarding** — Manual 47-step process to set up new client environments
2. **Project status sync** — Jira, Salesforce, Slack all have different project status
3. **Resource utilization** — No automated tracking of consultant availability
4. **Client reporting** — Manual compilation of metrics for client QBRs

### Previous Solutions Tried
- Custom Python scripts (maintenance nightmare)
- Zapier (hit limits, not powerful enough)
- Tray.io (too expensive at $100K+)

---

## Technical Requirements

### Must Haves
- ✅ Snowflake connector (read and write)
- ✅ Salesforce integration (custom objects)
- ✅ Jira integration (issue creation, status updates)
- ✅ SSO (Okta)
- ✅ Audit logs (SOC2 compliance)
- ⚠️ Custom code execution (Python preferred)
- ⚠️ API access for programmatic workflow management

### Nice to Haves
- dbt Cloud integration
- Fivetran status monitoring
- GitHub Actions integration
- Multi-environment (dev/staging/prod)

### Security Requirements
- SOC2 Type II (we have this ✅)
- Data residency options (US only is fine ✅)
- IP allowlisting ✅
- SSO enforcement ✅
- RBAC for team access ✅

---

## Demo Feedback

### January 8 — Initial Demo

**What Impressed Them:**
- Visual workflow builder — Jennifer: "Our Python scripts are black boxes. This is auditable."
- Error handling — Marcus: "I love that I can see exactly where something failed"
- Snowflake integration — "This is cleaner than I expected"

**Concerns:**
- No Python code blocks (only JavaScript currently)
- API documentation needed review
- Wanted to see complex multi-branch workflow

### January 15 — Technical Deep-Dive

Built proof-of-concept workflow:
1. Salesforce opportunity closed-won → Trigger
2. Create Jira project from template
3. Create Snowflake schema for client
4. Post to Slack #new-clients channel
5. Update Salesforce with project ID

**Jennifer's Reaction:**
> "That's our client onboarding workflow. You built it in 45 minutes. Our script took 3 sprints."

**Technical Concerns Remaining:**
1. Python support — "JavaScript is fine for simple stuff, but our team thinks in Python"
2. Workflow versioning — "We need to be able to roll back if something breaks"
3. Multi-environment — "Can we test workflows before pushing to production?"

---

## Competitive Situation

**Actively Evaluating:**
| Vendor | Status | Concern |
|--------|--------|---------|
| FlowForge | Finalist | Python, versioning |
| Workato | Finalist | Price ($80K+) |
| n8n (self-hosted) | Eliminated | Maintenance overhead |

**Decision Criteria:**
1. Technical capability (especially Snowflake) — 30%
2. Ease of use for non-developers — 25%
3. Security/compliance — 20%
4. Price — 15%
5. Support quality — 10%

**Our Position:** Strong on #1, #2, #3. Competitive on #4. Unknown on #5.

---

## Stakeholder Map

| Person | Role | Influence | Sentiment |
|--------|------|-----------|-----------|
| Marcus Thompson | CTO | Decision Maker | Positive |
| Jennifer Wu | Dir. Engineering | Technical Evaluator | Positive with concerns |
| CFO (unnamed) | Budget Holder | High | Unknown |
| CEO (unnamed) | Final Approval | Medium | Unknown |

**Champion:** Jennifer Wu — Doing most of the evaluation work  
**Economic Buyer:** Marcus Thompson — Has budget authority up to $50K

---

## Pricing Discussion

**Their Volume Estimate:**
- 50 workflows
- 200K tasks/month
- 25 users

**Our Quote (Enterprise):**
- $2,500/month base
- 500K tasks included
- 50 users included
- SSO, audit logs, priority support
- **Total: $30,000/year**

**Their Reaction:**
> Marcus: "That's reasonable. Workato quoted us $82K. I need to run this by finance but I don't anticipate pushback on price."

**Negotiation Request:**
- Annual payment upfront for discount → Offered 10% ($27K)
- Onboarding/training included → Agreed
- Dedicated CSM → Included in Enterprise

---

## Outstanding Questions

### From DataFlow:
1. **Python support timeline** — Can we commit to Q1?
   - *Our answer:* JavaScript code blocks Q1. Python planned but no committed date yet.
   
2. **Workflow versioning** — When is this coming?
   - *Our answer:* Q2 roadmap, no specific date
   
3. **Reference customer** — Do you have a similar analytics/consulting firm?
   - *Our answer:* Yes, can connect them with [Customer X]

### For DataFlow:
1. Security questionnaire completed?
   - *Status:* In progress, due Jan 20
   
2. Who else needs to be involved in final decision?
   - *Answer:* CFO will review contract, CEO trusts Marcus

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Python requirement blocks deal | 30% | High | Emphasize JS capability, share roadmap |
| Workato wins on features | 20% | High | Price advantage, ease of use |
| Internal champion leaves | 10% | High | Build relationship with Marcus |
| Budget cut | 15% | Medium | Show clear ROI calculation |
| Security questionnaire fails | 5% | High | Already compliant, low risk |

---

## Next Steps

| Action | Owner | Due Date |
|--------|-------|----------|
| Complete security questionnaire | DataFlow IT | Jan 20 |
| Provide reference customer contact | Chris | Jan 17 |
| Send contract draft | Chris | Jan 21 |
| Internal review of contract | DataFlow Legal | Jan 28 |
| Final decision meeting | Both teams | Feb 3 |

---

## Forecast

**Stage:** Negotiation  
**Probability:** 70%  
**Expected Close:** February 2026  
**Expected ACV:** $27,000 (annual prepay discount)

**Upside:** Strong technical fit, price is right, champion is engaged  
**Downside:** Python requirement could become a blocker if competitor offers it

---

## Expansion Potential

If initial deployment succeeds:
- Roll out to client-facing automation (higher volume)
- Add more users (currently scoped 25, could be 50+)
- Consulting arm could recommend FlowForge to their clients

**Strategic Value:** DataFlow advises Fortune 500 on data infrastructure. Win here = potential influence pipeline.

**24-month ACV potential:** $75K+ (if they become a partner/advocate)

---

## Quotes to Remember

> "We're a data company. If we can't automate our own operations, what are we telling our clients?" — Marcus

> "I don't want to manage another piece of infrastructure. That's why we're not doing n8n." — Jennifer

> "Show me it works with Snowflake and I'm sold." — Marcus (after demo, he's sold)
