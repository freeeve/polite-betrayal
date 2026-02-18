# Refactor Opening Book Infrastructure

## Status: In Progress

## Dependencies
- None (can proceed in parallel with data extraction)

## Description
Refactor opening_book.go to support embedded JSON data with feature-based matching for years beyond 1901.

### Changes
1. Replace hardcoded Go structs with `//go:embed opening_book.json`
2. Add tiered matching strategy:
   - 1901: exact unit positions (current behavior)
   - 1902-1903: SC ownership + key province control
   - 1904-1907: SC count + theater presence + fleet/army ratio
3. Add build/disband phase support
4. Remove year 1901 gate — work for any year with matching data
5. Maintain backward compatibility

### Data Format
```json
{
  "entries": [
    {
      "power": "france",
      "year": 1902,
      "season": "spring",
      "phase": "movement",
      "condition": {
        "exact_positions": null,
        "owned_scs": ["spa", "por", "bel"],
        "controls": ["eng"],
        "sc_count_min": 5,
        "sc_count_max": 7
      },
      "options": [
        {"name": "...", "weight": 0.45, "orders": [...]}
      ]
    }
  ]
}
```

## Acceptance Criteria
- Existing 1901 behavior unchanged (test with current book data)
- JSON loading works with go:embed
- Feature-based matching produces valid orders for 1902+
- Build/disband orders validate correctly
- Tests pass

## Estimated Effort: M
