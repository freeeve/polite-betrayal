# Fix Missing Map Adjacencies

## Status: Done

## Description
Four standard Diplomacy adjacencies were missing from both Go backend and Flutter frontend:
- **lvp-yor** (Liverpool-Yorkshire) - army + fleet
- **kie-hol** (Kiel-Holland) - army + fleet
- **rom-apu** (Rome-Apulia) - army + fleet
- **ank-smy** (Ankara-Smyrna) - army + fleet

## Changes
- `api/pkg/diplomacy/map_data.go` - Added 4 `addBothAdj()` calls
- `ui/lib/core/map/adjacency_data.dart` - Added 4 `AdjType.both` entries
- All Go tests pass, go vet clean, gofmt applied

## Notes
- The opening book (`api/internal/bot/opening_book.go`) had to work around these missing adjacencies
- Memory note about smy-ank corrected: they ARE adjacent in standard Diplomacy
