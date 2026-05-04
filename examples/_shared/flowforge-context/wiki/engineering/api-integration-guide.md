# API Integration Guide

**Last Updated:** January 2026  
**Audience:** Developers integrating with FlowForge  
**API Version:** v2 (current), v1 (deprecated, sunset April 2026)

---

## Quick Start

### Base URL
```
Production: https://api.flowforge.io/v2
Sandbox: https://sandbox-api.flowforge.io/v2
```

### Authentication

All API requests require authentication via API key in the header:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
     https://api.flowforge.io/v2/workflows
```

To get an API key:
1. Log into FlowForge dashboard
2. Go to Settings → API Keys
3. Click "Create New Key"
4. Copy and store securely (shown only once)

### Rate Limits

| Plan | Requests/min | Burst |
|------|--------------|-------|
| Free Trial | 60 | 100 |
| Starter | 300 | 500 |
| Pro | 1,000 | 2,000 |
| Enterprise | Custom | Custom |

Rate limit headers included in all responses:
```
X-RateLimit-Limit: 300
X-RateLimit-Remaining: 287
X-RateLimit-Reset: 1706547600
```

---

## Core Resources

### Workflows

#### List Workflows
```http
GET /v2/workflows
```

Query parameters:
| Param | Type | Description |
|-------|------|-------------|
| page | integer | Page number (default: 1) |
| per_page | integer | Items per page (max: 100) |
| status | string | Filter: active, paused, error |
| folder_id | string | Filter by folder |

Response:
```json
{
  "data": [
    {
      "id": "wf_abc123",
      "name": "Lead to Slack",
      "status": "active",
      "trigger_type": "webhook",
      "created_at": "2026-01-15T10:30:00Z",
      "updated_at": "2026-01-18T14:22:00Z",
      "last_run_at": "2026-01-20T09:15:00Z",
      "run_count": 1247,
      "error_count": 3
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 45,
    "total_pages": 3
  }
}
```

#### Get Workflow
```http
GET /v2/workflows/:id
```

Response includes full workflow definition:
```json
{
  "id": "wf_abc123",
  "name": "Lead to Slack",
  "status": "active",
  "trigger": {
    "type": "webhook",
    "config": {
      "url": "https://hooks.flowforge.io/wh_xyz789"
    }
  },
  "steps": [
    {
      "id": "step_1",
      "type": "action",
      "integration": "slack",
      "action": "send_message",
      "config": {
        "channel": "#sales-leads",
        "message": "New lead: {{trigger.name}}"
      }
    }
  ]
}
```

#### Create Workflow
```http
POST /v2/workflows
Content-Type: application/json
```

Request body:
```json
{
  "name": "My Workflow",
  "folder_id": "folder_xyz",
  "trigger": {
    "type": "webhook"
  },
  "steps": [
    {
      "type": "action",
      "integration": "slack",
      "action": "send_message",
      "connection_id": "conn_abc",
      "config": {
        "channel": "#general",
        "message": "Hello from API!"
      }
    }
  ]
}
```

#### Update Workflow
```http
PATCH /v2/workflows/:id
```

#### Delete Workflow
```http
DELETE /v2/workflows/:id
```

#### Activate/Pause Workflow
```http
POST /v2/workflows/:id/activate
POST /v2/workflows/:id/pause
```

---

### Executions

#### List Executions
```http
GET /v2/executions
```

Query parameters:
| Param | Type | Description |
|-------|------|-------------|
| workflow_id | string | Filter by workflow |
| status | string | success, failed, running |
| since | ISO8601 | Start date filter |
| until | ISO8601 | End date filter |

#### Get Execution Details
```http
GET /v2/executions/:id
```

Response includes full execution trace:
```json
{
  "id": "exec_def456",
  "workflow_id": "wf_abc123",
  "status": "success",
  "started_at": "2026-01-20T09:15:00.123Z",
  "completed_at": "2026-01-20T09:15:02.456Z",
  "duration_ms": 2333,
  "trigger_data": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "steps": [
    {
      "id": "step_1",
      "status": "success",
      "started_at": "2026-01-20T09:15:00.456Z",
      "completed_at": "2026-01-20T09:15:02.123Z",
      "input": { "channel": "#sales-leads", "message": "New lead: John Doe" },
      "output": { "ts": "1706543700.123456" }
    }
  ]
}
```

#### Trigger Manual Execution
```http
POST /v2/workflows/:id/run
Content-Type: application/json
```

Request body (optional, provides trigger data):
```json
{
  "data": {
    "name": "Test Lead",
    "email": "test@example.com"
  }
}
```

---

### Connections

#### List Connections
```http
GET /v2/connections
```

Response:
```json
{
  "data": [
    {
      "id": "conn_abc123",
      "integration": "slack",
      "name": "Acme Slack Workspace",
      "status": "active",
      "created_at": "2026-01-10T08:00:00Z",
      "last_used_at": "2026-01-20T09:15:00Z"
    }
  ]
}
```

#### Create Connection (OAuth)

Start OAuth flow:
```http
POST /v2/connections/oauth/start
Content-Type: application/json
```

Request:
```json
{
  "integration": "slack",
  "redirect_uri": "https://yourapp.com/oauth/callback"
}
```

Response:
```json
{
  "authorization_url": "https://api.flowforge.io/oauth/authorize/slack?state=xyz",
  "state": "xyz"
}
```

Complete OAuth flow:
```http
POST /v2/connections/oauth/complete
Content-Type: application/json
```

Request:
```json
{
  "integration": "slack",
  "code": "oauth_code_from_callback",
  "state": "xyz"
}
```

#### Delete Connection
```http
DELETE /v2/connections/:id
```

---

### Webhooks

#### Get Webhook URL
Each webhook trigger gets a unique URL:
```http
GET /v2/workflows/:id/webhook
```

Response:
```json
{
  "url": "https://hooks.flowforge.io/wh_xyz789",
  "method": "POST",
  "headers": {
    "required": ["Content-Type"],
    "signature": "X-FlowForge-Signature"
  }
}
```

#### Webhook Signature Verification

Webhooks include HMAC signature for verification:

```
X-FlowForge-Signature: sha256=abc123...
X-FlowForge-Timestamp: 1706543700
```

Verification example (Node.js):
```javascript
const crypto = require('crypto');

function verifyWebhook(payload, signature, timestamp, secret) {
  const signedPayload = `${timestamp}.${payload}`;
  const expectedSignature = crypto
    .createHmac('sha256', secret)
    .update(signedPayload)
    .digest('hex');
  return `sha256=${expectedSignature}` === signature;
}
```

---

## Data Types

### Trigger Types

| Type | Description | Config |
|------|-------------|--------|
| webhook | Incoming HTTP request | url (auto-generated) |
| schedule | Cron-based schedule | cron_expression |
| app_event | Integration event | integration, event_type |
| manual | API or UI triggered | none |

### Step Types

| Type | Description |
|------|-------------|
| action | Execute integration action |
| condition | If/else branching |
| loop | Iterate over array |
| delay | Wait for specified time |
| code | Custom JavaScript |

### Status Values

| Resource | Statuses |
|----------|----------|
| Workflow | draft, active, paused, error |
| Execution | queued, running, success, failed, cancelled |
| Connection | active, expired, error |

---

## Error Handling

### Error Response Format
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid workflow configuration",
    "details": [
      {
        "field": "steps[0].config.channel",
        "message": "Channel is required"
      }
    ]
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| UNAUTHORIZED | 401 | Invalid or missing API key |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource doesn't exist |
| VALIDATION_ERROR | 400 | Invalid request body |
| RATE_LIMIT_EXCEEDED | 429 | Too many requests |
| CONNECTION_ERROR | 502 | Integration connection failed |
| INTERNAL_ERROR | 500 | Server error |

### Retry Logic

Recommended retry strategy:
- Retry on 429, 500, 502, 503, 504
- Use exponential backoff: 1s, 2s, 4s, 8s, 16s
- Maximum 5 retries
- Include Idempotency-Key header for POST/PATCH

```javascript
const retry = async (fn, maxRetries = 5) => {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      if (![429, 500, 502, 503, 504].includes(error.status)) throw error;
      await sleep(Math.pow(2, i) * 1000);
    }
  }
};
```

---

## SDKs & Libraries

### Official SDKs

**Node.js**
```bash
npm install @flowforge/sdk
```

```javascript
const FlowForge = require('@flowforge/sdk');

