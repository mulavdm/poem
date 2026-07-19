(() => {
  const bind = (root) => {
    const breakpoint = Number(root.dataset.breakpoint) || 600;
    const compact = root.querySelector(":scope > [data-poem-compact]");
    const wide = root.querySelector(":scope > [data-poem-wide]");
    if (!compact || !wide) return;

    const media = window.matchMedia(`(min-width: ${breakpoint + 1}px)`);
    const update = () => {
      compact.hidden = media.matches;
      compact.disabled = media.matches;
      wide.hidden = !media.matches;
      wide.disabled = !media.matches;
    };
    update();
    media.addEventListener("change", update);
  };

  document.querySelectorAll("[data-poem-responsive]").forEach(bind);
})();
