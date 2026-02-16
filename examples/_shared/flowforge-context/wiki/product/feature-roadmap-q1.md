# Q1 2026 Product Roadmap

**Last Updated:** January 8, 2026  
**Owner:** Product Team  
**Status:** Approved by Leadership

## Strategic Priorities

### P0: Improve Trial Activation (60% of capacity)
Convert more trial users to paying customers by reducing friction and accelerating time-to-value.

### P1: AI-Powered Automation (25% of capacity)
Launch FlowForge AI to differentiate from competitors and simplify workflow creation.

### P2: Enterprise Readiness (15% of capacity)
Features needed to close larger deals in pipeline.

---

## January 2026

### Week 1-2: Template-First Onboarding (P0)
**Owner:** Sarah Chen  
**Status:** 🟡 In Progress

- [ ] Replace blank canvas with template selector as default entry
- [ ] Create 12 "Quick Start" templates (most common use cases)
- [ ] Add "Customize This Template" guided flow
- [ ] A/B test against current flow

**Success Metric:** +15% activation rate improvement

### Week 3-4: Onboarding Checklist Redesign (P0)
**Owner:** Sarah Chen  
**Status:** 🔵 Planned

- [ ] Persistent checklist sidebar (not modal)
- [ ] Progress celebration micro-interactions
- [ ] Contextual help based on checklist stage
- [ ] "Skip for now" with easy re-access

**Success Metric:** 80% checklist completion (currently 34%)

---

## February 2026

### Week 1-2: Guided Integration Setup (P0)
**Owner:** Jenny Liu  
**Status:** 🔵 Planned

- [ ] Step-by-step OAuth flow with screenshots
- [ ] Common error detection and solutions
- [ ] "Test Connection" button with clear feedback
- [ ] Fallback to manual token entry if OAuth fails

**Success Metric:** -50% integration setup failures

### Week 3-4: FlowForge AI v1 Launch (P1)
**Owner:** Alex Rodriguez  
**Status:** 🟡 In Development

- [ ] Natural language → workflow generation
- [ ] "What do you want to automate?" input field
- [ ] AI explains generated workflow step-by-step
- [ ] Easy refinement: "Add a filter for..." 

**Success Metric:** 30% of new workflows created via AI

---

## March 2026

### Week 1-2: Failure Recovery Experience (P0)
**Owner:** Marcus Williams  
**Status:** 🔵 Planned

- [ ] Human-readable error messages (no stack traces)
- [ ] "Fix This" button with suggested resolution
- [ ] Execution replay/debug mode
- [ ] Automatic retry with exponential backoff

**Success Metric:** -30% abandonment after first failure

### Week 3-4: Enterprise SSO & Provisioning (P2)
**Owner:** David Kim  
**Status:** 🔵 Planned

- [ ] SAML 2.0 integration
- [ ] SCIM user provisioning
- [ ] Admin console for seat management
- [ ] Audit log export

**Success Metric:** Unblock $400K in enterprise pipeline

---

## Backlog (Prioritized)

| Item | Priority | Effort | Impact | Owner |
|------|----------|--------|--------|-------|
| In-app live chat support | P1 | M | High | TBD |
| Workflow versioning & rollback | P1 | L | Medium | David |
| Mobile app (view-only) | P2 | L | Low | TBD |
| Custom branding for Enterprise | P2 | S | Medium | Jenny |
| Workflow performance analytics | P1 | M | High | Alex |
| Multi-workspace support | P2 | L | Medium | TBD |
| Sandbox/test mode | P0 | M | High | Jenny |

---

## Dependencies & Risks

### Dependencies
1. **AI v1** depends on OpenAI API rate limit increase (requested)
2. **SSO** depends on security review completion (scheduled Feb 15)
3. **Template-first** depends on content team creating new templates

### Risks
| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| AI quality insufficient | Medium | High | Fallback to template suggestions |
| Template adoption low | Low | Medium | A/B test, iterate quickly |
| Enterprise deals slip | Medium | High | Dedicate resource in March |
| Engineering bandwidth | High | Medium | Strict scope control |

---

## Q1 OKRs

### Objective 1: Make self-serve onboarding world-class
- **KR1:** Increase activation rate from 16% to 25%
- **KR2:** Reduce time-to-first-workflow from 4.2 days to 1 day
- **KR3:** Achieve 80% onboarding checklist completion

### Objective 2: Launch AI-powered workflow creation
- **KR1:** Ship FlowForge AI v1 by Feb 28
- **KR2:** 30% of new workflows created via AI by March 31
- **KR3:** AI-created workflows have ≥70% success rate on first run

### Objective 3: Close enterprise pipeline
- **KR1:** Ship SSO by March 15
- **KR2:** Close 3 enterprise deals worth ≥$100K each
- **KR3:** Achieve SOC 2 Type II certification

---

## Review Cadence

- **Weekly:** Engineering standup (Mon)
- **Bi-weekly:** Product review with leadership (Wed)
- **Monthly:** Board update on PLG metrics (First Monday)
