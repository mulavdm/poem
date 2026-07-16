class PoemWebComponents {
  static bindings = new WeakMap();

  constructor(root = document) { this.root = root; }
  static init(root = document) { return new PoemWebComponents(root).start(); }

  start() {
    this.bindDismiss(); this.bindModals(); this.bindTabs(); this.bindTables();
    this.bindCommandPalettes(); this.bindForms(); this.bindAsyncButtons();
    return this;
  }

  elements(selector) {
    const matched = this.root instanceof Element && this.root.matches(selector) ? [this.root] : [];
    return matched.concat(Array.from(this.root.querySelectorAll(selector)));
  }

  bind(element, event, listener) {
    if (!element) return;
    const events = PoemWebComponents.bindings.get(element) || new Set();
    if (events.has(event)) return;
    element.addEventListener(event, listener);
    events.add(event);
    PoemWebComponents.bindings.set(element, events);
  }

  bindDismiss() {
    this.elements("[data-poem-dismiss]").forEach((button) => this.bind(button, "click", () => button.closest(".poem-alert, .poem-toast")?.remove()));
  }

  bindModals() {
    this.elements("[data-poem-modal-open]").forEach((button) => this.bind(button, "click", () => this.openModal(button)));
    this.elements(".poem-modal").forEach((modal) => {
      this.bind(modal, "click", (event) => { if (event.target === modal) this.closeModal(modal); });
      this.bind(modal, "keydown", (event) => this.handleModalKeydown(event, modal));
    });
    this.elements("[data-poem-modal-close]").forEach((button) => this.bind(button, "click", () => this.closeModal(button.closest(".poem-modal"))));
  }

  openModal(button) {
    const modal = document.getElementById(button.dataset.gwModalOpen);
    if (!modal) return;
    modal._gwReturnFocus = document.activeElement;
    modal.hidden = false;
    (modal.querySelector("[data-poem-modal-close]") || modal.querySelector("button, [href], input, select, textarea") || modal.querySelector(".poem-modal__panel"))?.focus();
  }

  closeModal(modal) {
    if (!modal) return;
    modal.hidden = true;
    modal._gwReturnFocus?.focus?.();
  }

  handleModalKeydown(event, modal) {
    if (event.key === "Escape") { event.preventDefault(); this.closeModal(modal); return; }
    if (event.key !== "Tab") return;
    const items = Array.from(modal.querySelectorAll("a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])"));
    if (!items.length) return;
    const first = items[0], last = items[items.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }

  bindTabs() {
    this.elements("[data-poem-tabs]").forEach((tabs) => tabs.querySelectorAll("[data-poem-tab]").forEach((tab) => {
      this.bind(tab, "click", () => this.activateTab(tabs, tab));
      this.bind(tab, "keydown", (event) => {
        const all = Array.from(tabs.querySelectorAll("[data-poem-tab]"));
        const index = all.indexOf(tab);
        let next = null;
        if (event.key === "ArrowRight") next = all[(index + 1) % all.length];
        if (event.key === "ArrowLeft") next = all[(index - 1 + all.length) % all.length];
        if (event.key === "Home") next = all[0];
        if (event.key === "End") next = all[all.length - 1];
        if (next) { event.preventDefault(); this.activateTab(tabs, next); next.focus(); }
      });
    }));
  }

  activateTab(tabs, tab) {
    tabs.querySelectorAll("[data-poem-tab]").forEach((item) => {
      const selected = item === tab;
      item.setAttribute("aria-selected", String(selected)); item.tabIndex = selected ? 0 : -1;
    });
    tabs.querySelectorAll(".poem-tabs__panel").forEach((panel) => { panel.hidden = panel.id !== tab.dataset.gwTab; });
  }

  bindTables() {
    this.elements("[data-poem-table-filterable]").forEach((wrap) => {
      const input = wrap.querySelector("[data-poem-table-filter]");
      this.bind(input, "input", () => {
        const query = input.value.toLowerCase();
        wrap.querySelectorAll("tbody tr").forEach((row) => { row.hidden = !row.textContent.toLowerCase().includes(query); });
      });
    });
    this.elements("[data-poem-table-sortable]").forEach((wrap) => {
      const tbody = wrap.querySelector("tbody");
      wrap.querySelectorAll("[data-poem-sort]").forEach((button, index) => this.bind(button, "click", () => {
        const direction = button.dataset.gwSortDirection === "ascending" ? "descending" : "ascending";
        wrap.querySelectorAll("th[aria-sort]").forEach((header) => header.setAttribute("aria-sort", "none"));
        button.closest("th")?.setAttribute("aria-sort", direction); button.dataset.gwSortDirection = direction;
        Array.from(tbody.querySelectorAll("tr")).sort((a, b) => a.children[index].textContent.localeCompare(b.children[index].textContent) * (direction === "ascending" ? 1 : -1)).forEach((row) => tbody.appendChild(row));
      }));
    });
  }

  bindCommandPalettes() {
    this.elements("[data-poem-command]").forEach((palette) => {
      const input = palette.querySelector("[data-poem-command-input]");
      const buttons = Array.from(palette.querySelectorAll("[data-poem-command-list] button"));
      const visible = () => buttons.filter((button) => !button.closest("li").hidden);
      this.bind(input, "input", () => { const query = input.value.toLowerCase(); buttons.forEach((button) => { button.closest("li").hidden = !button.textContent.toLowerCase().includes(query); }); });
      this.bind(input, "keydown", (event) => { if ((event.key === "ArrowDown" || event.key === "Enter") && visible().length) { event.preventDefault(); event.key === "Enter" ? visible()[0].click() : visible()[0].focus(); } });
      buttons.forEach((button) => {
        this.bind(button, "click", () => palette.dispatchEvent(new CustomEvent("gw:command", { bubbles: true, detail: { value: button.dataset.value, label: button.textContent } })));
        this.bind(button, "keydown", (event) => {
          const options = visible(), position = options.indexOf(button);
          if (event.key === "ArrowDown" && position >= 0) { event.preventDefault(); options[(position + 1) % options.length].focus(); }
          if (event.key === "ArrowUp" && position >= 0) { event.preventDefault(); options[(position - 1 + options.length) % options.length].focus(); }
          if (event.key === "Escape") { event.preventDefault(); input.focus(); }
        });
      });
    });
  }

  bindForms() {
    this.elements("form[data-poem-validate]").forEach((form) => this.bind(form, "submit", (event) => {
      if (!form.checkValidity()) { event.preventDefault(); form.classList.add("poem-form--invalid"); form.reportValidity(); }
    }));
  }

  bindAsyncButtons() {
    this.elements("[data-poem-async]").forEach((button) => {
      this.bind(button, "gw:pending", () => { button.setAttribute("aria-busy", "true"); button.disabled = true; });
      this.bind(button, "gw:complete", () => { button.removeAttribute("aria-busy"); button.disabled = false; });
    });
  }
}

window.PoemWebComponents = PoemWebComponents;
document.addEventListener("DOMContentLoaded", () => PoemWebComponents.init(document));
