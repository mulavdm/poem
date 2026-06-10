package render

import (
	"fmt"
	"sync"
	"time"
)

const maxPerfEvents = 256

type PerfEvent struct {
	Kind       string  `json:"kind"`
	Name       string  `json:"name"`
	Timestamp  string  `json:"timestamp"`
	DurationMS float64 `json:"duration_ms"`
	Details    string  `json:"details,omitempty"`
}

type PerfFrameStats struct {
	FrameCount           uint64  `json:"frame_count"`
	LastTotalMS          float64 `json:"last_total_ms"`
	LastBuildPagesMS     float64 `json:"last_build_pages_ms"`
	LastRenderPipelineMS float64 `json:"last_render_pipeline_ms"`
	LastSerializeMS      float64 `json:"last_serialize_ms"`
	LastWriteMS          float64 `json:"last_write_ms"`
	MaxTotalMS           float64 `json:"max_total_ms"`
	AvgTotalMS           float64 `json:"avg_total_ms"`
	AvgBuildPagesMS      float64 `json:"avg_build_pages_ms"`
	AvgRenderPipelineMS  float64 `json:"avg_render_pipeline_ms"`
	AvgSerializeMS       float64 `json:"avg_serialize_ms"`
	AvgWriteMS           float64 `json:"avg_write_ms"`
}

type PerfEventBatchStats struct {
	BatchCount         uint64  `json:"batch_count"`
	LastBatchMS        float64 `json:"last_batch_ms"`
	LastBatchEvents    int     `json:"last_batch_events"`
	MaxBatchMS         float64 `json:"max_batch_ms"`
	AvgBatchMS         float64 `json:"avg_batch_ms"`
	ResizeEventCount   uint64  `json:"resize_event_count"`
	MouseEventCount    uint64  `json:"mouse_event_count"`
	KeyboardEventCount uint64  `json:"keyboard_event_count"`
}

type PerfAutomationStats struct {
	ActionCount    uint64  `json:"action_count"`
	LastActionMS   float64 `json:"last_action_ms"`
	MaxActionMS    float64 `json:"max_action_ms"`
	AvgActionMS    float64 `json:"avg_action_ms"`
	LastActionName string  `json:"last_action_name,omitempty"`
}

type PerfState struct {
	Frames       PerfFrameStats      `json:"frames"`
	EventBatches PerfEventBatchStats `json:"event_batches"`
	Automation   PerfAutomationStats `json:"automation"`
	Events       []PerfEvent         `json:"events"`
}

type perfTracker struct {
	mu sync.Mutex

	frameCount        uint64
	frameTotalMS      float64
	frameBuildPagesMS float64
	frameRenderMS     float64
	frameSerializeMS  float64
	frameWriteMS      float64
	lastFrameTotalMS  float64
	lastBuildPagesMS  float64
	lastRenderMS      float64
	lastSerializeMS   float64
	lastWriteMS       float64
	maxFrameTotalMS   float64

	batchCount         uint64
	batchTotalMS       float64
	lastBatchMS        float64
	lastBatchEvents    int
	maxBatchMS         float64
	resizeEventCount   uint64
	mouseEventCount    uint64
	keyboardEventCount uint64

	actionCount    uint64
	actionTotalMS  float64
	lastActionMS   float64
	maxActionMS    float64
	lastActionName string

	events []PerfEvent
}

var globalPerfTracker = &perfTracker{}

func (p *perfTracker) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.frameCount = 0
	p.frameTotalMS = 0
	p.frameBuildPagesMS = 0
	p.frameRenderMS = 0
	p.frameSerializeMS = 0
	p.frameWriteMS = 0
	p.lastFrameTotalMS = 0
	p.lastBuildPagesMS = 0
	p.lastRenderMS = 0
	p.lastSerializeMS = 0
	p.lastWriteMS = 0
	p.maxFrameTotalMS = 0

	p.batchCount = 0
	p.batchTotalMS = 0
	p.lastBatchMS = 0
	p.lastBatchEvents = 0
	p.maxBatchMS = 0
	p.resizeEventCount = 0
	p.mouseEventCount = 0
	p.keyboardEventCount = 0

	p.actionCount = 0
	p.actionTotalMS = 0
	p.lastActionMS = 0
	p.maxActionMS = 0
	p.lastActionName = ""

	p.events = nil
}

