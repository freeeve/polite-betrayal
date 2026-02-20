# Retreat Animations

## Status: Done

## Dependencies
- None

## Description
Add animated retreat movements to the Flutter UI map. Currently retreats are not visually animated — units jump to their retreat destination (or disappear if disbanded). This makes retreat phases hard to follow.

### Changes Required

1. **Animate retreat movements** (`ui/`):
   - Show retreating units sliding from their dislodged position to the retreat destination
   - Use a distinct visual style (e.g., dashed path, faded unit) to differentiate retreats from normal moves
   - Animate disbanded units fading out at their current position

2. **Retreat phase indication**:
   - Clearly indicate when a retreat phase is active in the UI
   - Show which provinces are valid retreat destinations (highlight or mark)

3. **Timing**:
   - Retreat animations should be shorter than movement animations since retreat phases are simpler
   - Ensure animations complete before advancing to the next phase in replay mode

## Acceptance Criteria
- Retreating units animate smoothly from origin to destination
- Disbanded units fade out visually
- Retreat movements are visually distinct from normal movements
- Animations work correctly in both live play and replay mode
- No regression in existing movement/build phase animations

## Estimated Effort: M
