# Bug Reports Summary

**Last Updated:** January 20, 2026  
**Source:** Support Tickets, Intercom, Error Tracking (Sentry)  
**Period:** Last 90 days

---

## Critical Bugs (P0) — Immediate Impact

### BUG-2847: Workflow Stops Without Error
**Status:** 🔴 Open | **Affected Users:** ~340 | **Reported:** Jan 15, 2026

**Description:**  
Workflows intermittently stop mid-execution without logging an error. No failure notification sent. Users only discover when downstream systems don't update.

**Reproduction Rate:** ~0.3% of executions  
**Steps to Reproduce:** Cannot reliably reproduce  
**Suspected Cause:** Race condition in workflow engine when processing parallel branches

**Customer Impact:**
> "My lead routing just... stopped. I had 47 leads sitting in limbo for 6 hours before I noticed." — Pro customer

**Engineering Notes:**  
Added additional logging Jan 17. Investigating RabbitMQ consumer acknowledgment timing.

---

### BUG-2831: OAuth Token Refresh Failing for Google
**Status:** 🟡 In Progress | **Affected Users:** ~180 | **Reported:** Jan 10, 2026

**Description:**  
Google Workspace connections (Sheets, Drive, Gmail) randomly lose authentication. Refresh token flow returns 400 error. Users must manually reconnect.

**Reproduction Rate:** Affects ~8% of Google connections after 7+ days  
**Error:** `invalid_grant: Token has been expired or revoked`  
**Root Cause:** Our refresh token storage wasn't handling Google's token rotation correctly.

**Fix ETA:** Jan 22, 2026  
**Workaround:** Users can reconnect, but this is disruptive.

---

### BUG-2819: Execution Timeout Not Respected
**Status:** 🔴 Open | **Affected Users:** ~95 | **Reported:** Jan 5, 2026

**Description:**  
Some workflows exceed the 5-minute timeout but continue running, then fail with "Internal Error" instead of "Timeout."

**Customer Impact:**
> "My workflow ran for 23 minutes, then failed. Why didn't it timeout at 5 min like the docs say?" — Enterprise customer

**Suspected Cause:** Timeout is checked at step level, not execution level. Long API calls can exceed.

---

## High Priority Bugs (P1) — Significant Impact

### BUG-2842: Duplicate Webhook Deliveries
**Status:** 🟡 In Progress | **Affected Users:** ~220 | **Reported:** Jan 14, 2026

**Description:**  
Webhook-triggered workflows sometimes execute twice for a single incoming webhook. Causing duplicate records in downstream systems.

**Reproduction Rate:** ~2% of webhook-triggered executions  
**Root Cause:** At-least-once delivery in message queue + missing idempotency check

**Customer Impact:**
> "Got duplicate Slack messages for every lead. My sales team thought it was a system issue." — Pro customer

**Fix ETA:** Jan 25, 2026

---

### BUG-2836: Date Parsing Incorrect for Non-US Formats
**Status:** 🔴 Open | **Affected Users:** ~150 | **Reported:** Jan 12, 2026

**Description:**  
Dates in DD/MM/YYYY format are parsed as MM/DD/YYYY, causing incorrect dates in workflows.

**Example:**  
- Input: 05/01/2026 (January 5th, UK format)
- Parsed as: May 1st, 2026

**Affected Integrations:** All integrations that pass dates as strings  
**Customer Impact:** International customers, particularly UK, Australia, Europe

---

### BUG-2824: Slack Message Formatting Breaks
**Status:** 🟢 Fixed (Jan 18) | **Affected Users:** ~340 | **Reported:** Jan 7, 2026

**Description:**  
Slack markdown formatting (bold, code blocks, links) renders as plain text.

**Root Cause:** Slack API migration to Blocks format. Our messages were using legacy format.  
**Resolution:** Updated to Block Kit format for all Slack messages.

---

### BUG-2815: Large Payload Workflows Fail Silently
**Status:** 🔴 Open | **Affected Users:** ~45 | **Reported:** Jan 3, 2026

**Description:**  
Workflows processing data payloads >1MB fail without clear error. Execution shows "Running" indefinitely.

**Error (in logs only):** `payload exceeds maximum size`  
**Expected:** User-facing error with guidance on payload limits

---

### BUG-2808: Condition "Contains" Case Sensitivity
**Status:** 🟢 Fixed (Jan 12) | **Affected Users:** ~280 | **Reported:** Dec 28, 2025

**Description:**  
The "contains" condition was case-sensitive, causing unexpected behavior.

