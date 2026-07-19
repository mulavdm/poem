# M6 migration: semantic adaptive design

M6 deliberately replaces application-authored visual layout with semantic intent.

- Set `App.Design` only for a product override. Nil selects POEM's native Windows, Android, or web profile and falls back to Adaptive House.
- Derive actions from state with `App.Commands`. Nodes reference command IDs; invocation remains a transport-safe `Msg`, and asynchronous work remains a `Cmd` returned by `Update`.
- Replace visual controls with semantic actions, labelled fields, collections, navigation, status, progress, disclosure, dialog, form, workspace, media, and viewport nodes.
- Give every semantic node a stable `Semantic.ID` and an accessible name or persistent label.
- Use `AdaptiveNode` for Compact, Medium, Expanded, and Ultra-wide alternatives. Breakpoints are fixed at 600, 840, and 1200 logical pixels.
- Remove application colors, radii, elevation, padding, and pixel gaps. Renderers obtain those values from `pkg/design`.
- Gate application trees in tests with `designlint.AssertClean`.

`pkg/render` remains the advanced native component layer and consumes the same resolved design environment and token theme.
