Markdown# Technical Design Document: Multi-Agent Blackboard Architecture
**Target System:** Windows (Local Run)  
**Architecture Style:** Decentralized IPC / Sidecar Architecture  
**Technology Stack:** Go 1.2x+ (Orchestrator) & Rust (Systems Core / Sidecar)  
**Communication Protocol:** Cap'n Proto over Windows Named Pipes  

---

## 1. System Overview & Objectives
The system is designed to provide a frameworkless, deterministic environment for orchestrating multiple local and cloud LLM agents to perform repository scanning, analysis, and code modifications. 

To bypass memory and toolchain complications caused by embedding both runtimes inside a single binary via CGO, this system splits duties across two completely independent processes using an **Inter-Process Communication (IPC)** bridge.

### Core Component Split:
* **Go Orchestrator (Manager):** Handles user interaction, coordinates execution phases, maintains systemic state boundaries, manages asynchronous pooling to cloud endpoints, and controls the subprocess lifecycle.
* **Rust Sidecar (Compute Engine):** Handles file indexing, performance-critical static analysis, token budgeting constraints, and text pattern matching.

---

## 2. Process Topography & Windows Lifecycle Control

┌───────────────────────────────────┐               ┌───────────────────────────────────┐│          Go Orchestrator          │               │           Rust Sidecar            ││     - User Query Entry            │               │     - High-speed File Mapping     ││     - Cloud API Client Pool       │  Named Pipe   │     - Structural Analysis Core    ││     - Main State Controller       │ ◄───────────► │     - Context Size Calculators    ││     - Subprocess Lifecycle Manager│               │     - Local Data Serialization    │└─────────────────┬─────────────────┘               └───────────────────────────────────┘│▼ (Spawns & Monitors)┌───────────────────────────────────┐│         Rust Child Process        │└───────────────────────────────────┘
On Windows, the lifecycle of the distributed processes must be explicitly tied together to prevent rogue or orphaned background workers.

### Implementation Requirements:
* **Transport Engine:** Windows **Named Pipes** (e.g., `\\.\pipe\agent_council_socket`) will serve as the IPC mechanism, matching the performance profiles of Unix Domain Sockets by bypassing the network loopback stack completely.
* **Process Spawning:** Go will start the Rust executable using `os/exec.Command`.
* **Zombie Mitigation (Windows Job Objects):** Go will register the Rust child handle inside a Windows `Job Object` with the `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` flag applied. If the Go app crashes, is forced closed, or is killed via Task Manager, the Windows kernel will forcefully destroy the Rust background daemon immediately.

---

## 3. Cap'n Proto Data Contracts (`schema.capnp`)

This schema serves as the explicit binary layout mapped directly into the memory buffers of both languages without parsing penalties.

