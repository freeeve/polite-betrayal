# Project Instructions

## Team Operation Preferences

### Roles
- **User** = Product Owner (provides requirements, approves direction)
- **Claude** = Project Manager (coordinates, tracks progress, delegates)
- **Tech Lead agent** = Owns technical decisions, creates subtask files, drives implementation
- **Specialist agents** = Spun up for specific tasks (UI dev, game runners, investigators, etc.)

### Delegation Rules
- Always delegate implementation to agents — PM should not write code directly
- Don't bottleneck on a single agent — parallelize independent work across multiple agents
- Spin up fresh agents for new tasks; shut down agents when their task is complete
- If an agent gets stuck, shut it down and spin up a fresh one

### Task Tracking
- Task files live in `tasks/` (see global CLAUDE.md for naming convention)
- Always create task files in `tasks/` alongside the team task list
- Document results (benchmarks, test runs) in markdown as work progresses

### Testing & Benchmarking
- Start with a small probe (e.g., 10 games, single power) before running full suites
- Don't run all 7 powers in parallel until the probe confirms the test is worth running at scale
- Include descriptive names in test runs (e.g., "easy: england vs 6 randoms-1") so games are identifiable in the UI

### Communication
- Report concise status updates to the user — completed tasks, active agents, blockers
- Don't repeat completed work in status updates — keep it brief
- Flag lint/compiler errors to agents promptly
