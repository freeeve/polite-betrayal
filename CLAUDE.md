# Project Instructions

## Team Operation Preferences

### Roles
- **User** = Product Owner (provides requirements, approves direction)
- **Claude** = Project Manager (coordinates, tracks progress, delegates)
- **Specialist agents** = Spun up for specific tasks (UI dev, Rust dev, game runners, investigators, etc.)

### Project Architecture
- Go backend: `api/` (game engine in `api/pkg/diplomacy/`, services in `api/internal/`, bots in `api/internal/bot/`)
- Flutter UI: `ui/` (Riverpod state, go_router, SVG map with CustomPainter overlay)
- Rust engine: `engine/` (DUI protocol, resolver, eval — being ported from Go)
- Key gotchas: GameState JSON uses PascalCase, UnitType is int (0=Army, 1=Fleet), split-coast provinces (spa nc/sc, stp nc/sc, bul ec/sc), resolver uses optimistic initial guess

### Delegation Rules
- Always delegate implementation to agents — PM should not write code directly
- Delegate ALL work including test runs, investigations, and commits to conserve main context
- Don't bottleneck on a single agent — parallelize independent work across multiple agents
- Spin up fresh agents for new tasks; shut down agents when their task is complete
- If an agent gets stuck, shut it down and spin up a fresh one
- Proactively spin up agents for newly unblocked tasks WITHOUT asking — check task dependency chains after each task completes and immediately launch agents for anything that's ready
- NEVER ask "want me to spin up X?" or "should I start Y?" — just do it

### Task Completion Requirements
- Every agent MUST run tests and commit before marking a task complete
- Run `gofmt -s` on Go files, `cargo fmt` on Rust files, `flutter analyze` on Dart files before committing
- Use semantic commit messages with module paths. Examples:
  - `feat(api/pkg/diplomacy): add convoy path validation`
  - `fix(api/internal/bot): scale SC defense penalty by empire size`
  - `feat(engine): implement Kruijswijk guess-and-check order resolver`
  - `fix(ui): fix replay freeze on build and empty retreat phases`
  - `feat(api/internal/bot): add easy vs random all-powers benchmark test`
  - `chore: add task tracking and project docs`
- Always use HEREDOC format for commit messages
- Stage specific files, never `git add -A` or `git add .`
- NEVER use `git stash` — parallel agents will conflict with each other's stashes

### Task Tracking
- Task files live in `tasks/` (see global CLAUDE.md for naming convention)
- Always create task files in `tasks/` alongside the team task list
- Document results (benchmarks, test runs) in markdown as work progresses

### Testing & Benchmarking
- Include descriptive names in test runs (e.g., "bench-easy-england-vs-random") so games are identifiable in the UI
- For DB benchmarks, use `DryRun: false` so games are reviewable in the UI
- Store benchmark results in `benchmarks/` (not `tasks/`). Use descriptive filenames like `easy-vs-random-2026-02-17.md`
- Include SC timeline stats (avg/min/p25/p50/p75/p95/max per year) when running arena benchmarks
- Never use BENCH_VERBOSE=1 when running arena benchmarks in agents — it wastes context tokens. Default quiet mode only prints summaries.

### Communication
- Report concise status updates to the user — completed tasks, active agents, blockers
- Don't repeat completed work in status updates — keep it brief
- Flag lint/compiler errors to agents promptly
