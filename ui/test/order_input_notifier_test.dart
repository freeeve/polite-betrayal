import 'package:flutter_test/flutter_test.dart';

import 'package:polite_betrayal/core/models/game_state.dart';
import 'package:polite_betrayal/features/game/order_input/order_input_notifier.dart';
import 'package:polite_betrayal/features/game/order_input/order_state.dart';

void main() {
  group('Support order candidate selection', () {
    /// Fleet ADR should be able to support Army SER moving to TRI.
    /// SER is not adjacent to ADR, but both can reach TRI.
    test('includes non-adjacent unit whose move target is reachable', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'ser'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');

      // Select Fleet ADR.
      notifier.selectProvince('adr');
      expect(notifier.state.phase, OrderPhase.unitSelected);

      // Choose support order type.
      notifier.selectOrderType('support');
      expect(notifier.state.phase, OrderPhase.awaitingAuxLoc);
      expect(notifier.state.validTargets, contains('ser'),
          reason: 'SER army can move to TRI which ADR fleet can reach');
    });

    /// Fleet ADR should be able to support an adjacent unit (e.g. Army TRI).
    test('includes adjacent unit for support hold', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'tri'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');
      notifier.selectProvince('adr');
      notifier.selectOrderType('support');

      expect(notifier.state.validTargets, contains('tri'),
          reason: 'TRI is directly adjacent to ADR');
    });

    /// Army MUN shares no reachable provinces with Fleet ADR so it should
    /// not appear as a support candidate.
    test('excludes unit with no shared reachable target', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'germany', province: 'mun'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');
      notifier.selectProvince('adr');
      notifier.selectOrderType('support');

      expect(notifier.state.validTargets, isNot(contains('mun')),
          reason: 'MUN army cannot reach any province ADR fleet can reach');
    });

    /// The supporting unit itself should never appear as a support candidate.
    test('excludes the supporting unit itself', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'tri'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');
      notifier.selectProvince('adr');
      notifier.selectOrderType('support');

      expect(notifier.state.validTargets, isNot(contains('adr')));
    });
  });

  group('Support order full flow', () {
    /// ADR support SER → TRI end-to-end: select unit, support, aux loc, target.
    test('ADR supports SER move to TRI', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'ser'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');

      // Step 1: select Fleet ADR.
      notifier.selectProvince('adr');
      expect(notifier.state.phase, OrderPhase.unitSelected);

      // Step 2: choose support.
      notifier.selectOrderType('support');
      expect(notifier.state.phase, OrderPhase.awaitingAuxLoc);
      expect(notifier.state.validTargets, contains('ser'));

      // Step 3: select SER as the unit to support.
      notifier.selectProvince('ser');
      expect(notifier.state.phase, OrderPhase.awaitingAuxTarget);
      expect(notifier.state.validTargets, contains('tri'),
          reason: 'TRI is reachable by both ADR and SER');

      // Step 4: select TRI as support destination.
      notifier.selectProvince('tri');
      expect(notifier.state.phase, OrderPhase.idle);

      // Verify the pending order.
      expect(notifier.state.pendingOrders, hasLength(1));
      final order = notifier.state.pendingOrders.first;
      expect(order.orderType, 'support');
      expect(order.location, 'adr');
      expect(order.unitType, 'fleet');
      expect(order.auxLoc, 'ser');
      expect(order.auxTarget, 'tri');
      expect(order.auxUnitType, 'army');
    });

    /// ADR support TRI hold end-to-end.
    test('ADR supports TRI hold', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'tri'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');

      notifier.selectProvince('adr');
      notifier.selectOrderType('support');
      expect(notifier.state.validTargets, contains('tri'));

      notifier.selectProvince('tri');
      expect(notifier.state.phase, OrderPhase.awaitingAuxTarget);
      // Support-hold: select the same province as destination.
      expect(notifier.state.validTargets, contains('tri'));

      notifier.selectProvince('tri');
      expect(notifier.state.phase, OrderPhase.idle);

      expect(notifier.state.pendingOrders, hasLength(1));
      final order = notifier.state.pendingOrders.first;
      expect(order.orderType, 'support');
      expect(order.location, 'adr');
      expect(order.auxLoc, 'tri');
      expect(order.auxTarget, isNull);
    });

    /// Support move targets must be the intersection of both units' reachable
    /// provinces. ADR and SER share TRI and ALB as reachable targets.
    test('support move targets are intersection of both units reach', () {
      final gs = GameState(
        year: 1901,
        season: 'Fall',
        phase: 'Diplomacy',
        units: [
          GameUnit(type: UnitType.fleet, power: 'italy', province: 'adr'),
          GameUnit(type: UnitType.army, power: 'austria', province: 'ser'),
        ],
        supplyCenters: {},
      );

      final notifier = OrderInputNotifier(gameState: gs, myPower: 'italy');
      notifier.selectProvince('adr');
      notifier.selectOrderType('support');
      notifier.selectProvince('ser');

      // ADR fleet reaches: tri, ven, apu, ion, alb
      // SER army reaches: bud, tri, alb, gre, bul, rum
      // Intersection: tri, alb
      // SER is NOT in ADR's fleet targets, so no support-hold option.
      expect(notifier.state.validTargets, containsAll(['tri', 'alb']));
      expect(notifier.state.validTargets, isNot(contains('ven')));
      expect(notifier.state.validTargets, isNot(contains('bud')));
      expect(notifier.state.validTargets, isNot(contains('ser')),
          reason: 'ADR fleet cannot reach SER so support-hold is not valid');
    });
  });
}