```capnp
@0xfa3b921f14bf241c; # Unique schema ID file generation flag

struct CodeChange {
  filePath    @0 :Text;
  targetFunc  @1 :Text;
  explanation @2 :Text;
}

struct GeneratedDiff {
  filePath    @0 :Text;
  unifiedDiff @1 :Text;
}

struct SystemState {
  repoPath          @0 :Text;
  userQuery         @1 :Text;
  targetFiles       @2 :List(Text);
  proposedChanges   @3 :List(CodeChange);
  generatedDiffs    @4 :List(GeneratedDiff);
  validationPassed  @5 :Bool;
}

interface ComputeService {
  scanRepository     @0 (query :Text, repoPath :Text) -> (files :List(Text));
  analyzeTargetFiles @1 (state :SystemState) -> (changes :List(CodeChange));
}
4. Finite State Machine & Orchestrator LogicThe Go system controller operates inside a strict, linear state loop. Agents are completely separated from each other—they never talk directly; instead, the Go manager extracts context from the State struct, passes it down the pipe to an execution block, reads the binary result, and applies the update.Core Execution Loop Matrix:StageInput RequirementsExecution EngineOutput Validation CheckFallback Action on Failure1. InitUser input string + pathPure Go CodeEnsure folder path path is valid on filesystem via os.Stat.Halt pipeline immediately. Abort run.2. Context DiscoveryUserQuery + RepoPathRust Sidecar (Via Named Pipe Map-Reduce)Verify output array length > 0. Ensure all returned files explicitly exist.Scrub non-existent files. If 0 remain, prompt user for more context.3. PlanningChecked file paths + queryTier 3 Agent Slot (Local or Cloud Switch)Parse plan map into strict Cap'n Proto definitions.Limit to 2 automatic retries with shifted model temperature parameters.4. GenerationValidated Planning ArrayTier 2/3 ConferenceCompile/Syntax validation check on resulting text patterns.Flag target failure; preserve previous state snapshot; notify operator.5. Architectural Red-Team MitigationsA. Windows Pathing and Serialization DiscrepanciesThe Threat: Local LLMs often output file locations formatting paths with Linux-style forward slashes (/src/db.go), whereas Windows system environments require backslashes (\src\db.go). This can cause native file tracking actions to fail silently.The Mitigation: The Go manager must run a sanitization pipeline over every Cap'n Proto packet containing file arrays, normalizing strings using Go's native filepath.Clean() function before running disk operations.B. Hardware Thrashing on Local ConferencesThe Threat: Spawning asynchronous map routines inside Go that prompt several local model engines simultaneously will trigger VRAM saturation, degrading local token generation speed or causing OOM states.The Mitigation: When configuring the target engine list inside the local system properties, any model targeted at local infrastructure (Ollama/llama.cpp) must be forced into a strictly sequential execution pipeline handled via a mutex locker or basic channels in Go. True concurrency is explicitly reserved for external network API instances.C. Leaky Abstraction Memory SafetyThe Threat: Even though Cap'n Proto offers zero-copy parsing speed, raw byte buffers passed improperly from Windows named pipes can introduce memory safety issues on the Rust boundary if the Go process drops or cleans the underlying tracking buffer mid-read.The Mitigation: The Rust application wrapper must process frames inside explicitly scoped lifetimes ('a) using capnp::message::ReaderSegments, ensuring memory allocations are fully pinned during data reading and dropped immediately after the native memory-copy pass concludes.

One Small Detail to Watch Out For
Your design document is incredibly tight, but there is one Windows-specific quirk you should watch out for during implementation in Section 5-C (Leaky Abstraction Memory Safety):

On Windows, named pipes can operate in two modes: Byte Stream mode or Message mode.

If you use standard Byte Stream mode (which is default for most basic pipe implementations), Cap'n Proto might read a partial frame if the network buffer gets split, causing a serialization panic.

The Fix: Ensure your Go orchestrator writes the data using Message mode (PIPE_TYPE_MESSAGE), or explicitly prefix every Cap'n Proto message with a 4-byte integer indicating the exact size of the payload so the Rust sidecar knows exactly how many bytes to wait for before passing the buffer to the Cap'n Proto reader segments.

but I mean if we should then swtich to rust and use Ipc with rust and go?

If you use the Process-Isolated IPC architecture over Windows Named Pipes, you can absolutely use Rust instead of C++ for the GUI engine core.

In fact, using the IPC sidecar model completely neutralizes Rust's biggest drawback regarding GUI development, making it an incredibly powerful setup.

Here is exactly how the calculus shifts if you build your GUI engine as a Rust IPC Sidecar paired with a Go Application Manager.

1. Why IPC Solves Rust's Biggest GUI Problem
Earlier, we discussed how Rust's strict memory rules (the borrow checker) make building a traditional UI widget tree incredibly frustrating because widgets naturally want to hold pointers to their parents and children simultaneously (cyclic references).

When you use an IPC architecture, your UI layer doesn't need to be highly stateful or deeply nested anymore. It becomes a reactive rendering engine.

The Reactive Rust Engine Pipeline
Instead of building a massive, interconnected object-oriented widget tree inside Rust, your Go application dictates the exact layout flatly over the pipe.

Go passes a linearized, flat array of elements via Cap'n Proto (e.g., a list of IDs, layout bounds, and text strings).

The Rust sidecar reads this flat data buffer.

Because Rust excels at data processing, it maps this flat list straight into an array-backed data structure (often called an Arena or Entity-Component System structure) with zero pointer overhead.

Rust loops through this flat array linearly, calculates layout box geometry, and pushes the vertices to the GPU.

By switching to a data-driven IPC model, you are no longer fighting Rust's borrow checker to maintain a massive stateful tree; you are leveraging Rust's ultimate strength—high-speed, linear array processing.

2. Pros of Using a Rust Sidecar for the GUI Engine
Modern, Elegant Tooling (cargo)
You get to completely skip the nightmare of C++ build systems, header files, and CMake. To handle cross-platform window management and graphics, you simply add native, production-grade Rust dependencies to your engine project:

winit: For raw, high-performance window creation and OS event pumping.

wgpu or vulkano: For cutting-edge, hardware-accelerated rendering pipelines.

Your engine build pipeline becomes just as clean and predictable as Go’s.

Ironclad Safety Over the Pipe
When reading raw binary data from an IPC pipe, C++ is prone to buffer overflows or memory corruption if you miscalculate packet lengths. Rust natively guarantees that reading from a data stream into raw memory slices is checked at compile-time. If the Go app sends malformed data, Rust will catch it safely without triggering an unmanaged segmentation fault.

3. Cons of Using a Rust Sidecar for the GUI Engine
The Frame Latency Tax
Because every single frame change or layout update requires serialization (Cap'n Proto) and a context switch across the Windows kernel via the named pipe, your frame-pumping pipeline must be highly optimized.

While Rust handles serialization blazingly fast, writing custom state management to ensure you only send diffs (changes) down the pipe—rather than re-sending the entire UI tree every single frame—requires careful architectural planning.

Architectural Inversion
You cannot just copy-paste standard C++ GUI engine tutorials. Most documentation for building UI pipelines from scratch is written with C++ object-oriented principles in mind. Using Rust forces you to translate those concepts into data-driven, index-based patterns.

C++ IPC vs. Rust IPC: The Final Choice
Since the Go layer remains exactly the same (handling your database, network, and high-level logic), your choice for the GUI sidecar comes down to this:

Choose C++ for the IPC Sidecar if: You want to write a classic, object-oriented GUI engine using traditional pointer-heavy widget trees, and you are comfortable debugging raw memory behavior to achieve it.

Choose Rust for the IPC Sidecar if: You want a clean, modern development toolchain (cargo), ironclad safety on your rendering engine, and you are excited to design a highly optimized, data-driven, reactive UI pipeline.

Given that you are already standardizing your multi-agent architecture around Go and a Rust sidecar via Cap'n Proto, using Rust for the GUI engine sidecar as well lets you reuse your entire IPC pipeline, schemas, and build workflows across all your projects.