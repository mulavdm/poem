(() => {
  const bind = (workspace) => {
    const tools = workspace.querySelector("[data-poem-workspace-tools]");
    const handle = workspace.querySelector("[data-poem-workspace-handle]");
    if (!tools || !handle) return;
    let startY = 0;
    let startHeight = 0;
    const compact = () => matchMedia("(max-width: 599px)").matches;
    handle.addEventListener("pointerdown", (event) => {
      if (!compact()) return;
      startY = event.clientY;
      startHeight = tools.getBoundingClientRect().height;
      handle.setPointerCapture(event.pointerId);
    });
    handle.addEventListener("pointermove", (event) => {
      if (!handle.hasPointerCapture(event.pointerId)) return;
      const height = Math.max(innerHeight * .22, Math.min(innerHeight * .78, startHeight + startY - event.clientY));
      tools.style.height = `${height}px`;
    });
    handle.addEventListener("pointerup", (event) => {
      if (!handle.hasPointerCapture(event.pointerId)) return;
      handle.releasePointerCapture(event.pointerId);
      const ratio = tools.getBoundingClientRect().height / innerHeight;
      tools.style.height = `${innerHeight * (ratio < .34 ? .24 : ratio > .62 ? .76 : .46)}px`;
    });
  };
  document.querySelectorAll("[data-poem-workspace]").forEach(bind);
})();
