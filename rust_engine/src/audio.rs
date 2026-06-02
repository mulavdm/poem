use rodio::{buffer::SamplesBuffer, OutputStreamHandle};
use std::f32::consts::PI;

pub struct AudioEngine {
    stream_handle: Option<OutputStreamHandle>,
    hover_buffer: Vec<f32>,
    click_buffer: Vec<f32>,
    success_buffer: Vec<f32>,
}

impl AudioEngine {
    pub fn new(handle: Option<OutputStreamHandle>) -> Self {
        let hover = Self::synthesize_hover();
        let click = Self::synthesize_click();
        let success = Self::synthesize_success();
        Self {
            stream_handle: handle,
            hover_buffer: hover,
            click_buffer: click,
            success_buffer: success,
        }
    }

    pub fn play_hover(&self) {
        if let Some(ref handle) = self.stream_handle {
            let buf = SamplesBuffer::new(1, 44100, self.hover_buffer.clone());
            let _ = handle.play_raw(buf);
        }
    }

    pub fn play_click(&self) {
        if let Some(ref handle) = self.stream_handle {
            let buf = SamplesBuffer::new(1, 44100, self.click_buffer.clone());
            let _ = handle.play_raw(buf);
        }
    }

    pub fn play_success(&self) {
        if let Some(ref handle) = self.stream_handle {
            let buf = SamplesBuffer::new(1, 44100, self.success_buffer.clone());
            let _ = handle.play_raw(buf);
        }
    }

    fn synthesize_hover() -> Vec<f32> {
        let sample_rate = 44100;
        let duration = 0.015;
        let num_samples = (sample_rate as f32 * duration) as usize;
        let mut samples = Vec::with_capacity(num_samples);
        for i in 0..num_samples {
            let t = i as f32 / sample_rate as f32;
            let envelope = (-t * 220.0).exp();
            let val = (2.0 * PI * 1200.0 * t).sin();
            // Blended f32 amplitude centered around 0.0 (volume scaled)
            samples.push(val * 0.15 * envelope);
        }
        samples
    }

    fn synthesize_click() -> Vec<f32> {
        let sample_rate = 44100;
        let duration = 0.12;
        let num_samples = (sample_rate as f32 * duration) as usize;
        let mut samples = Vec::with_capacity(num_samples);
        for i in 0..num_samples {
            let t = i as f32 / sample_rate as f32;
            let envelope = (-t * 28.0).exp();
            let val1 = (2.0 * PI * 900.0 * t).sin();
            let val2 = (2.0 * PI * 1800.0 * t).sin() * 0.4;
            let sample_val = (val1 + val2) / 1.4;
            samples.push(sample_val * 0.35 * envelope);
        }
        samples
    }

    fn synthesize_success() -> Vec<f32> {
        let sample_rate = 44100;
        let note_freqs = vec![659.25, 880.0, 1109.73, 1318.51];
        let note_duration = 0.10;
        let total_duration = note_duration * note_freqs.len() as f32;
        let num_samples = (sample_rate as f32 * total_duration) as usize;
        let mut samples = Vec::with_capacity(num_samples);
        for i in 0..num_samples {
            let t = i as f32 / sample_rate as f32;
            let mut note_idx = (t / note_duration) as usize;
            if note_idx >= note_freqs.len() {
                note_idx = note_freqs.len() - 1;
            }
            let freq = note_freqs[note_idx];
            let note_t = t - note_idx as f32 * note_duration;
            let envelope = (-note_t * 18.0).exp();
            let val = (2.0 * PI * freq * t).sin();
            samples.push(val * 0.3 * envelope);
        }
        samples
    }
}
