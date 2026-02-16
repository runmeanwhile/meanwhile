# Technical Architecture Overview

**Last Updated:** January 2026  
**Owner:** Engineering Team  
**Audience:** Internal (Engineering, DevOps, Architecture)

---

## System Overview

FlowForge is a cloud-native workflow automation platform built on a microservices architecture. The system processes approximately 500,000 workflow executions per day with 99.9% uptime.

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Load Balancer                              │
│                        (AWS ALB + CloudFront)                        │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
        ┌─────────────────────────┼─────────────────────────┐
        │                         │                         │
        ▼                         ▼                         ▼
┌───────────────┐       ┌──────────────────┐      ┌─────────────────┐
│   Web App     │       │    API Gateway   │      │   Webhook       │
│   (React)     │       │    (Kong)        │      │   Receiver      │
│   Static CDN  │       │                  │      │   (Go)          │
└───────────────┘       └────────┬─────────┘      └────────┬────────┘
                                 │                         │
                                 ▼                         │
                        ┌────────────────┐                 │
                        │  Auth Service  │                 │
                        │  (Node.js)     │                 │
                        └────────┬───────┘                 │
                                 │                         │
        ┌────────────────────────┼─────────────────────────┘
        │                        │
        ▼                        ▼
┌───────────────────────────────────────────────────────────────────┐
│                        Message Queue (RabbitMQ)                    │
│           workflow.execute | webhook.process | email.send          │
└───────────────────────────────┬───────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐     ┌──────────────────┐    ┌─────────────────┐
│   Workflow    │     │   Integration    │    │   Scheduler     │
│   Engine      │     │   Workers        │    │   Service       │
│   (Go)        │     │   (Node.js)      │    │   (Go)          │
└───────┬───────┘     └────────┬─────────┘    └────────┬────────┘
        │                      │                       │
        └──────────────────────┼───────────────────────┘
                               │
                               ▼
                ┌──────────────────────────────┐
                │        Data Layer            │
                │  PostgreSQL | Redis | S3     │
                └──────────────────────────────┘
```

---

## Core Services

### 1. Web Application
- **Technology:** React 18, TypeScript, Vite
- **Hosting:** CloudFront CDN (static assets)
- **Features:** Workflow builder, dashboard, settings
- **Key Libraries:** React Flow (canvas), TanStack Query, Zustand

### 2. API Gateway
- **Technology:** Kong Gateway
- **Responsibilities:** Rate limiting, authentication routing, request transformation
- **Rate Limits:** 
  - Free trial: 60 req/min
  - Starter: 300 req/min
  - Pro: 1000 req/min
  - Enterprise: Custom

### 3. Auth Service
- **Technology:** Node.js, Express, Passport.js
- **Authentication:** Email/password, Google OAuth, Microsoft OAuth, SSO (Enterprise)
- **Session Management:** JWT with Redis-backed session store
- **Key Features:** MFA, password reset, team invites

### 4. Workflow Engine
- **Technology:** Go 1.21
- **Responsibilities:** Workflow parsing, execution orchestration, state management
- **Architecture:** Event-driven with finite state machine per execution
- **Concurrency:** Up to 100 parallel executions per customer (configurable)
- **Key Metrics:**
  - p50 execution time: 230ms
  - p95 execution time: 1.2s
  - p99 execution time: 4.8s

### 5. Integration Workers
- **Technology:** Node.js worker threads
- **Responsibilities:** Execute individual integration steps (API calls, data transforms)
- **Worker Pool:** 50 workers per pod, auto-scales 2-20 pods
- **Integrations Supported:** 150+ apps
- **Key Features:**
  - OAuth token management
  - Automatic retry with exponential backoff
  - Request/response logging

### 6. Scheduler Service
- **Technology:** Go, using `robfig/cron`
- **Responsibilities:** Schedule-triggered workflows, delayed executions
- **Storage:** PostgreSQL for job definitions, Redis for distributed locking
- **Resolution:** 1-minute minimum interval

### 7. Webhook Receiver
- **Technology:** Go, high-performance HTTP server
- **Responsibilities:** Receive and validate inbound webhooks
- **Throughput:** 10,000 req/sec per pod
- **Validation:** HMAC signatures, IP allowlists

---

## Data Architecture

### PostgreSQL (Primary Database)
- **Version:** PostgreSQL 15
- **Hosting:** AWS RDS (Multi-AZ)
- **Schemas:**
  - `public` - Core application data
  - `workflows` - Workflow definitions and executions
  - `integrations` - OAuth tokens, connection configs
  - `analytics` - Usage metrics, aggregations

### Key Tables

```sql
-- Core tables (simplified)
users (id, email, name, created_at, ...)
organizations (id, name, plan, billing_status, ...)
teams (id, org_id, name, ...)
team_memberships (user_id, team_id, role, ...)

-- Workflow tables
workflows (id, org_id, name, trigger_type, definition_json, status, ...)
workflow_executions (id, workflow_id, status, started_at, completed_at, ...)
execution_steps (id, execution_id, step_name, status, input_json, output_json, ...)

