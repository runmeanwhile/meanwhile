# Sales Call Notes: RetailMax

**Company:** RetailMax  
**Industry:** E-commerce / Retail  
**Company Size:** 85 employees  
**Current Stage:** Closed Lost  
**Lost Date:** January 10, 2026

---

## Meeting Details

**Date:** December 18, 2025 (initial), January 8, 2026 (final)  
**Attendees:**  
- Priya Sharma, VP of Operations (RetailMax)
- Tom Bradley, E-commerce Manager (RetailMax)
- Lisa Wong, AE (FlowForge)
- Sarah Kim, Solutions Engineer (FlowForge)

---

## Company Background

RetailMax is a mid-sized e-commerce company selling home goods through Shopify Plus, Amazon, and their own warehouse. Heavy operations focus.

**Company Profile:**
- 85 employees, $15M ARR
- 2,000+ SKUs
- 500-800 orders/day
- Peak season (Nov-Dec) = 2,000+ orders/day

**Tech Stack:**
- Shopify Plus (Primary store)
- Amazon Seller Central
- ShipBob (3PL)
- NetSuite (ERP)
- Klaviyo (Email marketing)
- Gorgias (Support)

---

## Why They Were Evaluating

### Initial Interest
Priya reached out after seeing our Shopify integration announcement. Current situation:

> "We're using Zapier and a bunch of custom scripts. It's held together with duct tape. During Black Friday, we had three critical failures that cost us $50K in delayed shipments."

### Key Use Cases
1. **Order routing** — Shopify order → Determine fulfillment method → Route to ShipBob or internal warehouse
2. **Inventory sync** — Real-time sync between Shopify, Amazon, and NetSuite
3. **Returns processing** — Automate RMA creation, inventory adjustment, customer refund
4. **Customer service** — Auto-tag orders with issues, create Gorgias tickets

---

## What Went Well

### Demo (Dec 18)
- Shopify → ShipBob workflow was a hit
- Visual builder impressed Tom ("way better than our spaghetti scripts")
- Error handling and alerts resonated with Priya (Black Friday trauma)

### Trial (Dec 20 - Jan 5)
- Successfully built order routing workflow
- Integration connections worked smoothly
- Performance was good even with test load

**Tom's Feedback:**
> "The product is legitimately good. I built our order routing workflow in 3 hours. Our custom script took 3 weeks."

---

## What Went Wrong

### Issue 1: Amazon Integration Limitations
Our Amazon Seller Central integration doesn't support real-time inventory updates via API. Only supports scheduled polling (every 15 minutes minimum).

> Priya: "We sell on Amazon. If inventory isn't real-time, we'll oversell. This is a dealbreaker."

**Our Response:**
- Explained Amazon's API limitations (not just us)
- Offered workaround with webhooks from their middleware
- Committed to improving Amazon integration in Q2

**Outcome:** Not sufficient. They need real-time or near-real-time.

### Issue 2: Peak Season Capacity Concerns
During technical review, Tom asked about our capacity during high volume:

> "During Black Friday, we process 100,000+ API calls per day. What's your guaranteed throughput?"

**Our Response:**
- Shared capacity numbers (10K req/sec per pod)
- Showed auto-scaling architecture
- Admitted we don't have guaranteed SLA for burst capacity above standard tiers

**Outcome:** They wanted a contractual SLA for peak capacity. We couldn't provide it at their price point (would need Enterprise tier).

### Issue 3: Pricing for Their Volume
Their estimated task volume: 500K tasks/month during peak

**Pricing Discussion:**
- Pro plan (20K tasks): $149/month — way too little
- Enterprise: Custom quote came back at $2,400/month

> Priya: "That's $28K/year. We were expecting closer to $10K based on your marketing."

**Our Response:**
- Explained task-based pricing model
- Offered volume discount (got to $1,800/month)
- Still higher than their budget

### Issue 4: Competitor Came In Cheaper
During final evaluation, they got a quote from Make (Integromat):
- Make Enterprise: $12K/year (vs. our $21.6K)
- Make's Amazon integration is more mature
- Make offered peak season SLA in writing

---

## The Decision

**January 8, 2026 — Final Call**

Priya's summary:
> "We really liked FlowForge. The product is easier to use than Make. Tom preferred working in your system. But we can't justify paying almost double, and the Amazon limitation is a real problem for us."

**They chose Make (Integromat)**

---

## Loss Analysis

| Factor | Weight | FlowForge | Make | Notes |
|--------|--------|-----------|------|-------|
| Ease of use | 20% | 9/10 | 6/10 | We won |
| Amazon integration | 25% | 5/10 | 8/10 | They won |
| Pricing | 25% | 5/10 | 8/10 | They won |
| Peak SLA | 15% | 4/10 | 7/10 | They won |
| Support quality | 15% | 8/10 | 6/10 | We won |

**Overall:** We lost on the two highest-weighted factors (Amazon + pricing).

---

## Lessons Learned

### 1. Amazon Integration Gap
This is the second enterprise e-commerce deal we've lost due to Amazon limitations. Need to prioritize Amazon Seller Central improvements.

**Recommendation:** Add real-time Amazon inventory sync to Q2 roadmap.

### 2. Enterprise Pricing Model
Our per-task pricing doesn't work well for high-volume e-commerce. They'd rather pay flat rate for unlimited tasks.

**Recommendation:** Consider volume tier or flat-rate enterprise option.

### 3. Peak Season SLA
E-commerce companies need contractual guarantees for Black Friday / holiday season. We couldn't offer this.

**Recommendation:** Develop enterprise SLA package for peak capacity.

### 4. Competitive Intel
Make is being aggressive on enterprise pricing. They're willing to undercut significantly to win.

**Recommendation:** Update competitive battle cards with Make's enterprise pricing.

---

## Could We Have Won?

**If we had:**
- Real-time Amazon integration: Yes, probably would have won
- Matched Make's price ($12K): Maybe, but margins would be thin
- Offered peak SLA: Would have helped but not decisive alone

**Verdict:** This was a product gap loss, not a sales execution loss.

---

## Re-engagement Opportunity

Priya mentioned:
> "If you fix the Amazon integration, reach back out. We're not thrilled about Make's UX."

**Re-engage Triggers:**
- [ ] When Amazon integration is improved (target Q2)
- [ ] If they have issues with Make (monitor G2 reviews, LinkedIn)
- [ ] Their contract renewal (likely Dec 2026)

**Set Reminder:** July 2026 — Check in on Amazon integration status and RetailMax relationship with Make.

---

## Feedback Shared with Product

1. Amazon Seller Central real-time inventory is critical for e-commerce segment
2. High-volume pricing model needs work for enterprise e-commerce
3. Peak season SLA is table stakes for this vertical
4. Make is price-competitive and winning on integration depth

**Product Response:** Amazon improvements added to Q2 backlog. Pricing model review scheduled for February.
