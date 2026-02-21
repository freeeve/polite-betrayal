# Fix Adjacency Types to Match Standard Diplomacy Rules

## Status: In Progress

## Description
Audit found 10 coastal-to-coastal adjacencies typed as `both` that should be `army-only` per standard rules (provinces share a land border but face different bodies of water, so no fleet can traverse). Also 1 missing adjacency (arm-syr) in all codebases, 4 additional missing in Rust, and 1 incorrect Rust test.

## Changes Required

### Type fixes (both → army-only): ank-smy, arm-smy, apu-rom, edi-lvp, fin-nwy, lvp-yor, pie-ven, rom-ven, tus-ven, wal-yor

### Missing adjacencies:
- arm-syr (all three codebases)
- ank-smy, apu-rom, lvp-yor, tus-ven (Rust only — as army-only)

### Incorrect test:
- Rust: `smyrna_ankara_not_adjacent` should only assert fleet non-adjacency

## Affected Files
- `api/pkg/diplomacy/map_data.go`
- `engine/src/board/adjacency.rs`
- `ui/lib/core/map/adjacency_data.dart`
