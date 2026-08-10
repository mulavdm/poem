# Vanilla Class-Based JS

`pkg/web/components/static/components.js` is a single `PoemWebComponents` class
in about 140 lines of dependency-free JavaScript. There is no bundler, no
framework, and no build step — the file is embedded in the Go binary and served
as-is.

## The shape

```js
class PoemWebComponents {
  static bindings = new WeakMap();
  static init(root = document) { return new PoemWebComponents(root).start(); }
  start() { /* bindDismiss, bindModals, bindTabs, bindTables,
                bindCommandPalettes, bindForms, bindAsyncButtons */ }
}
document.addEventListener("DOMContentLoaded", () => PoemWebComponents.init(document));
```

Each `bind*` method finds elements by a `data-poem-*` attribute or `poem-` class
and attaches behavior. The class is constructed over a root, so the same code
enhances the whole document at load or one freshly injected subtree later.

## Why a class, not modules or a framework

The server already produced correct, complete HTML. The only job left is
enhancement, and that job is small enough that a framework would be a larger
dependency than the thing it enhances. A single class keeps the whole behavior
surface readable in one file, ships without a toolchain, and cannot drift out of
sync with the CSS because both are embedded from the same directory.

It also keeps the failure mode honest: if the script never loads, every widget
still works in its baseline form.

## Idempotent initialisation

`init` is safe to call repeatedly. Bindings are recorded per element in a static
`WeakMap` of element to bound event names, and `bind` returns early if that
element already has a listener for that event. This matters because dynamic
pages call `init` again after every DOM change, and double-binding a modal close
button would close and immediately reopen it. Using a `WeakMap` rather than an
attribute flag means removed elements are garbage collected normally.

## Selector coupling is API

The script locates elements through `data-poem-*` attributes and `poem-`
classes. That makes those names part of the public contract, not private
implementation — renaming `data-poem-modal-open` breaks downstream markup as
surely as renaming an exported Go function. They are versioned alongside the Go
API; see [Public Component API](../web/public-component-api.md).

## Relationships

- [Accessibility](../web/accessibility.md) — the keyboard and semantic behavior these bindings add
- [Asset Delivery](../web/component-asset-delivery.md) — how the file is embedded and served
- [Downstream Consumption](../web/downstream-consumption.md) — re-initialising after dynamic DOM changes
