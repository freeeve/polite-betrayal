# Fix Build/Disband Animations

## Status: Done

## Problem
In bot games, build phases resolve so fast the client never observes `_previousPhaseType == 'build'`. The phase jumps straight from Fall movement to Spring movement.

## Fix in `ui/lib/features/game/game_screen.dart`
Detect build phases even when skipped:
```dart
final wasBuildPhase = _previousPhaseType == 'build';
final unitsChanged = newProvinces.length != _previousPhaseUnitProvinces!.length
    || newProvinces.difference(_previousPhaseUnitProvinces!).isNotEmpty;
final nowSpring = gameViewState.currentPhase?.season == 'spring';
final buildPhaseOccurred = wasBuildPhase || (unitsChanged && nowSpring);
```
