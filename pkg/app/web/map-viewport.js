(() => {
  const assetVersion = new URL(document.currentScript.src).searchParams.get('v') || '';
  const assetURL = path => `${path}?v=${encodeURIComponent(assetVersion)}`;
  const decode = value => JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(value), c => c.charCodeAt(0))));
  const clamp = (value, min, max) => Math.max(min, Math.min(max, value));
  const world = (lon, lat) => [(lon + 180) / 360, (1 - Math.log(Math.tan(Math.PI / 4 + clamp(lat, -85.05112878, 85.05112878) * Math.PI / 360)) / Math.PI) / 2];
  const lonLat = (x, y) => [x * 360 - 180, Math.atan(Math.sinh(Math.PI * (1 - 2 * y))) * 180 / Math.PI];
  const rgba = value => {
	const match = /^#?([0-9a-f]{6})([0-9a-f]{2})?$/i.exec((value || '').trim());
	if (match) return {R: parseInt(match[1].slice(0, 2), 16), G: parseInt(match[1].slice(2, 4), 16), B: parseInt(match[1].slice(4, 6), 16), A: match[2] ? parseInt(match[2], 16) : 255};
	const numbers = /^rgba?\(/i.test(value || '') ? (value.match(/[\d.]+/g) || []).map(Number) : [];
	if (numbers.length >= 3) return {R: Math.round(numbers[0]), G: Math.round(numbers[1]), B: Math.round(numbers[2]), A: numbers.length > 3 ? Math.round(numbers[3] <= 1 ? numbers[3] * 255 : numbers[3]) : 255};
	return {R: 0, G: 0, B: 0, A: 0};
  };

  document.querySelectorAll('[data-poem-map-viewport]').forEach(root => {
    const canvas = root.querySelector('canvas');
    if (!canvas) return;
    let camera = decode(root.dataset.camera);
	const source = decode(root.dataset.source);
	const style = decode(root.dataset.style);
	const features = decode(root.dataset.features);
    const activeFeatureIndexes = new Set(decode(root.dataset.activeFeatures));
	const minZoom = Number(root.dataset.minZoom), maxZoom = Number(root.dataset.maxZoom);
	const minPitch = Number(root.dataset.minPitch), maxPitch = Number(root.dataset.maxPitch);
	const quality = Number(root.dataset.quality), tileMaximum = quality === 1 ? 24 : quality === 3 ? 64 : 48;
	const labelMaximum = quality === 1 ? 4096 : quality === 3 ? 16384 : 12288;
	const terrainDimension = quality === 1 ? 33 : quality === 3 ? 129 : 65;
	const locale = style.Locale || document.documentElement.lang || navigator.language || 'und';
	let pointer = null, wheelTimer = 0, keyTimer = 0, vectorWorker = null, vectorStarted = false, context = null, vectorPicks = [];
    const canVector = typeof Worker !== 'undefined' && typeof OffscreenCanvas !== 'undefined' && typeof canvas.transferControlToOffscreen === 'function' && typeof WebAssembly !== 'undefined';
    const setVectorStatus = (status, message = '') => {
      root.dataset.vectorStatus = status;
      root.setAttribute('aria-busy', status === 'loading' ? 'true' : 'false');
      if (message) root.dataset.vectorError = message;
      else delete root.dataset.vectorError;
    };
	setVectorStatus(canVector ? 'loading' : 'fallback');
	const computed = getComputedStyle(root);
	const tokenVariables = {surface: '--poem-surface', 'surface-muted': '--poem-surface-muted', sunken: '--poem-surface-muted', 'surface-raised': '--poem-surface-raised', raised: '--poem-surface-raised', text: '--poem-text', label: '--poem-text', muted: '--poem-muted', line: '--poem-line', border: '--poem-line', primary: '--poem-primary', accent: '--poem-primary', route: '--poem-primary', success: '--poem-success', marker: '--poem-success', warning: '--poem-warning', danger: '--poem-danger', traffic: '--poem-danger'};
	const roleVariables = {land: '--poem-surface-muted', water: '--poem-primary', road: '--poem-surface-raised', building: '--poem-line', label: '--poem-text', route: '--poem-primary', marker: '--poem-success', traffic: '--poem-danger'};
	const resolvedColor = (value, role) => {
	  if (/^#?[0-9a-f]{6}([0-9a-f]{2})?$/i.test((value || '').trim())) return rgba(value);
	  const variable = tokenVariables[(value || '').trim()] || roleVariables[role];
	  return rgba(computed.getPropertyValue(variable).trim());
	};
	const palette = {Land: resolvedColor(style.Colors && style.Colors.Land, 'land'), Water: resolvedColor(style.Colors && style.Colors.Water, 'water'), Waterway: resolvedColor(style.Colors && style.Colors.Water, 'water'), Road: resolvedColor(style.Colors && style.Colors.Road, 'road'), Building: resolvedColor(style.Colors && style.Colors.Building, 'building'), Label: resolvedColor(style.Colors && style.Colors.Label, 'label'), Route: resolvedColor(style.Colors && style.Colors.Route, 'route'), Marker: resolvedColor('marker', 'marker'), Traffic: resolvedColor(style.Colors && style.Colors.Traffic, 'traffic')};

    const project = position => {
      const center = world(camera.longitude, camera.latitude), point = world(position.Longitude, position.Latitude), scale = 512 * Math.pow(2, camera.zoom);
	  let deltaX = point[0] - center[0]; if (deltaX > .5) deltaX--; else if (deltaX < -.5) deltaX++;
	  const dx = deltaX * scale, dy = (point[1] - center[1]) * scale, angle = -camera.bearing * Math.PI / 180, pitch = camera.pitch * Math.PI / 180;
	  const localX = dx * Math.cos(angle) - dy * Math.sin(angle), localY = dx * Math.sin(angle) + dy * Math.cos(angle);
	  const projectedY = localY * Math.cos(pitch), depth = localY * Math.sin(pitch), distance = canvas.clientHeight * .5 / Math.tan(Math.PI / 8);
	  const perspective = distance / Math.max(distance * .05, distance - depth);
	  return [canvas.clientWidth / 2 + localX * perspective, canvas.clientHeight / 2 + projectedY * perspective];
    };
    const segmentDistance = (point, a, b) => {
      const dx = b[0] - a[0], dy = b[1] - a[1], lengthSquared = dx * dx + dy * dy;
      if (!lengthSquared) return Math.hypot(point[0] - a[0], point[1] - a[1]);
      const t = clamp(((point[0] - a[0]) * dx + (point[1] - a[1]) * dy) / lengthSquared, 0, 1);
      return Math.hypot(point[0] - (a[0] + t * dx), point[1] - (a[1] + t * dy));
    };
    const pointInPolygon = (point, polygon) => {
      let inside = false;
      for (let index = 0, previous = polygon.length - 1; index < polygon.length; previous = index++) {
        const a = polygon[index], b = polygon[previous];
        if ((a[1] > point[1]) !== (b[1] > point[1]) && point[0] < (b[0] - a[0]) * (point[1] - a[1]) / (b[1] - a[1]) + a[0]) inside = !inside;
      }
      return inside;
    };
    const featureAt = point => {
	  for (let index = features.length - 1; index >= 0; index--) {
        if (!activeFeatureIndexes.has(index)) continue;
        const feature = features[index], projected = feature.Positions.map(project);
        if (feature.Geometry === 0 && projected[0] && Math.hypot(point[0] - projected[0][0], point[1] - projected[0][1]) <= 18) return {index, feature};
        if (feature.Geometry === 1) for (let segment = 1; segment < projected.length; segment++) if (segmentDistance(point, projected[segment - 1], projected[segment]) <= (feature.Selected ? 12 : 9)) return {index, feature};
        if (feature.Geometry === 2 && projected.length >= 3 && pointInPolygon(point, projected)) return {index, feature};
	  }
	  for (let index = vectorPicks.length - 1; index >= 0; index--) {
		const pick = vectorPicks[index], projected = project({Longitude: pick.Longitude, Latitude: pick.Latitude});
		if (Math.hypot(point[0] - projected[0], point[1] - projected[1]) <= 18) return {vector: pick};
	  }
	  return null;
    };
    const color = role => {
      const styles = getComputedStyle(root);
      if (role === 1) return styles.getPropertyValue('--poem-map-route').trim() || styles.getPropertyValue('--poem-primary');
      if (role === 4) return styles.getPropertyValue('--poem-map-traffic').trim() || styles.getPropertyValue('--poem-warning');
      return styles.getPropertyValue('--poem-text').trim() || '#222';
    };
    const fallbackDraw = () => {
      if (!context) return;
      context.clearRect(0, 0, canvas.clientWidth, canvas.clientHeight);
      for (const feature of features) {
        const points = feature.Positions.map(project); if (!points.length) continue;
        context.beginPath();
        if (feature.Geometry === 0) { context.arc(points[0][0], points[0][1], feature.Selected ? 9 : 7, 0, Math.PI * 2); context.fillStyle = color(feature.Role); context.fill(); }
        else { context.moveTo(points[0][0], points[0][1]); for (let index = 1; index < points.length; index++) context.lineTo(points[index][0], points[index][1]); if (feature.Geometry === 2) { context.closePath(); context.globalAlpha = .35; context.fillStyle = color(feature.Role); context.fill(); context.globalAlpha = 1; } context.lineJoin = 'round'; context.lineCap = 'round'; context.lineWidth = feature.Selected ? 7 : 4; context.strokeStyle = color(feature.Role); context.stroke(); }
      }
    };
    const draw = () => { if (vectorWorker) vectorWorker.postMessage({type: 'camera', camera}); else fallbackDraw(); };
    const startVector = (width, height, logicalWidth, logicalHeight, ratio) => {
      if (!canVector || vectorStarted) return;
      vectorStarted = true;
      try {
        vectorWorker = new Worker(assetURL('/__poem/map-worker.js'));
        vectorWorker.addEventListener('message', event => {
          if (event.data.type === 'failure') setVectorStatus('error', event.data.message || 'Vector map unavailable');
		  if (event.data.type === 'loaded') { vectorPicks = Array.isArray(event.data.picks) ? event.data.picks : []; setVectorStatus('ready'); }
        });
        vectorWorker.addEventListener('error', event => setVectorStatus('error', event.message || 'Vector worker failed'));
        const offscreen = canvas.transferControlToOffscreen();
		vectorWorker.postMessage({type: 'init', canvas: offscreen, width, height, ratio, config: {viewportID: root.dataset.viewportId, source, camera, features, palette, buildingHeights: !!style.BuildingHeights, locale, poiFilters: Number(style.POIFilters || 0), labelDensity: Number(style.LabelDensity || 0), labelMaximum, terrainScale: Number(style.TerrainScale || 0), terrainDimension, width: logicalWidth, height: logicalHeight, maximum: tileMaximum}}, [offscreen]);
      } catch (error) { setVectorStatus('error', String(error)); vectorWorker = null; }
    };
    const resize = () => {
      const pixelRatio = window.devicePixelRatio || 1, logicalWidth = Math.max(1, canvas.clientWidth), logicalHeight = Math.max(1, canvas.clientHeight);
      const width = Math.max(1, Math.round(logicalWidth * pixelRatio)), height = Math.max(1, Math.round(logicalHeight * pixelRatio));
      startVector(width, height, logicalWidth, logicalHeight, pixelRatio);
      if (vectorWorker) vectorWorker.postMessage({type: 'resize', width, height, logicalWidth, logicalHeight, ratio: pixelRatio});
      else { if (!context) context = canvas.getContext('2d'); if (canvas.width !== width || canvas.height !== height) { canvas.width = width; canvas.height = height; } context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0); fallbackDraw(); }
    };
    const settleVector = () => { if (vectorWorker) vectorWorker.postMessage({type: 'settle', camera}); };
    const submitField = (field, value) => {
      const csrf = document.querySelector('input[name="trellis_csrf"]');
      if (!csrf || !field) return false;
      const body = new URLSearchParams({trellis_csrf: csrf.value, trellis_field: field, trellis_value: value});
      fetch('/__field', {method: 'POST', headers: {'Content-Type': 'application/x-www-form-urlencoded'}, body, credentials: 'same-origin'}).then(response => { if (response.redirected) window.location.assign(response.url); });
      return true;
    };
    const commit = () => {
      settleVector();
      submitField(root.dataset.field, JSON.stringify(camera));
    };
    canvas.addEventListener('pointerdown', event => { if (root.dataset.disabled === 'true') return; canvas.setPointerCapture(event.pointerId); pointer = {id: event.pointerId, x: event.clientX, y: event.clientY, camera: {...camera}, rotate: event.shiftKey, moved: false}; });
    canvas.addEventListener('pointermove', event => {
      if (!pointer || event.pointerId !== pointer.id) return;
      const dx = event.clientX - pointer.x, dy = event.clientY - pointer.y;
      if (Math.hypot(dx, dy) > 4) pointer.moved = true;
      if (pointer.rotate) { camera.bearing = ((pointer.camera.bearing + dx * .4) % 360 + 360) % 360; camera.pitch = clamp(pointer.camera.pitch - dy * .25, minPitch, maxPitch); }
      else { const center = world(pointer.camera.longitude, pointer.camera.latitude), scale = 512 * Math.pow(2, pointer.camera.zoom), value = lonLat(center[0] - dx / scale, center[1] - dy / scale); camera.longitude = clamp(value[0], -180, 180); camera.latitude = clamp(value[1], -85.05112878, 85.05112878); }
      draw();
    });
    const endPointer = event => {
      if (!pointer || event.pointerId !== pointer.id) return;
      const interaction = pointer; pointer = null;
      if (!interaction.moved) {
        const rect = canvas.getBoundingClientRect(), hit = featureAt([event.clientX - rect.left, event.clientY - rect.top]);
		if (hit && hit.vector && submitField(`${root.dataset.field}/vector-feature`, JSON.stringify({id: hit.vector.ID, name: hit.vector.Name || '', description: hit.vector.Description || '', category: hit.vector.Category || '', source_layer: hit.vector.SourceLayer, latitude: hit.vector.Latitude, longitude: hit.vector.Longitude}))) return;
		if (hit && hit.feature && submitField(`${root.dataset.field}/feature-${hit.index}`, hit.feature.ID)) return;
      }
      commit();
    };
    canvas.addEventListener('pointerup', endPointer); canvas.addEventListener('pointercancel', endPointer);
    canvas.addEventListener('wheel', event => { if (root.dataset.disabled === 'true') return; event.preventDefault(); camera.zoom = clamp(camera.zoom - event.deltaY * .002, minZoom, maxZoom); draw(); clearTimeout(wheelTimer); wheelTimer = setTimeout(commit, 180); }, {passive: false});
    canvas.addEventListener('keydown', event => {
      if (root.dataset.disabled === 'true') return;
      const center = world(camera.longitude, camera.latitude), step = 96 / (512 * Math.pow(2, camera.zoom));
      let handled = true;
      switch (event.key) {
        case 'ArrowLeft': { const value = lonLat(center[0] - step, center[1]); camera.longitude = clamp(value[0], -180, 180); break; }
        case 'ArrowRight': { const value = lonLat(center[0] + step, center[1]); camera.longitude = clamp(value[0], -180, 180); break; }
        case 'ArrowUp': { const value = lonLat(center[0], center[1] - step); camera.latitude = clamp(value[1], -85.05112878, 85.05112878); break; }
        case 'ArrowDown': { const value = lonLat(center[0], center[1] + step); camera.latitude = clamp(value[1], -85.05112878, 85.05112878); break; }
        case '+': case '=': camera.zoom = clamp(camera.zoom + .5, minZoom, maxZoom); break;
        case '-': case '_': camera.zoom = clamp(camera.zoom - .5, minZoom, maxZoom); break;
        case 'q': case 'Q': camera.bearing = ((camera.bearing - 10) % 360 + 360) % 360; break;
        case 'e': case 'E': camera.bearing = (camera.bearing + 10) % 360; break;
        case 'PageUp': camera.pitch = clamp(camera.pitch + 5, minPitch, maxPitch); break;
        case 'PageDown': camera.pitch = clamp(camera.pitch - 5, minPitch, maxPitch); break;
        case 'Home': camera.bearing = 0; camera.pitch = 0; break;
        default: handled = false;
      }
      if (!handled) return;
      event.preventDefault(); draw(); clearTimeout(keyTimer); keyTimer = setTimeout(commit, 140);
    });
    window.addEventListener('pagehide', () => { if (vectorWorker) vectorWorker.postMessage({type: 'dispose'}); }, {once: true});
    new ResizeObserver(resize).observe(root); resize();
  });
})();
