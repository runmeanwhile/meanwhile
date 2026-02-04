# Example 14: Semantic Memory

This example demonstrates the semantic memory capabilities of the Meanwhile framework.

## Features Demonstrated

1. **Embedding-Based Storage**: Events are automatically embedded using OpenAI's embedding API
2. **Semantic Search**: Find relevant conversations by meaning, not just keywords
3. **Smart Context Building**: Combine recent and semantically relevant messages
4. **Relevance Scoring**: Filter results by similarity threshold
5. **Message Deduplication**: Automatically remove duplicate context

## What It Does

The example:
1. Creates a semantic memory store with OpenAI embeddings
2. Populates it with a conversation history about security topics
3. Demonstrates semantic search (finding relevant content without exact keyword matches)
4. Shows smart context building (combining recent + relevant messages)
5. Runs an agent query with intelligent context retrieval

## Running the Example

```bash
export OPENAI_API_KEY=sk-...
go run main.go
```

## Expected Output

```
=== Building Conversation History ===

1. User: What is OAuth 2.0?
   Assistant: OAuth 2.0 is an authorization framework...

2. User: Tell me about JWT tokens
   Assistant: JWT (JSON Web Token) is a compact...

...

=== Semantic Search Examples ===

1. Searching for 'security credentials' (semantic, not keyword match):
   [Score: 0.823] user: What are the best practices for password storage?
   [Score: 0.791] assistant: Authentication verifies WHO you are...
   [Score: 0.765] user: Tell me about JWT tokens

...

=== Smart Context Building ===

Smart Context Retrieved:
  Total messages: 5
  
  Context includes:
    1. [user] What's the difference between authentication and authoriz...
    2. [assistant] Authentication verifies WHO you are (identity), while...
    ...
```

## Key Concepts

### Semantic Search vs. Keyword Search

Traditional keyword search requires exact matches:
```go
// Keyword: "password" only finds messages containing "password"
```

Semantic search finds conceptually related content:
```go
// Query: "security credentials"
// Finds: password storage, authentication, JWT tokens, OAuth
```

### Smart Context Building

The semantic context builder intelligently combines:
- **Recent messages**: Maintain conversation flow
- **Relevant messages**: Add historical context by meaning
- **Deduplication**: Avoid repeating the same information
- **Token limits**: Fit within model constraints

```go
builder := memory.NewSemanticContextBuilder(store)
context, err := builder.BuildSemanticContext(ctx, sessionID,
    memory.WithQuery("authentication methods"),
    memory.WithRecentMessages(5),          // Last 5 messages
    memory.WithRelevantMessages(10),       // Plus 10 relevant
    memory.WithSimilarityThreshold(0.7),   // 70% similarity minimum
    memory.WithDeduplication(true),        // Remove duplicates
)
```

## Use Cases

1. **Chatbots**: Remember past conversations and provide contextual responses
2. **Knowledge Bases**: Find relevant documentation without exact keyword matches
3. **Customer Support**: Retrieve similar past issues and solutions
4. **Research Assistants**: Find related discussions across multiple sessions
5. **Code Review**: Find similar code patterns or past decisions

## Cost Considerations

Semantic memory uses OpenAI embeddings:
- **text-embedding-3-small**: $0.02 per 1M tokens (1536 dimensions)
- **text-embedding-3-large**: $0.13 per 1M tokens (3072 dimensions)

For typical chat messages (~100 tokens each):
- 1,000 messages ≈ 100K tokens ≈ $0.002 (small model)
- 10,000 messages ≈ 1M tokens ≈ $0.02 (small model)

## Next Steps

1. Try different embedding models (see `memory.WithModel()`)
2. Experiment with similarity thresholds (0.6-0.9)
3. Adjust the balance between recent vs. relevant messages
4. Integrate with persistent storage (combine with FileChatStore)
5. Implement cross-session semantic search
