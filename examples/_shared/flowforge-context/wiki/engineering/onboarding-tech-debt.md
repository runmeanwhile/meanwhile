# Onboarding Technical Debt

**Last Updated:** January 2026  
**Owner:** Platform Team  
**Priority:** P0 (Critical for PLG success)

---

## Executive Summary

Our onboarding flow has accumulated significant technical debt that directly impacts our PLG metrics. This document catalogs the issues, their business impact, and proposed remediation paths.

**Key Statistic:** We estimate technical debt in onboarding is responsible for 15-20% of trial churn.

---

## Critical Issues (P0)

### 1. Synchronous OAuth Token Exchange

**Problem:**  
When users connect an integration, we perform OAuth token exchange synchronously, blocking the UI for 2-8 seconds depending on the provider.

**Symptoms:**
- Users see a blank loading screen
- No progress indicator
- 12% of users abandon during integration connection
- Support tickets: "The app froze when I tried to connect"

**Technical Details:**
```
Current flow:
User clicks "Connect" → Browser redirects to provider → User authorizes →
Provider redirects back → [BLOCKING: Token exchange 2-8s] → Show success

Users don't know what's happening during the blocking period.
```

**Root Cause:**  
Original implementation was quick MVP. Token exchange happens in the API request handler instead of being queued to a background worker.

**Proposed Fix:**
1. Make token exchange async
2. Show animated progress state ("Connecting to Slack...")
3. Use WebSocket/polling to notify completion
4. Timeout after 30s with actionable error

**Effort:** 2-3 sprints (1 engineer)  
**Business Impact:** Estimated +3-4% activation rate

---

### 2. Template Preview Doesn't Show Real Data

**Problem:**  
When users browse templates, they see placeholder data instead of a preview with their actual connected apps.

**Symptoms:**
- Templates feel abstract and generic
- Users don't understand what the template will do for them
- 34% of users who view templates don't select one
- NPS comment: "I couldn't tell which template was right for me"

**Technical Details:**
```
Current: Template shows "When [Trigger App] does X, then [Action App] does Y"
Ideal: Template shows "When Slack receives a message in #sales, then create a row in your Airtable 'Leads' base"
```

**Root Cause:**  
Templates were designed before we had the connection architecture. Retrofitting to show real data requires refactoring the template renderer.

**Proposed Fix:**
1. Query user's connected apps when loading template gallery
2. Pre-filter templates to only show compatible ones
3. Inject real app names and sample data into preview
4. Show "Connect X to use this template" CTA for missing connections

**Effort:** 3-4 sprints (2 engineers)  
**Business Impact:** Estimated +5-6% template selection rate

---

### 3. Workflow Builder Initial Load Time

**Problem:**  
The workflow builder takes 3.2 seconds to fully load on average, with p95 at 7.1 seconds.

**Symptoms:**
- Blank canvas for several seconds
- Users click multiple times thinking it didn't work
- Mobile/low-bandwidth users see even worse performance
- Support tickets: "The builder is slow/broken"

**Technical Details:**
```
Load waterfall breakdown:
- Main bundle.js: 1.8MB (gzipped 420KB)
- Integration icons: 2.3MB (loaded all upfront)
- Font files: 340KB
- Initial API calls: 1.2s (waterfall, not parallel)

Total time to interactive: 3.2s (desktop, fast connection)
```

**Root Cause:**  
1. Workflow builder not code-split from main app
2. All 150 integration icons loaded upfront
3. API calls made sequentially, not in parallel
4. No skeleton loading states

**Proposed Fix:**
1. Code-split builder into separate chunk
2. Lazy-load integration icons (only load visible + commonly used)
3. Parallelize API calls with Promise.all()
4. Add skeleton loaders for perceived performance
5. Preload builder assets after login

**Effort:** 2 sprints (1 frontend engineer)  
**Business Impact:** Estimated +2-3% activation rate

---

## High Priority Issues (P1)

### 4. Error Messages Are Cryptic

**Problem:**  
When workflows fail during onboarding, users see technical error messages instead of actionable guidance.

**Examples:**
```
Current: "Error: ECONNREFUSED at TCP.onconnect"
Should be: "We couldn't connect to Salesforce. This usually means your session expired. Click here to reconnect."

Current: "Invalid field: opportunity.closedate"
Should be: "Salesforce couldn't find the 'Close Date' field. Make sure this field exists and you have permission to access it."
```

**Root Cause:**  
Errors bubble up from integration APIs without being mapped to user-friendly messages. No error taxonomy exists.

**Proposed Fix:**
1. Create error taxonomy (auth, permission, validation, timeout, etc.)
2. Map common errors to friendly messages with fix suggestions
3. Add error context (which step, which app, what was attempted)
4. Link to relevant documentation

**Effort:** 2 sprints (1 engineer)  
**Business Impact:** Estimated +2% activation rate, -20% support tickets

---

### 5. No Progress Persistence

**Problem:**  
If users leave the onboarding flow and come back, they start from scratch.

**Symptoms:**
- Users who need to leave mid-onboarding must restart
- No "pick up where you left off" experience
- Multi-session users (42% of trials) frustrated
- Drop-off at step 3 (integration connection) doesn't recover

**Technical Details:**
```
Currently stored in localStorage only:
- Email (for pre-fill)
- Nothing else

Should be stored server-side:
- Current onboarding step
- Connected integrations
- Partial workflow state
- Selected template
```

**Root Cause:**  
MVP onboarding assumed single-session completion. No backend state management for onboarding progress.

**Proposed Fix:**
1. Create onboarding_progress table in database
2. Save state after each significant action
3. Show "Continue Setup" modal on return
4. Allow explicit "Start Over" if needed

