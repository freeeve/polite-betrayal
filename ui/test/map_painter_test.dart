import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:polite_betrayal/core/models/game_state.dart';
import 'package:polite_betrayal/features/game/widgets/map_painter.dart';

/// Initial Diplomacy game state — Spring 1901, all 7 powers at starting positions.
GameState _initialGameState() {
  return const GameState(
    year: 1901,
    season: 'Spring',
    phase: 'Diplomacy',
    units: [
      // Austria
      GameUnit(type: UnitType.army, power: 'austria', province: 'vie'),
      GameUnit(type: UnitType.army, power: 'austria', province: 'bud'),
      GameUnit(type: UnitType.fleet, power: 'austria', province: 'tri'),
      // England
      GameUnit(type: UnitType.fleet, power: 'england', province: 'lon'),
      GameUnit(type: UnitType.fleet, power: 'england', province: 'edi'),
      GameUnit(type: UnitType.army, power: 'england', province: 'lvp'),
      // France
      GameUnit(type: UnitType.army, power: 'france', province: 'par'),
      GameUnit(type: UnitType.army, power: 'france', province: 'mar'),
      GameUnit(type: UnitType.fleet, power: 'france', province: 'bre'),
      // Germany
      GameUnit(type: UnitType.army, power: 'germany', province: 'ber'),
      GameUnit(type: UnitType.army, power: 'germany', province: 'mun'),
      GameUnit(type: UnitType.fleet, power: 'germany', province: 'kie'),
      // Italy
      GameUnit(type: UnitType.army, power: 'italy', province: 'rom'),
      GameUnit(type: UnitType.army, power: 'italy', province: 'ven'),
      GameUnit(type: UnitType.fleet, power: 'italy', province: 'nap'),
      // Russia
      GameUnit(type: UnitType.army, power: 'russia', province: 'mos'),
      GameUnit(type: UnitType.army, power: 'russia', province: 'war'),
      GameUnit(type: UnitType.fleet, power: 'russia', province: 'sev'),
      GameUnit(type: UnitType.fleet, power: 'russia', province: 'stp', coast: 'sc'),
      // Turkey
      GameUnit(type: UnitType.army, power: 'turkey', province: 'con'),
      GameUnit(type: UnitType.army, power: 'turkey', province: 'smy'),
      GameUnit(type: UnitType.fleet, power: 'turkey', province: 'ank'),
    ],
    supplyCenters: {
      'vie': 'austria', 'bud': 'austria', 'tri': 'austria',
      'lon': 'england', 'edi': 'england', 'lvp': 'england',
      'par': 'france', 'mar': 'france', 'bre': 'france',
      'ber': 'germany', 'mun': 'germany', 'kie': 'germany',
      'rom': 'italy', 'ven': 'italy', 'nap': 'italy',
      'mos': 'russia', 'war': 'russia', 'sev': 'russia', 'stp': 'russia',
      'con': 'turkey', 'smy': 'turkey', 'ank': 'turkey',
    },
  );
}

void main() {
  testWidgets('MapPainter territory shading - initial positions', (tester) async {
    final painter = MapPainter(gameState: _initialGameState());

    final key = GlobalKey();
    await tester.pumpWidget(
      MaterialApp(
        home: Center(
          child: RepaintBoundary(
            key: key,
            child: SizedBox(
              width: 576,
              height: 576,
              child: CustomPaint(
                painter: painter,
                size: const Size(576, 576),
              ),
            ),
          ),
        ),
      ),
    );

    await expectLater(
      find.byKey(key),
      matchesGoldenFile('goldens/map_initial_positions.png'),
    );
  });
}
