# Sales Call Notes: Acme Corp

**Company:** Acme Corporation  
**Industry:** Manufacturing & Distribution  
**Company Size:** 450 employees  
**Current Stage:** Trial → Pro Evaluation  
**ACV Potential:** $24,000/year

---

## Meeting Details

**Date:** January 14, 2026  
**Attendees:**  
- Jennifer Walsh, Director of Operations (Acme)
- Mike Chen, IT Manager (Acme)
- David Rodriguez, AE (FlowForge)
- Sarah Kim, Solutions Engineer (FlowForge)

**Meeting Type:** Demo + Discovery

---

## Company Background

Acme Corp manufactures industrial components and distributes to 2,000+ retailers. They're dealing with:
- 12 different software systems that don't talk to each other
- Manual data entry between ERP, CRM, and warehouse management
- 3 FTE hours just on daily order reconciliation
- Recent acquisition that added 2 more systems

**Current Stack:**
- NetSuite (ERP)
- Salesforce (CRM)
- ShipStation (Shipping)
- QuickBooks (AR/AP)
- Monday.com (Project Management)
- Custom warehouse system (REST API available)

---

## Key Pain Points

### 1. Order-to-Ship Chaos
> "When a sales rep closes a deal in Salesforce, someone has to manually copy the order to NetSuite, then create a pick ticket in the warehouse system. It takes 15-20 minutes per order, and we do 80 orders a day."

**Impact:** 3 FTE hours/day = $45K/year in labor alone

### 2. Inventory Disconnect
> "Our sales reps don't know real-time inventory. They promise customers items that are out of stock. We lose deals and credibility."

**Impact:** Estimated 5-8% of deals lost due to inventory issues

### 3. Customer Communication Gaps
> "Customers call asking where their order is. Our team has to check 3 different systems to answer. Average call takes 8 minutes."

**Impact:** Support overhead, customer frustration

### 4. Acquisition Integration
> "We acquired a smaller distributor 6 months ago. Their data is in completely different systems. Manual reconciliation is killing us."

**Impact:** Delayed synergies, duplicate data entry

---

## Demo Feedback

### What Resonated
- ✅ Visual workflow builder — Jennifer: "This is something my team could use, not just IT"
- ✅ Pre-built Salesforce and NetSuite connectors
- ✅ Error handling with notifications — Mike: "We need to know immediately if something fails"
- ✅ Execution logs — Mike: "Finally, audit trail for compliance"

### Concerns Raised
- ❓ Custom warehouse system — "Can you connect to our proprietary system?"
  - **Answer:** Yes, via REST API. Would need to build custom connector (~2 weeks)
  
- ❓ Data volume — "We process 80+ orders daily, 400+ line items"
  - **Answer:** Well within our capacity. Showed similar customer case study.
  
- ❓ Security — "NetSuite has sensitive financial data. What's your security posture?"
  - **Answer:** Shared SOC2 report, encryption details. Mike satisfied.
  
- ❓ Onboarding support — "We don't have time to figure this out ourselves"
  - **Answer:** Pro plan includes onboarding. Enterprise includes dedicated CSM.

---

## Use Cases Discussed

### Priority 1: Salesforce → NetSuite Order Sync
- Trigger: Opportunity closed-won in Salesforce
- Actions: Create sales order in NetSuite, update custom fields
- Estimated savings: 2 FTE hours/day

### Priority 2: Inventory Sync
- Trigger: Inventory update in warehouse system (webhook)
- Actions: Update NetSuite, update Salesforce custom field
- Estimated impact: Reduce stock-out promises by 80%

### Priority 3: Shipping Notifications
- Trigger: Shipment created in ShipStation
- Actions: Update NetSuite, send customer email, log in Salesforce
- Estimated impact: Reduce "where's my order" calls by 60%

### Future: Acquisition Data Migration
- Complex data transformation between systems
- Would require custom code blocks (mentioned Q1 roadmap)

---

## Competitive Situation

**Also Evaluating:**
- Workato — "Their quote came back at $80K/year. Too expensive."
- Zapier — "We tried it but hit limits immediately. Not enterprise-ready."
- Boomi — "Our IT team doesn't have bandwidth for that complexity."

**Why FlowForge is in the running:**
- Price-to-value ratio
- Self-serve enough for ops, robust enough for IT
- Specific NetSuite + Salesforce experience

---

## Buying Process

**Decision Makers:**
- Jennifer Walsh (Ops) — Primary champion, owns budget
- Mike Chen (IT) — Technical approval required
- CFO (name unknown) — Signs off on contracts >$15K

**Timeline:**
- Evaluate: Now through end of January
- Decision: First week of February
- Implementation: Want to start Q1

**Budget:**
- Jennifer mentioned $20-30K range
- Currently spending $45K+ on manual labor they want to eliminate

---

## Objections & Responses

| Objection | Our Response | Status |
|-----------|--------------|--------|
| "What if your company shuts down?" | 5 years in business, $8M ARR, funded through 2027. Data export available. | Resolved |
| "Our warehouse system is custom" | Walked through REST API connector process. Can do POC. | Needs POC |
| "We need 99.9% uptime for order processing" | Enterprise SLA includes 99.9%. Showed status page history. | Resolved |
| "Concerned about implementation time" | Shared 4-week typical timeline. Offered onboarding package. | Partially resolved |

---

## Technical Requirements Captured

- [ ] REST API connector for custom warehouse system
- [ ] NetSuite SuiteScript triggers (not just scheduled)
- [ ] Multi-subsidiary support in NetSuite
- [ ] Field-level mapping for custom objects in Salesforce
- [ ] IP allowlisting for security requirements
- [ ] Execution logs retained 1+ year for compliance

---

## Next Steps

| Action | Owner | Due Date |
|--------|-------|----------|
| Send proposal (Pro plan + onboarding package) | David | Jan 16 |
| Send SOC2 report and security questionnaire | Sarah | Jan 16 |
| Build POC for warehouse API connector | Sarah | Jan 21 |
| Schedule technical deep-dive with Mike | David | Jan 22 |
| Follow up for decision | David | Feb 3 |

---

## Forecast

**Stage:** Evaluation  
**Probability:** 60%  
**Expected Close:** February 2026  
**Expected ACV:** $24,000 (Pro plan + onboarding)

**Notes:** Strong champion in Jennifer. Technical approval from Mike likely after POC. Price comparison to Workato ($80K) works heavily in our favor.

---

## Potential Expansion

If initial implementation succeeds:
- Monday.com integration for project tracking
- QuickBooks sync for AR/AP automation
- Add more users (currently scoping 5, could be 15+)
- Potential upsell to Enterprise in 12-18 months

**Long-term ACV potential:** $50K+
