# Dynamic HTML PPT Export Design

## Goal

Improve PPT export so generated HTML can drive the PowerPoint layout when the user has not explicitly selected a fixed template.

The export behavior should be:

- If `template` is non-empty, use the selected fixed template exactly as today.
- If `template` is empty, convert the returned HTML into an editable PPTX by mapping common HTML structure and inline styles.
- If the HTML has little or no styling, still produce a plain structural PPT mapping rather than falling back to `classic`, `clean`, or `business`.

## Current State

PPT export currently parses only:

- `<section>` as slides
- first `<h1>` or `<h2>` as title
- `<li>` as bullet text

All visual styling comes from backend templates. Inline HTML styles such as `font-weight`, `color`, `background-color`, card classes, and highlight boxes are discarded during export.

## Recommended Approach

Add a lightweight HTML-to-PPT rendering path beside the existing template rendering path.

This keeps the two behaviors explicit:

- Fixed-template path for deliberate user template selection.
- Dynamic HTML path for generated HTML that already contains visual intent.

The dynamic path should produce editable PPT shapes and text boxes, not a screenshot or image-only deck.

## Export Routing

`exportPPT(content, title, templateID)` should route as follows:

1. Trim `templateID`.
2. If it is non-empty, resolve and use the existing fixed template renderer.
3. If it is empty, parse HTML into a richer slide document model and render it with the dynamic HTML renderer.

Invalid template IDs should still return the current unsupported-template error.

## Dynamic Slide Model

The parser should convert each `<section>` into a slide model with:

- section-level style
- ordered block elements
- text runs with inherited inline style
- semantic class names

Supported block elements:

- `h1`, `h2`, `h3`
- `p`
- `ul`, `ol`, `li`
- `div`
- `span`

Unknown elements should be traversed for text content, but their unknown tag behavior and unsupported styles should be ignored.

## Supported Styles

Dynamic rendering should support the CSS used by generated lesson HTML:

- `background` and `background-color`
- `color`
- `font-size`
- `font-weight`
- `text-align`
- `margin-top`
- `padding`
- `border`
- `border-radius`

Supported color formats:

- `#RGB`
- `#RRGGBB`
- `rgb(r,g,b)`
- a small named-color set if needed by generated HTML

Unsupported CSS should be ignored without failing export.

## Semantic Classes

The renderer should recognize common generated class names:

- `highlight-box`: render as a filled rounded card with optional border, preserving nested text.
- `section-number`: render as a small section/page marker near the top corner.

Unknown class names should not fail export.

## Layout Strategy

The dynamic renderer should use one PPT slide per `<section>`.

Within each slide:

1. Apply section background when present; otherwise use a plain light background.
2. Lay out blocks in source order from top to bottom.
3. Render headings larger and bolder by default, unless inline styles override them.
4. Render paragraphs as editable text boxes.
5. Render lists as individual editable lines, preserving list order and inline text style.
6. Render `highlight-box` blocks as rounded cards with padding.
7. Convert simple spacing from `margin-top` and `padding` into approximate PowerPoint inches.

The goal is close visual intent, not browser-perfect layout.

## Overflow And Degradation

Dynamic export should be robust when HTML is imperfect.

Rules:

- If style parsing fails, use plain defaults for that element.
- If color parsing fails, use inherited or default color.
- If a slide has too much content, reduce body font size within a safe minimum before truncating.
- If a block is empty after text normalization, skip it.
- If no `<section>` exists, return the current invalid PPT content error.
- If sections exist but have no strong styling, still render a plain structural PPT.

## Testing

Add service tests for:

- Empty `template` uses dynamic HTML rendering.
- Non-empty `template` still uses existing fixed template rendering.
- Inline `font-weight`, `color`, and `font-size` appear in slide XML.
- Section or block background colors appear in slide XML.
- `highlight-box` creates a rounded shape and preserves text.
- `section-number` creates a marker text box.
- Plain unstyled HTML still exports a valid PPTX.
- Unsupported CSS is ignored while content remains exported.

## Scope Boundaries

Included:

- Dynamic HTML parser for common generated PPT HTML.
- Editable PPT output for text and common block shapes.
- Conditional routing based on `template`.
- Tests covering routing, style mapping, and degradation.

Not included:

- Full browser CSS layout.
- JavaScript execution.
- External image downloading.
- CSS stylesheet fetching.
- Pixel-perfect HTML rendering.
- Image-only slide screenshots.
