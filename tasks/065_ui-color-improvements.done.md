# UI Power Color Improvements

## Status: Done

## Changes
Final culturally-matched 7-color scheme in `ui/lib/core/theme/app_theme.dart`:
- Austria = Yellow (0xFFFFEB3B) - Habsburg gold
- England = Red (0xFFC62828) - St. George's Cross
- France = Blue (0xFF1565C0) - Les Bleus
- Germany = Brown (0xFF795548) - Prussian black substitute
- Italy = Green (0xFF2E7D32) - Italian flag
- Russia = Purple (0xFF7B1FA2) - Imperial Russia
- Turkey = Orange (0xFFEF6C00) - Mediterranean warmth

Per-power opacity boost in `map_painter.dart` for low-saturation colors (brown/yellow):
- Germany/Austria land fill: 65% (vs 45% default)
- Germany/Austria land border: 55% (vs 35% default)
