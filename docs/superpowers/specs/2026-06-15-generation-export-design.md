# Generation Export Design

## Goal

Add a local export capability after generation completes so users can download generated artifacts by type:

- `note` exports as `.md`
- `quiz` exports as `.json`
- `mindmap` exports as `.md`
- `ppt` exports as `.pptx`

The existing generation preview API remains unchanged. Export is a separate action that converts the generated content into a downloadable file.

## Current State

- `POST /api/v1/generations` returns an in-memory `GenerationResponse`
- `note` content is Markdown
- `quiz` content is JSON
- `mindmap` content is Markmap-compatible Markdown
- `ppt` content is HTML fragments composed of multiple `<section>` blocks
- There is no current download endpoint or file export service

## Recommended Approach

Introduce a dedicated export endpoint in the generation module. The client first generates preview content through the existing API, then submits the selected generation type and content to the export API. The backend validates the content and returns a downloadable file stream with the correct filename, MIME type, and extension.

This keeps responsibilities clean:

- generation remains focused on producing preview content
- export remains focused on file packaging and format conversion

## API Design

### New Route

- `POST /api/v1/generations/export`

### Request Shape

The request body should include:

- `type`: one of `note`, `quiz`, `mindmap`, `ppt`
- `content`: generated preview content to export
- `title`: optional preferred filename stem

### Response Behavior

On success, the endpoint returns a binary file response with:

- `Content-Disposition: attachment; filename="<sanitized-name>.<ext>"`
- the correct `Content-Type`
- the file bytes in the response body

On validation failure or unsupported content structure, the endpoint returns a normal JSON business error response.

## Service Design

### Interface Addition

Extend `GenerationService` with a separate export method rather than coupling export into `Generate`.

Suggested shape:

```go
Export(ctx context.Context, req *GenerationExportRequest) (*GenerationExportResult, error)
```

Suggested export request fields:

- `Type GenerationType`
- `Content string`
- `Title string`

Suggested export result fields:

- `Filename string`
- `ContentType string`
- `Data []byte`

### Export Flow

1. Validate request is not empty
2. Validate `type`
3. Validate `content` is not blank
4. Resolve export filename stem from `title`, or derive a fallback from content/type
5. Branch by generation type
6. Produce a typed export result

## Export Rules By Type

### `note`

- Input is Markdown
- Output is raw UTF-8 `.md`
- MIME type: `text/markdown; charset=utf-8`

### `mindmap`

- Input is Markmap-compatible Markdown
- Output is raw UTF-8 `.md`
- MIME type: `text/markdown; charset=utf-8`

### `quiz`

- Input is JSON text
- Validate it is valid JSON before export
- Output is normalized UTF-8 `.json`
- MIME type: `application/json`

### `ppt`

- Input is HTML fragments containing multiple `<section>` blocks
- Parse the HTML into a slide model
- Convert the slide model into a real `.pptx`
- MIME type: `application/vnd.openxmlformats-officedocument.presentationml.presentation`

## PPT Conversion Design

### Why Conversion Is Needed

The current PPT generation agent does not produce `.pptx`. It produces structured HTML fragments such as:

- one `<section>` per slide
- `h1` or `h2` for titles
- `ul/li` for bullets

That HTML is sufficient for preview but not for native PowerPoint download. Export must convert it.

### Internal Slide Model

Create a small internal export model for PPT conversion:

- deck title
- ordered slides
- each slide has title and bullet list

### Parsing Rules

For each `<section>`:

- first `h1` or `h2` becomes the slide title
- all `li` nodes become bullet items
- empty slides are invalid

If the document contains zero valid sections, export fails with a clear error.

### PPTX Writer

Use a Go library capable of writing `.pptx` files. The writer only needs simple text slides for the first version:

- one title area
- one bullet text area
- one generated slide per parsed section

Version 1 does not need themes, images, transitions, or charts.

## Filename Rules

Filename generation should:

- prefer explicit `title`
- otherwise derive from the first heading in `content`
- otherwise fall back to a type-based default such as `note-export`

Before returning the filename:

- trim whitespace
- replace illegal filesystem characters
- cap extreme length if needed

## Error Handling

### Common Errors

- empty export request
- unsupported generation type
- blank content

### Type-Specific Errors

- `quiz`: invalid JSON
- `ppt`: invalid or unparseable HTML, or no valid slides extracted

Errors should be specific enough for the frontend to surface meaningful feedback instead of generic "export failed".

## Testing

### Controller Tests

- authenticated request reaches export service
- invalid request returns `400`
- success sets attachment headers and binary response body

### Service Tests

- `note` exports non-empty `.md`
- `mindmap` exports non-empty `.md`
- `quiz` rejects invalid JSON and exports valid JSON
- `ppt` converts simple valid HTML sections into non-empty `.pptx`
- filename sanitization behaves deterministically

## Scope Boundaries

Included in this change:

- export endpoint
- export service logic
- file streaming from backend
- content validation
- basic `.pptx` generation from current HTML structure

Not included in this change:

- persistent export history
- temporary file storage
- async export jobs
- richer PPTX styling
- alternate mindmap formats

## Implementation Notes

- Keep `Generate` API stable to avoid breaking current clients
- Prefer in-memory export bytes over temporary files for the first version
- Keep PPT parsing and PPT writing in isolated helpers so the export logic remains testable
- Frontend integration can use a single "Export" action that posts current `type`, `content`, and optional `title` to the new endpoint
