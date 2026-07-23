# Recorded baselines

Reference measurements for the scenes in `scenes/`, written by:

```bash
go run ./cmd/poemdrive -save-baseline scenes/baselines/<scene>.json scenes/<scene>.json
```

and compared against with `-baseline`.

**These are environment-specific.** A baseline recorded on one emulator, GPU, or
machine is not a threshold another machine must meet — it is the reference for
detecting that *this* environment got worse. Re-record per environment before
gating on it, and treat a diff here as a deliberate act with a reason in the
commit message, not as noise to be refreshed away.

`android-preferences.json` was recorded on the `Medium_Phone` AVD (API 36,
x86_64, 420 dpi) on 2026-07-23.

The gallery and MAPPS scenes have no committed baseline yet. The figures in
`docs/M0_performance_baselines.md` predate this tool and are not comparable to
its output: they were reconstructed by polling `/perf/events` during the run,
which added load that the server-side percentiles do not.
