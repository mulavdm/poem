(() => {
  const number = (element, name, fallback) => {
    const value = Number(element.dataset[name]);
    return Number.isFinite(value) ? value : fallback;
  };

  const bind = (viewport) => {
    if (viewport.dataset.disabled === "true") return;
    const image = viewport.querySelector("img");
    const field = viewport.dataset.field;
    const activateField = viewport.dataset.activateField;
	const markers = [...viewport.querySelectorAll("[data-poem-image-marker]")];
    const minScale = number(viewport, "minScale", 0.5);
    const maxScale = number(viewport, "maxScale", 8);
    let transform = {
      x: number(viewport, "x", 0),
      y: number(viewport, "y", 0),
      scale: number(viewport, "scale", 1) || 1,
    };
    const pointers = new Map();
    let startTransform = {...transform};
    let startPoint = null;
    let startPinch = null;
    let wheelTimer = 0;
    let dirty = false;
	let maxPointers = 0;
	let resizeTimer = 0;

    const clamp = () => {
      transform.x = Math.max(-4, Math.min(4, transform.x));
      transform.y = Math.max(-4, Math.min(4, transform.y));
      transform.scale = Math.max(minScale, Math.min(maxScale, transform.scale));
    };
    const draw = () => {
      clamp();
      const rect = viewport.getBoundingClientRect();
      image.style.transform = `translate(${transform.x * rect.width}px, ${transform.y * rect.height}px) scale(${transform.scale})`;
	  markers.forEach((marker) => {
		const x = number(marker, "imageX", 0.5);
		const y = number(marker, "imageY", 0.5);
		marker.style.left = `${50 + (transform.x + (x - 0.5) * transform.scale) * 100}%`;
		marker.style.top = `${50 + (transform.y + (y - 0.5) * transform.scale) * 100}%`;
	  });
    };
	const commitValue = (targetField, value) => {
	  const csrf = document.querySelector('input[name="trellis_csrf"]');
	  if (!csrf || !targetField) return;
	  const form = document.createElement("form");
	  form.method = "post";
	  form.action = "/__field";
	  for (const [name, fieldValue] of [
		["trellis_csrf", csrf.value],
		["trellis_field", targetField],
		["trellis_value", value],
	  ]) {
		const input = document.createElement("input");
		input.type = "hidden";
		input.name = name;
		input.value = fieldValue;
		form.appendChild(input);
	  }
	  document.body.appendChild(form);
	  form.submit();
	};
    const commit = () => {
      if (!dirty) return;
      dirty = false;
	  const rect = viewport.getBoundingClientRect();
	  commitValue(field, JSON.stringify({...transform, width: Math.max(32, Math.round(rect.width)), height: Math.max(32, Math.round(rect.height))}));
    };
    const point = (event) => ({x: event.clientX, y: event.clientY});
    const pinchState = () => {
      const values = [...pointers.values()];
      if (values.length < 2) return null;
      const a = values[0], b = values[1];
      return {
        x: (a.x + b.x) / 2,
        y: (a.y + b.y) / 2,
        distance: Math.max(1, Math.hypot(a.x - b.x, a.y - b.y)),
      };
    };

    viewport.addEventListener("pointerdown", (event) => {
      event.preventDefault();
      viewport.setPointerCapture(event.pointerId);
      pointers.set(event.pointerId, point(event));
	  maxPointers = Math.max(maxPointers, pointers.size);
      startTransform = {...transform};
      startPoint = point(event);
      startPinch = pinchState();
    });
    viewport.addEventListener("pointermove", (event) => {
      if (!pointers.has(event.pointerId)) return;
      event.preventDefault();
      pointers.set(event.pointerId, point(event));
      const rect = viewport.getBoundingClientRect();
      const pinch = pinchState();
      if (pinch && startPinch) {
        transform.scale = startTransform.scale * pinch.distance / startPinch.distance;
        transform.x = startTransform.x + (pinch.x - startPinch.x) / Math.max(1, rect.width);
        transform.y = startTransform.y + (pinch.y - startPinch.y) / Math.max(1, rect.height);
      } else if (startPoint) {
		if (Math.hypot(event.clientX - startPoint.x, event.clientY - startPoint.y) <= 6 && !dirty) return;
        transform.x = startTransform.x + (event.clientX - startPoint.x) / Math.max(1, rect.width);
        transform.y = startTransform.y + (event.clientY - startPoint.y) / Math.max(1, rect.height);
      }
      dirty = true;
      draw();
    });
	const endPointer = (event) => {
      if (!pointers.delete(event.pointerId)) return;
	  if (pointers.size === 0) {
		if (dirty) commit();
		else if (event.type === "pointerup" && maxPointers === 1 && activateField) {
		  const rect = viewport.getBoundingClientRect();
		  const x = Math.max(0, Math.min(1, (event.clientX - rect.left) / Math.max(1, rect.width)));
		  const y = Math.max(0, Math.min(1, (event.clientY - rect.top) / Math.max(1, rect.height)));
		  commitValue(activateField, JSON.stringify({x, y}));
		}
		maxPointers = 0;
	  }
      else {
        startTransform = {...transform};
        startPoint = [...pointers.values()][0];
        startPinch = pinchState();
      }
    };
    viewport.addEventListener("pointerup", endPointer);
    viewport.addEventListener("pointercancel", endPointer);
	markers.forEach((marker) => marker.addEventListener("pointerdown", (event) => event.stopPropagation()));
    viewport.addEventListener("wheel", (event) => {
      event.preventDefault();
      const rect = viewport.getBoundingClientRect();
      const oldScale = transform.scale;
      const nextScale = Math.max(minScale, Math.min(maxScale, oldScale * Math.exp(-event.deltaY * 0.0015)));
      const ratio = nextScale / oldScale - 1;
      const fx = (event.clientX - rect.left) / Math.max(1, rect.width) - 0.5;
      const fy = (event.clientY - rect.top) / Math.max(1, rect.height) - 0.5;
      transform.x -= (fx - transform.x) * ratio;
      transform.y -= (fy - transform.y) * ratio;
      transform.scale = nextScale;
      dirty = true;
      draw();
      clearTimeout(wheelTimer);
      wheelTimer = setTimeout(commit, 140);
    }, {passive: false});
    viewport.addEventListener("keydown", (event) => {
      let factor = 0;
      if (event.key === "+" || event.key === "=") factor = 1.25;
      if (event.key === "-") factor = 0.8;
      if (!factor) return;
      event.preventDefault();
      transform.scale *= factor;
      dirty = true;
      draw();
      commit();
    });
    draw();
	if ("ResizeObserver" in window) {
	  const renderedWidth = number(viewport, "renderWidth", 0);
	  const renderedHeight = number(viewport, "renderHeight", 0);
	  const meaningfullyChanged = (actual, rendered) => !rendered ||
		(Math.abs(actual - rendered) > 127 && Math.abs(actual - rendered) / Math.max(1, rendered) > .15);
	  const scheduleResizeCommit = (rect) => {
		if (rect.width < 32 || rect.height < 32) return;
		if (!meaningfullyChanged(rect.width, renderedWidth) && !meaningfullyChanged(rect.height, renderedHeight)) return;
		clearTimeout(resizeTimer);
		resizeTimer = setTimeout(() => { dirty = true; commit(); }, 220);
	  };
	  new ResizeObserver((entries) => {
		scheduleResizeCommit(entries[0].contentRect);
	  }).observe(viewport);
	} else {
	  const renderedWidth = number(viewport, "renderWidth", 0);
	  const renderedHeight = number(viewport, "renderHeight", 0);
	  const scheduleResizeCommit = () => {
		const rect = viewport.getBoundingClientRect();
		const widthChanged = !renderedWidth || (Math.abs(rect.width - renderedWidth) > 127 && Math.abs(rect.width - renderedWidth) / Math.max(1, renderedWidth) > .15);
		const heightChanged = !renderedHeight || (Math.abs(rect.height - renderedHeight) > 127 && Math.abs(rect.height - renderedHeight) / Math.max(1, renderedHeight) > .15);
		if (rect.width < 32 || rect.height < 32 || (!widthChanged && !heightChanged)) return;
		clearTimeout(resizeTimer);
		resizeTimer = setTimeout(() => { dirty = true; commit(); }, 220);
	  };
	  addEventListener("resize", scheduleResizeCommit, {passive: true});
	  setTimeout(scheduleResizeCommit, 0);
	}
  };

  const init = () => document.querySelectorAll("[data-poem-image-viewport]").forEach(bind);
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
  else init();
})();
