# Web Engine (`pkg/web`)

The server-rendered HTML component library and HTTP middleware (formerly GopherWeb; CSS
prefix `poem-`). The `pkg/app` web driver renders through these components; they are also
usable directly for hand-built server-rendered pages.

* [Trusted HTML](/concepts/web/trusted-html.md) — the caller-trusted `...HTML template.HTML` fields and the sanitization obligations at that boundary
