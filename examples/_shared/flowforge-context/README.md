# Shared Example Data: FlowForge Context

This directory contains mock organizational data for a fictional B2B SaaS company called **FlowForge**. It's designed to provide realistic context for examples that demonstrate semantic search, RAG (retrieval-augmented generation), and multi-agent brainstorming.

## About FlowForge

**FlowForge** is a workflow automation platform competing with Zapier, Make, and Workato. The company is transitioning from Sales-Led to Product-Led Growth and recently launched a 14-day free trial.

### Key Metrics (Fictional)
- **ARR:** $8.2M
- **Employees:** 52
- **Monthly Trials:** 2,100
- **Trial-to-Paid Conversion:** 11%
- **Activation Rate:** 16% (target: 25%)
- **Trial Churn:** 68%

### Core Problem
New trial users struggle to experience value quickly. The time-to-first-workflow averages 4.2 days—most users churn before reaching that milestone.

## Directory Structure

```
flowforge-context/
├── wiki/
│   ├── product/           # Product docs, metrics, roadmap
│   │   ├── product-overview.md
│   │   ├── onboarding-current-state.md
│   │   ├── activation-metrics.md
│   │   ├── feature-roadmap-q1.md
│   │   └── competitor-analysis.md
│   ├── marketing/         # PLG strategy, personas, messaging
│   │   ├── plg-strategy.md
│   │   ├── trial-funnel-analysis.md
│   │   ├── persona-definitions.md
│   │   └── messaging-framework.md
│   └── engineering/       # Architecture, tech debt
│       ├── technical-architecture.md
│       ├── onboarding-tech-debt.md
│       └── api-integration-guide.md
├── customer-feedback/     # Voice of customer
│   ├── feature-requests.md
│   ├── bug-reports.md
│   ├── support-tickets-summary.md
│   └── nps-survey-responses.md
└── sales/                 # Sales intelligence
    ├── meeting-notes-acme-corp.md
    ├── meeting-notes-techstart.md
    ├── meeting-notes-retailmax.md
    ├── meeting-notes-dataflow-inc.md
    ├── lost-deal-analysis.md
    └── win-loss-patterns.md
```

## Usage

### With DocumentStore (Semantic Search)

```go
import "github.com/runmeanwhile/meanwhile/pkg/memory"

// Create embeddings provider
embedder := memory.NewOpenAIEmbeddings(apiKey)

// Create and index document store
docStore := memory.NewDocumentStore(embedder)
err := docStore.IndexDirectory(ctx, "examples/_shared/flowforge-context")

// Semantic search
results, err := docStore.Search(ctx, memory.DocumentQuery{
    Text:      "how to improve trial conversion",
    Limit:     10,
    Threshold: 0.5,
})
```

### Examples Using This Data

- **Example 26**: IDEO-inspired brainstorming with semantic recall tool
- (Future examples can use this same knowledge base)

## Document Statistics

- **Total Files:** 22 markdown documents
- **Total Lines:** ~5,800
- **Categories:** 5 (wiki/product, wiki/marketing, wiki/engineering, customer-feedback, sales)

## Notes

- All data is fictional and for demonstration purposes only
- Customer names, quotes, and metrics are fabricated
- Company/competitor names should not be confused with real entities