**Effort:** 2-3 sprints (1 backend, 1 frontend engineer)  
**Business Impact:** Estimated +4-5% activation rate

---

### 6. Mobile Experience Broken

**Problem:**  
Onboarding and workflow builder are not responsive. Mobile users (18% of trials) have a broken experience.

**Symptoms:**
- Buttons hidden or overlapping on mobile
- Workflow canvas doesn't scroll properly
- Touch targets too small
- Cannot complete onboarding on mobile

**Root Cause:**  
Early development focused on desktop. CSS doesn't include mobile breakpoints. Workflow canvas uses mouse-specific events.

**Proposed Fix:**
1. Add responsive breakpoints throughout onboarding
2. Create mobile-specific workflow builder (simplified view mode)
3. Replace mouse events with pointer events
4. Add mobile-specific onboarding flow (template-only, no builder)

**Effort:** 4-5 sprints (1 frontend engineer)  
**Business Impact:** +2-3% overall activation (18% of users × 15-20% improvement)

---

## Medium Priority Issues (P2)

### 7. Rate Limiting During Trial

**Problem:**  
Trial users hit rate limits during normal exploration, creating a frustrating experience.

**Current Limits:**
- API: 60 req/min (often hit during builder exploration)
- Workflow executions: 100/day (hit by eager users)
- Integrations: 5 (artificial, not technical limit)

**Proposed Fix:**
1. Increase trial limits to match Starter plan temporarily
2. Add warning before hitting limit (not just after)
3. Soft-limit with degraded experience rather than hard block

**Effort:** 1 sprint  
**Business Impact:** Improved trial experience, fewer support tickets

---

### 8. Webhook Testing Is Manual

**Problem:**  
Users testing webhook-triggered workflows must manually send test requests using tools like Postman or curl.

**Current Experience:**
1. User creates webhook trigger
2. System shows "Waiting for webhook..."
3. User must figure out how to send a test request
4. Most users don't have Postman or don't know curl

**Proposed Fix:**
1. Add "Send Test Event" button in workflow builder
2. Show example payload with inline editor
3. Automatically populate with realistic sample data
4. Show test result immediately

**Effort:** 2 sprints  
**Business Impact:** +1-2% activation for webhook users

---

### 9. Inconsistent Loading States

**Problem:**  
Different parts of the app handle loading differently, creating jarring experiences.

**Examples:**
- Template gallery: No loader, then everything appears at once
- Integration connection: Spinning circle, no text
- Workflow execution: "Running..." with no progress indication
- Builder: Complete blank canvas for 3 seconds

**Proposed Fix:**
1. Create shared loading component library
2. Implement skeleton loaders for content
3. Add progress indicators for multi-step operations
4. Consistent spinner with context text

**Effort:** 1-2 sprints  
**Business Impact:** Improved perceived performance

---

### 10. No Guided Tour for Builder

**Problem:**  
The workflow builder has a steep learning curve with no in-app guidance.

**Symptoms:**
- Users don't know where to start
- Key features (testing, execution logs) are undiscovered
- Support tickets: "How do I add a step?"
- Users watch YouTube tutorials instead of using product

**Proposed Fix:**
1. Add first-time builder tour (dismissible)
2. Tooltips for key features
3. Contextual help (?) icons with inline docs
4. Sample workflow walkthrough

**Effort:** 2 sprints  
**Business Impact:** +2-3% activation rate

---

## Technical Debt Backlog (P3)

| Issue | Description | Effort | Owner |
|-------|-------------|--------|-------|
| Onboarding analytics gaps | Missing events for key steps | 1 sprint | Growth |
| A/B testing framework | No way to test onboarding variants | 2 sprints | Platform |
| Localization support | Onboarding is English-only | 3 sprints | Frontend |
| Accessibility audit | WCAG compliance unknown | 2 sprints | Frontend |
| SSO onboarding flow | Enterprise SSO users have different path | 2 sprints | Auth |

---

## Dependencies & Blockers

### External Dependencies
- Design resources for mobile responsive work
- PM decision on trial limits
- Legal review for tracking/analytics changes

### Internal Dependencies
- Error taxonomy requires backend API changes first
- Progress persistence needs database migration
- Mobile builder depends on core builder refactor

---

## Proposed Roadmap

### Sprint 1-2: Quick Wins
- Error messages improvement (P1)
- Rate limiting adjustments (P2)
- Loading states consistency (P2)

### Sprint 3-5: Core Fixes
- OAuth async flow (P0)
- Builder load time optimization (P0)
- Progress persistence (P1)

### Sprint 6-9: Major Improvements
- Template preview with real data (P0)
- Guided builder tour (P2)
- Webhook testing (P2)

### Sprint 10+: Mobile & Polish
- Mobile responsive (P1)
- A/B testing framework (P3)
- Accessibility audit (P3)

---

## Success Metrics

| Metric | Current | Target | Notes |
|--------|---------|--------|-------|
| Integration connection drop-off | 12% | 5% | OAuth async fix |
| Builder load time (p50) | 3.2s | <1.5s | Performance work |
| Template selection rate | 34% | 50% | Preview improvements |
| Multi-session activation | 8% | 15% | Progress persistence |
| Mobile activation | 4% | 10% | Responsive work |
| Onboarding support tickets | 180/week | <100/week | Overall improvements |

---

## Related Documents
- [Activation Metrics](../product/activation-metrics.md)
- [Onboarding Current State](../product/onboarding-current-state.md)
- [Feature Roadmap Q1](../product/feature-roadmap-q1.md)