-- Integration tables
connections (id, org_id, integration_id, credentials_encrypted, ...)
oauth_tokens (id, connection_id, access_token_enc, refresh_token_enc, expires_at, ...)
```

### Redis
- **Use Cases:**
  - Session storage (auth tokens)
  - Distributed locking (scheduler)
  - Rate limiting counters
  - Real-time execution status cache
- **Hosting:** AWS ElastiCache (cluster mode)

### S3
- **Use Cases:**
  - Workflow execution logs (long-term storage)
  - File attachments in workflows
  - Backup archives
- **Lifecycle Policy:** 90 days hot, then Glacier

---

## Integration Architecture

### OAuth Flow

```
User clicks "Connect" → Redirect to provider → Provider consent →
Redirect back with code → Exchange for tokens → Store encrypted tokens →
Token refresh on expiry
```

### Integration Categories

| Category | Examples | Auth Type | Data Flow |
|----------|----------|-----------|-----------|
| CRM | Salesforce, HubSpot | OAuth 2.0 | Bi-directional |
| Communication | Slack, Teams, Gmail | OAuth 2.0 | Bi-directional |
| Databases | PostgreSQL, MySQL | Connection string | Bi-directional |
| Spreadsheets | Google Sheets, Airtable | OAuth 2.0 | Bi-directional |
| Storage | Dropbox, Google Drive | OAuth 2.0 | Read/Write |
| Payments | Stripe, PayPal | API Key | Read + Webhooks |
| Custom | REST API, GraphQL | Various | Bi-directional |

### Webhook Processing

```
Inbound webhook → Validate signature → Parse payload →
Enqueue to RabbitMQ → Worker picks up → Match to workflow →
Execute workflow → Log result
```

**Webhook Reliability:**
- All webhooks acknowledged within 500ms
- Failed processing: Retry 3x with exponential backoff
- Dead letter queue for permanent failures

---

## Security Architecture

### Encryption
- **At Rest:** AES-256 for all PII and credentials
- **In Transit:** TLS 1.3 for all connections
- **Key Management:** AWS KMS with automatic rotation

### Authentication
- **Password Policy:** 12+ chars, complexity requirements
- **Session Duration:** 24 hours (configurable by org)
- **MFA:** TOTP (Google Authenticator, etc.)
- **SSO:** SAML 2.0, OIDC (Enterprise)

### Authorization
- **Model:** Role-Based Access Control (RBAC)
- **Roles:** Owner, Admin, Member, Viewer
- **Permissions:** 42 distinct permissions across resources

### Compliance
- **SOC 2 Type II:** Certified since 2024
- **GDPR:** Data processing agreements, right to deletion
- **HIPAA:** BAA available for Enterprise (separate environment)

### Audit Logging
- All API calls logged with user, action, resource, timestamp
- Retention: 1 year
- Export available for Enterprise customers

---

## Deployment & Infrastructure

### AWS Infrastructure
- **Region:** Primary us-east-1, DR us-west-2
- **Compute:** EKS (Kubernetes 1.28)
- **Networking:** VPC with private subnets, NAT Gateway

### CI/CD Pipeline
```
GitHub PR → Run tests → Build Docker image → Push to ECR →
Deploy to staging → Run integration tests → Manual approval →
Deploy to production (canary) → Full rollout
```

### Observability Stack
- **Metrics:** Prometheus + Grafana
- **Logging:** Fluentd → Elasticsearch → Kibana
- **Tracing:** Jaeger (OpenTelemetry)
- **Alerting:** PagerDuty integration

### Key Dashboards
1. **System Health:** CPU, memory, request rate, error rate
2. **Workflow Metrics:** Executions/min, success rate, latency percentiles
3. **Integration Health:** Per-integration error rates, latency
4. **Business Metrics:** Signups, activations, conversions

---

## Known Technical Debt

See [Onboarding Tech Debt](./onboarding-tech-debt.md) for onboarding-specific issues.

### General Technical Debt

| Item | Priority | Impact | Owner |
|------|----------|--------|-------|
| Monolith auth service | Medium | Scaling limits | Platform |
| Legacy workflow format v1 | High | Migration blockers | Engine |
| jQuery in email templates | Low | Maintenance burden | Growth |
| Inconsistent error codes | Medium | Developer experience | API |
| Manual integration updates | High | Time to market | Integrations |

### Performance Bottlenecks
1. **Workflow builder load time:** 3.2s for complex workflows (target: <1s)
2. **Large workflow execution:** >100 steps can timeout
3. **Real-time sync lag:** Up to 30s for execution status updates

---

## Capacity Planning

### Current Capacity
| Resource | Current | Limit | Utilization |
|----------|---------|-------|-------------|
| Workflow executions/day | 500K | 2M | 25% |
| Webhook requests/sec | 1.2K | 10K | 12% |
| Concurrent executions | 2K | 10K | 20% |
| Data storage | 2.4TB | 10TB | 24% |

### Growth Projections
Assuming 20% MoM growth in executions:
- 6 months: 1.5M executions/day (need to scale workers)
- 12 months: 4.5M executions/day (need database sharding)
- 18 months: 14M executions/day (need multi-region active-active)

---

## On-Call & Incident Response

### Escalation Path
1. **P4 (Low):** Engineering on-call, respond within 4 hours
2. **P3 (Medium):** Engineering on-call, respond within 1 hour
3. **P2 (High):** Engineering lead + on-call, respond within 15 min
4. **P1 (Critical):** All-hands, respond immediately

### Recent Incidents (Last 90 Days)
| Date | Severity | Duration | Root Cause |
|------|----------|----------|------------|
| Jan 12 | P3 | 23 min | RabbitMQ connection pool exhaustion |
| Jan 3 | P2 | 1h 12min | AWS us-east-1 availability zone issue |
| Dec 18 | P4 | 4h | Salesforce API rate limit exceeded |

### Status Page
Public status: [status.flowforge.io](https://status.flowforge.io)