func (p *perfTracker) recordFrame(buildPages, renderPipeline, serialize, write, total time.Duration, commandCount int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.frameCount++
	p.lastBuildPagesMS = durationMS(buildPages)
	p.lastRenderMS = durationMS(renderPipeline)
	p.lastSerializeMS = durationMS(serialize)
	p.lastWriteMS = durationMS(write)
	p.lastFrameTotalMS = durationMS(total)
	p.frameBuildPagesMS += p.lastBuildPagesMS
	p.frameRenderMS += p.lastRenderMS
	p.frameSerializeMS += p.lastSerializeMS
	p.frameWriteMS += p.lastWriteMS
	p.frameTotalMS += p.lastFrameTotalMS
	if p.lastFrameTotalMS > p.maxFrameTotalMS {
		p.maxFrameTotalMS = p.lastFrameTotalMS
	}
	p.appendEventLocked(PerfEvent{
		Kind:       "frame",
		Name:       "repaint",
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		DurationMS: p.lastFrameTotalMS,
		Details:    fmt.Sprintf("commands=%d build=%.2f render=%.2f serialize=%.2f write=%.2f", commandCount, p.lastBuildPagesMS, p.lastRenderMS, p.lastSerializeMS, p.lastWriteMS),
	})
}

func (p *perfTracker) recordEventBatch(batchSize int, total time.Duration, resizeEvents, mouseEvents, keyboardEvents int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.batchCount++
	p.lastBatchMS = durationMS(total)
	p.lastBatchEvents = batchSize
	p.batchTotalMS += p.lastBatchMS
	if p.lastBatchMS > p.maxBatchMS {
		p.maxBatchMS = p.lastBatchMS
	}
	p.resizeEventCount += uint64(resizeEvents)
	p.mouseEventCount += uint64(mouseEvents)
	p.keyboardEventCount += uint64(keyboardEvents)
	p.appendEventLocked(PerfEvent{
		Kind:       "event_batch",
		Name:       "input",
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		DurationMS: p.lastBatchMS,
		Details:    fmt.Sprintf("events=%d resize=%d mouse=%d keyboard=%d", batchSize, resizeEvents, mouseEvents, keyboardEvents),
	})
}

func (p *perfTracker) recordAutomationAction(name string, total time.Duration, details string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.actionCount++
	p.lastActionName = name
	p.lastActionMS = durationMS(total)
	p.actionTotalMS += p.lastActionMS
	if p.lastActionMS > p.maxActionMS {
		p.maxActionMS = p.lastActionMS
	}
	p.appendEventLocked(PerfEvent{
		Kind:       "automation",
		Name:       name,
		Timestamp:  time.Now().Format(time.RFC3339Nano),
		DurationMS: p.lastActionMS,
		Details:    details,
	})
}

func (p *perfTracker) snapshot() PerfState {
	p.mu.Lock()
	defer p.mu.Unlock()

	state := PerfState{
		Frames: PerfFrameStats{
			FrameCount:           p.frameCount,
			LastTotalMS:          p.lastFrameTotalMS,
			LastBuildPagesMS:     p.lastBuildPagesMS,
			LastRenderPipelineMS: p.lastRenderMS,
			LastSerializeMS:      p.lastSerializeMS,
			LastWriteMS:          p.lastWriteMS,
			MaxTotalMS:           p.maxFrameTotalMS,
		},
		EventBatches: PerfEventBatchStats{
			BatchCount:         p.batchCount,
			LastBatchMS:        p.lastBatchMS,
			LastBatchEvents:    p.lastBatchEvents,
			MaxBatchMS:         p.maxBatchMS,
			ResizeEventCount:   p.resizeEventCount,
			MouseEventCount:    p.mouseEventCount,
			KeyboardEventCount: p.keyboardEventCount,
		},
		Automation: PerfAutomationStats{
			ActionCount:    p.actionCount,
			LastActionMS:   p.lastActionMS,
			MaxActionMS:    p.maxActionMS,
			AvgActionMS:    average(p.actionTotalMS, p.actionCount),
			LastActionName: p.lastActionName,
		},
		Events: append([]PerfEvent(nil), p.events...),
	}
	state.Frames.AvgTotalMS = average(p.frameTotalMS, p.frameCount)
	state.Frames.AvgBuildPagesMS = average(p.frameBuildPagesMS, p.frameCount)
	state.Frames.AvgRenderPipelineMS = average(p.frameRenderMS, p.frameCount)
	state.Frames.AvgSerializeMS = average(p.frameSerializeMS, p.frameCount)
	state.Frames.AvgWriteMS = average(p.frameWriteMS, p.frameCount)
	state.EventBatches.AvgBatchMS = average(p.batchTotalMS, p.batchCount)
	return state
}

func (p *perfTracker) appendEventLocked(event PerfEvent) {
	p.events = append(p.events, event)
	if len(p.events) > maxPerfEvents {
		p.events = p.events[len(p.events)-maxPerfEvents:]
	}
}

func durationMS(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1_000_000.0
}

func average(total float64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return total / float64(count)
}
