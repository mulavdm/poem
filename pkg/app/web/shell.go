package web

import (
	"html/template"
	"io"
)

// pageShell is Trellis's own minimal owned page: GopherWeb's internal/render
// and page-layout templates are unexported and cannot be imported from a
// separate module, so Trellis renders its own document shell and links
// GopherWeb's public component stylesheet and behavior script for visual
// and interactive consistency instead. components.js self-initializes on
// DOMContentLoaded (GopherWebComponents.init(document)) — no extra call
// needed here — and is what actually wires up ModalNode's open/close/focus
// trap; without it a modal's data-poem-modal-open trigger is inert.
//
// Body is wrapped in a real <form> here — every interactive Node's
// type="submit" button relies on an enclosing form to actually submit
// anything in a real browser. A modal's own open/close buttons render as
// type="button" (GopherWeb's default), so nesting the whole modal inside
// this form is safe: they never trigger a submit regardless of nesting, and
// hidden on a modal's wrapping <div> does not exclude its descendant form
// fields from submission — only disabled/name-less fields are excluded.
const pageShell = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="/components/static/components.css">
<script src="/components/static/components.js" defer></script>
<style>
  body { font-family: system-ui, sans-serif; margin: 0; padding: 2rem; }
  .trellis-container { display: flex; }
  .trellis-container--vertical { flex-direction: column; }
  .trellis-container--horizontal { flex-direction: row; align-items: center; }
  .trellis-text { margin: 0; }
</style>
</head>
<body>
<form method="post" action="/__event">
<input type="hidden" name="trellis_csrf" value="{{.CSRF}}">
{{.Body}}
</form>
</body>
</html>
`

var shellTemplate = template.Must(template.New("shell").Parse(pageShell))

type shellData struct {
	Title string
	CSRF  string
	Body  template.HTML
}

// renderShell wraps body in Trellis's document shell and writes it to w.
func renderShell(w io.Writer, title, csrf string, body template.HTML) error {
	return shellTemplate.Execute(w, shellData{Title: title, CSRF: csrf, Body: body})
}
