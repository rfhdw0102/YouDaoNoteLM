# Generation Memory Design

## Goal

Add lightweight Redis-backed memory to the generation Agent so repeated generation requests can reuse recent user preferences and prior outputs within the same notebook and generation type.

The first version should improve continuity without changing the existing generation API shape or making Redis a hard dependency for generation.

## Scope

Memory is scoped by:

- `user_id`
- `notebook_id`
- `generation_type`

This prevents memory from leaking across users, notebooks, or output types such as `ppt`, `mindmap`, `quiz`, and `note`.

## Recommended Approach

Use a lightweight Redis list per memory scope. Before generation, the service reads the latest memory entries and appends them to the existing generation context. After a successful generation, the service writes a compact memory entry back to Redis.

This approach is intentionally small:

- no new database table
- no new HTTP endpoint
- no extra LLM summarization call
- no change to the request or response contract beyond extra `meta` fields

## Redis Data Model

### Key

Use a deterministic key format:

```text
generation:memory:{user_id}:{notebook_id}:{generation_type}
```

### Entry

Each list entry is JSON:

```json
{
  "prompt": "user prompt",
  "input_summary": "short markdown summary",
  "output_summary": "short generated content summary",
  "created_at": "2026-06-15T20:00:00Z"
}
```

The entry stores summaries, not full request and output content, to control Redis size and prompt growth.

### Retention

Keep the latest 10 entries per key and set a 7 day TTL on every write.

## Service Design

Introduce a small memory abstraction in the generation service layer:

```go
type GenerationMemoryStore interface {
    GetRecent(ctx context.Context, scope GenerationMemoryScope, limit int) ([]GenerationMemoryEntry, error)
    Add(ctx context.Context, scope GenerationMemoryScope, entry GenerationMemoryEntry) error
}
```

The Redis implementation should live near the existing cache code, following the current `pkg/cache` pattern.

`generationService` should accept the memory store as an optional dependency. If the store is nil, memory is disabled.

## Generation Flow

1. Validate the generation request.
2. Build the existing generation query plan.
3. Retrieve local RAG references and optional web search results.
4. Read recent memory for `user_id + notebook_id + generation_type`.
5. Build the existing generation context and append a dedicated memory section when memory exists.
6. Run the selected sub-agent.
7. On successful generation, write a compact memory entry to Redis.
8. Return the normal generation response.

The memory section should be clearly separated from source references and web search results. It should tell the model that memory is preference and continuity context, not authoritative source material.

## Error Handling

Redis memory must be best-effort.

- If Redis is unavailable, generation continues without memory.
- If reading memory fails, generation continues and logs a warning.
- If writing memory fails, the response still succeeds and logs a warning.
- Memory failures must not be returned as user-facing generation errors.

## Response Metadata

Add generation `meta` fields for observability:

- `memory_enabled`: whether a memory store is configured
- `memory_count`: number of memory entries injected into this generation

These fields help debug behavior without changing the main response content.

## Testing

Add focused tests for:

- memory entries are appended to generation context before model generation
- successful generation writes a memory entry after the agent returns
- memory read failure does not block generation
- memory write failure does not block generation
- memory scope includes user, notebook, and generation type
- nil memory store keeps the current behavior unchanged

## Scope Boundaries

Included:

- Redis-backed recent memory for generation
- optional dependency injection in app assembly
- memory context injection
- best-effort read and write behavior
- unit tests around service behavior

Not included:

- long-term semantic memory
- LLM-generated memory summaries
- memory management endpoints
- UI changes
- persistent database history
- cross-notebook or cross-type memory sharing
