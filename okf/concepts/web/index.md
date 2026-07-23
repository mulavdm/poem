# Web Engine (`pkg/web`)

The server-rendered HTML component library and HTTP middleware (formerly GopherWeb; CSS
prefix `poem-`). The `pkg/app` web driver renders through these components; they are also
usable directly for hand-built server-rendered pages.

**Why there is no top-level `web_engine/` directory:** the top-level engine directories
(`cpp_sidecar/`, `android_engine/`, `rust_engine/`) exist for foreign-toolchain *native
presenters* that draw the engine's binary draw-command protocol. The web target never sees
draw commands — it renders semantic HTML at the component level, in pure Go — so its engine
lives under `pkg/` like all Go code: `pkg/web` is the web engine, `pkg/app/web` is its
driver.

* [Public Component API](/concepts/web/public-component-api.md) — the shared `Variant`/`Size`/`Attrs` vocabulary, the asset functions, and what counts as a pre-v1 contract
* [Trusted HTML](/concepts/web/trusted-html.md) — the caller-trusted `...HTML template.HTML` fields and the sanitization obligations at that boundary
* [Accessibility](/concepts/web/accessibility.md) — the semantic/keyboard behavior the HTML renderers promise
* [Asset Delivery](/concepts/web/component-asset-delivery.md) — how the embedded component CSS/JS ships and is served
* [Family Packages](/concepts/web/family-packages.md) — why renderers are grouped into action/form/feedback/data/layout/navigation/widget
* [Vanilla Class-Based JS](/concepts/web/vanilla-class-based-js.md) — why the enhancement layer is one dependency-free class, and why its selectors are API
* [Downstream Consumption](/concepts/web/downstream-consumption.md) — what an external project imports, serves, and re-initialises
