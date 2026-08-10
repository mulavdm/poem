> **Ported from the GopherWeb bundle at the 2026-07-16 consolidation.** "GopherWeb" = `pkg/web`; CSS prefix is now `poem-`.
# Component Asset Delivery

The `components` package embeds its own CSS and JavaScript assets under `pkg/web/components/static`.

Downstream projects can call `components.Assets()` to receive an `fs.FS`, or use `components.AssetHandler()` to serve the embedded files through `net/http`.

The scaffold serves these files at `/components/static/`. The CSS defines stable `poem-` prefixed classes and CSS variables. The JavaScript is vanilla and class-based, progressively enhancing widgets such as tabs, modals, command palettes, tables, dismissible alerts, form validation, and async button states.

## Relationships

- Widget behavior follows [Vanilla Class-Based JS](../web/vanilla-class-based-js.md).
- The public API is described in [Public Component API](../web/public-component-api.md).