**Example:**  
- Condition: Subject contains "URGENT"
- Email subject: "Urgent request"
- Result: Condition evaluated as FALSE (should be TRUE)

**Resolution:** Added case-insensitive option (now default).

---

## Medium Priority Bugs (P2)

| Bug ID | Title | Status | Affected | Reported |
|--------|-------|--------|----------|----------|
| BUG-2839 | Template preview shows stale data | 🔴 Open | ~120 | Jan 13 |
| BUG-2833 | Email notification links broken on mobile | 🟡 In Progress | ~85 | Jan 11 |
| BUG-2827 | Workflow builder undo sometimes undoes too much | 🔴 Open | ~190 | Jan 8 |
| BUG-2821 | Execution log timestamps in wrong timezone | 🔴 Open | ~160 | Jan 6 |
| BUG-2816 | Copy/paste steps loses connections | 🔴 Open | ~75 | Jan 4 |
| BUG-2812 | Search doesn't find workflows by trigger type | 🔴 Open | ~40 | Jan 2 |
| BUG-2805 | CSV parsing fails on quoted commas | 🟢 Fixed | ~65 | Dec 26 |
| BUG-2799 | Airtable rate limiting not handled gracefully | 🟡 In Progress | ~110 | Dec 22 |

---

## Low Priority Bugs (P3)

| Bug ID | Title | Affected | Status |
|--------|-------|----------|--------|
| BUG-2844 | Dark mode: Some buttons hard to see | ~45 | Backlog |
| BUG-2838 | Typo in error message ("occured" → "occurred") | All | Backlog |
| BUG-2829 | Keyboard shortcut conflicts with browser | ~25 | Backlog |
| BUG-2823 | Avatar images not loading for some users | ~30 | Backlog |
| BUG-2817 | Export to CSV missing column headers | ~20 | Backlog |

---

## Bug Metrics

### Last 90 Days
| Metric | Value | Trend |
|--------|-------|-------|
| Total Bugs Reported | 287 | ↑ 12% |
| P0 Bugs | 8 | ↑ 2 |
| P1 Bugs | 23 | → |
| Bugs Fixed | 198 | ↑ 8% |
| Mean Time to Fix (P0) | 4.2 days | ↓ (better) |
| Mean Time to Fix (P1) | 11.3 days | → |
| Bugs Reopened | 12 | ↓ |

### By Category
| Category | Count | % of Total |
|----------|-------|------------|
| Integrations | 89 | 31% |
| Workflow Builder | 67 | 23% |
| Execution Engine | 52 | 18% |
| UI/UX | 41 | 14% |
| Authentication | 23 | 8% |
| Other | 15 | 5% |

### By Affected Integration
| Integration | Bug Count |
|-------------|-----------|
| Google Workspace | 34 |
| Slack | 28 |
| Salesforce | 21 |
| HubSpot | 18 |
| Airtable | 15 |
| Notion | 12 |
| Other | 31 |

---

## Customer-Reported vs. Internal Discovery

| Source | Count | % |
|--------|-------|---|
| Support Ticket | 142 | 49% |
| In-App Feedback | 67 | 23% |
| Error Monitoring (Sentry) | 48 | 17% |
| Internal Testing | 21 | 7% |
| Social Media | 9 | 3% |

**Concern:** 72% of bugs are customer-reported. We need better automated detection.

---

## Bugs Causing Churn

Exit survey mentions (last 90 days):

| Bug/Issue | Churn Mentions |
|-----------|----------------|
| "Workflows just stopped/broke" | 34 |
| "Connection kept disconnecting" | 27 |
| "Too many errors/bugs" (general) | 21 |
| "Data wasn't syncing correctly" | 18 |
| "Couldn't get workflow to work" | 15 |

**Total Churn Mentions of Bugs:** 115 (17% of churned users)

---

## Known Issues (Not Bugs)

These are limitations, not bugs, but cause confusion:

1. **Max 100 steps per workflow** — Architectural limit
2. **15-minute minimum for scheduled triggers** — By design
3. **No real-time sync for some integrations** — API limitations (e.g., HubSpot)
4. **File size limit 10MB** — Infrastructure constraint
5. **Webhook timeout 30 seconds** — Standard practice

---

## How to Report Bugs

**Customers:**
- In-app: Help → Report a Bug
- Email: bugs@flowforge.io
- Support chat: Describe the issue

**Include:**
- What you expected to happen
- What actually happened
- Steps to reproduce
- Screenshots/screen recording
- Workflow ID (if applicable)

**Internal:**
- Sentry for automatic error tracking
- #bugs Slack channel for observed issues
- Jira for tracking and prioritization
