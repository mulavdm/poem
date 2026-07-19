package app

import "time"

// QualityGovernor resolves the effective map quality tier for MapQualityAuto by
// adapting to measured frame time within a structural ceiling derived from GPU
// capabilities. Explicit tiers are honored (still capped by the ceiling); only
// Auto adapts. Adaptation uses an EMA of frame time plus dwell hysteresis so the
// tier does not oscillate. This is the "Auto selects and adjusts from GPU
// capabilities and measured frame time" behavior, with a downgrade-before-
// instability bias.
type QualityGovernor struct {
	requested MapQuality
	ceiling   MapQuality
	budgetMS  float64
	effective MapQuality
	ema       float64
	dwell     int
	started   bool
}

// minGovernorDwell is how many observed frames must pass between tier changes.
const minGovernorDwell = 45

// NewQualityGovernor creates a governor. ceiling is the highest tier the GPU can
// structurally sustain (see QualityCeiling); it is clamped into
// [BatterySaver, High]. targetFPS defaults to 60 when non-positive.
func NewQualityGovernor(requested, ceiling MapQuality, targetFPS float64) *QualityGovernor {
	if targetFPS <= 0 {
		targetFPS = 60
	}
	governor := &QualityGovernor{
		requested: requested,
		ceiling:   clampTier(ceiling, MapQualityHigh),
		budgetMS:  1000.0 / targetFPS,
	}
	governor.effective = governor.initial()
	return governor
}

func (g *QualityGovernor) initial() MapQuality {
	if g.requested != MapQualityAuto {
		return minTier(g.requested, g.ceiling)
	}
	// Auto starts conservatively at Balanced (or lower if the GPU caps below it).
	return minTier(MapQualityBalanced, g.ceiling)
}

// Effective is the current resolved tier (never MapQualityAuto).
func (g *QualityGovernor) Effective() MapQuality { return g.effective }

// SmoothedFrameMillis exposes the EMA frame time for diagnostics.
func (g *QualityGovernor) SmoothedFrameMillis() float64 { return g.ema }

// Observe records one frame's render time and returns the resolved tier. For an
// explicit request it returns that tier capped by the ceiling; for Auto it steps
// one tier at a time with hysteresis: down when frames run over budget, up when
// they run comfortably under it and the ceiling allows.
func (g *QualityGovernor) Observe(frame time.Duration) MapQuality {
	ms := float64(frame) / float64(time.Millisecond)
	if ms < 0 {
		ms = 0
	}
	if !g.started {
		g.ema, g.started = ms, true
	} else {
		g.ema = 0.8*g.ema + 0.2*ms
	}

	if g.requested != MapQualityAuto {
		g.effective = minTier(g.requested, g.ceiling)
		return g.effective
	}

	g.dwell++
	if g.dwell < minGovernorDwell {
		return g.effective
	}
	switch {
	case g.ema > g.budgetMS*1.2 && g.effective > MapQualityBatterySaver:
		g.effective--
		g.dwell = 0
	case g.ema < g.budgetMS*0.6 && g.effective < g.ceiling:
		g.effective++
		g.dwell = 0
	}
	return g.effective
}

// QualityCeiling derives the highest structurally sustainable tier from GPU
// capabilities: no retained meshes means the raster fallback (Battery Saver);
// a small texture budget or tight memory caps at Balanced; otherwise High.
func QualityCeiling(retainedMeshes bool, maxTextureSize int, memoryBudget int64) MapQuality {
	if !retainedMeshes {
		return MapQualityBatterySaver
	}
	if maxTextureSize < 4096 || (memoryBudget > 0 && memoryBudget < 512<<20) {
		return MapQualityBalanced
	}
	return MapQualityHigh
}

func clampTier(tier, fallback MapQuality) MapQuality {
	if tier < MapQualityBatterySaver || tier > MapQualityHigh {
		return fallback
	}
	return tier
}

func minTier(a, b MapQuality) MapQuality {
	if a < b {
		return a
	}
	return b
}
