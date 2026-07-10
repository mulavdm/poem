---
type: Concept
title: Cosine Spline Curve Telemetry (LineChart)
description: The per-pixel cosine interpolation used by LineChart to smooth telemetry data without an external rendering engine.
tags: [architecture, rendering, line-chart, math]
timestamp: 2026-07-10T00:00:00Z
---
# Cosine Spline Curve Telemetry (`LineChart`)

Traditional linear graphs produce jagged, unpolished vector segments. To solve this without adding external rendering engines, `LineChart` engineers per-pixel cosine interpolation spline equations directly:

$$y = y_1 \cdot (1 - t') + y_2 \cdot t'$$
$$t' = \frac{1 - \cos(t \cdot \pi)}{2}$$

This maps discrete heap memory telemetry arrays into smooth wave-like paths in microsecond execution times.

**Translucent Fills**: data visualizations may use restrained product-specific fills where they communicate information. Ordinary controls and app chrome use semantic theme tokens instead of glow or glass by default.

## Usage

```go
&render.LineChart{
    CompID:    "mem_heap_chart",
    Rect:      image.Rect(120, 400, 700, 650),
    BGColor:   color.RGBA{21, 24, 29, 255},
    LineColor: color.RGBA{91, 141, 239, 255},
    Data:      state.HeapHistory, // Slice of float32 telemetry
    Title:     "REALTIME HEAP MONITOR (MB)",
    Rounding:  10,
}
```

Charts are content visualizations, so product-specific colors are acceptable when they communicate data meaning. Avoid neon or glow as the default app chrome.

## See also
- [Architecture Overview](/concepts/architecture/overview.md)
- [Component Catalog](/concepts/architecture/component-catalog.md)