const client = new FlowForge({ apiKey: 'YOUR_API_KEY' });

// List workflows
const workflows = await client.workflows.list();

// Trigger workflow
await client.workflows.run('wf_abc123', { data: { name: 'Test' } });
```

**Python**
```bash
pip install flowforge
```

```python
from flowforge import FlowForge

client = FlowForge(api_key="YOUR_API_KEY")

# List workflows
workflows = client.workflows.list()

# Trigger workflow
client.workflows.run("wf_abc123", data={"name": "Test"})
```

### Community SDKs

| Language | Package | Maintainer |
|----------|---------|------------|
| Ruby | flowforge-ruby | @community/ruby |
| Go | go-flowforge | @community/go |
| PHP | flowforge-php | @community/php |

---

## Webhooks (Outbound)

### Event Types

Subscribe to events via Settings → Webhooks:

| Event | Description |
|-------|-------------|
| workflow.created | New workflow created |
| workflow.activated | Workflow turned on |
| workflow.paused | Workflow turned off |
| execution.started | Execution began |
| execution.completed | Execution finished (success or fail) |
| execution.failed | Execution failed |
| connection.expired | OAuth token expired |

### Event Payload
```json
{
  "event": "execution.completed",
  "timestamp": "2026-01-20T09:15:02.456Z",
  "data": {
    "execution_id": "exec_def456",
    "workflow_id": "wf_abc123",
    "status": "success",
    "duration_ms": 2333
  }
}
```

---

## Best Practices

### 1. Use Idempotency Keys
```http
POST /v2/workflows/:id/run
Idempotency-Key: unique-request-id-12345
```

### 2. Handle Pagination
```javascript
async function getAllWorkflows() {
  let workflows = [];
  let page = 1;
  while (true) {
    const response = await client.workflows.list({ page, per_page: 100 });
    workflows = workflows.concat(response.data);
    if (page >= response.meta.total_pages) break;
    page++;
  }
  return workflows;
}
```

### 3. Use Webhook Signature Verification
Always verify webhook signatures to prevent replay attacks.

### 4. Implement Graceful Degradation
Handle API errors without crashing your application.

### 5. Cache Connection Status
Check connection health periodically, not on every request.

---

## Sandbox Environment

The sandbox environment mirrors production but:
- Rate limits are more generous
- Data is reset weekly
- Connected integrations use sandbox/test accounts
- Executions don't affect real systems

Sandbox URL: `https://sandbox-api.flowforge.io/v2`

---

## Support

- **API Status:** [status.flowforge.io](https://status.flowforge.io)
- **Documentation:** [docs.flowforge.io](https://docs.flowforge.io)
- **Discord:** [discord.gg/flowforge](https://discord.gg/flowforge)
- **Email:** api-support@flowforge.io
