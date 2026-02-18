import 'package:flutter/material.dart';

import '../../../core/map/adjacency_data.dart';
import '../../../core/map/province_data.dart';
import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';

/// Retreat phase panel: shows dislodged units, retreat or disband.
class RetreatOrderPanel extends StatelessWidget {
  final GameState gameState;
  final String myPower;
  final List<OrderInput> pendingOrders;
  final void Function(OrderInput order)? onAddOrder;
  final void Function(int index)? onRemoveOrder;

  const RetreatOrderPanel({
    super.key,
    required this.gameState,
    required this.myPower,
    required this.pendingOrders,
    this.onAddOrder,
    this.onRemoveOrder,
  });

  @override
  Widget build(BuildContext context) {
    final myDislodged = gameState.dislodged
        .where((d) => d.unit.power == myPower)
        .toList();

    if (myDislodged.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(16),
        child: const Text('No units need retreating.'),
      );
    }

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(top: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Retreat Orders (${myDislodged.length} dislodged)',
            style: Theme.of(context).textTheme.titleSmall,
          ),
          const SizedBox(height: 8),
          ...myDislodged.map((d) => _DislodgedUnitTile(
                dislodged: d,
                gameState: gameState,
                hasOrder: pendingOrders.any((o) => o.location == d.unit.province),
                onRetreat: (target) {
                  onAddOrder?.call(OrderInput(
                    unitType: d.unit.type.fullName,
                    location: d.unit.province,
                    orderType: 'retreat_move',
                    target: target,
                  ));
                },
                onDisband: () {
                  onAddOrder?.call(OrderInput(
                    unitType: d.unit.type.fullName,
                    location: d.unit.province,
                    orderType: 'disband',
                  ));
                },
              )),
          if (pendingOrders.isNotEmpty) ...[
            const Divider(),
            ...pendingOrders.asMap().entries.map((e) {
              final order = e.value;
              final from = provinces[order.location]?.name ?? order.location;
              final to = order.target != null ? provinces[order.target]?.name ?? order.target! : '';
              return ListTile(
                dense: true,
                title: Text('$from ${order.orderType.toUpperCase()} ${to.isNotEmpty ? "-> $to" : ""}'),
                trailing: IconButton(
                  icon: const Icon(Icons.delete_outline, size: 20),
                  onPressed: () => onRemoveOrder?.call(e.key),
                ),
              );
            }),
          ],
        ],
      ),
    );
  }
}

class _DislodgedUnitTile extends StatelessWidget {
  final DislodgedUnit dislodged;
  final GameState gameState;
  final bool hasOrder;
  final void Function(String target) onRetreat;
  final VoidCallback onDisband;

  const _DislodgedUnitTile({
    required this.dislodged,
    required this.gameState,
    required this.hasOrder,
    required this.onRetreat,
    required this.onDisband,
  });

  @override
  Widget build(BuildContext context) {
    final unit = dislodged.unit;
    final name = provinces[unit.province]?.name ?? unit.province;
    final isFleet = unit.type == UnitType.fleet;
    final targets = isFleet
        ? fleetTargets(unit.province, coast: unit.coast)
        : armyTargets(unit.province);
    // Can't retreat to attacker's province or occupied provinces.
    final validTargets = targets.where((t) {
      if (t == dislodged.attackerFrom) return false;
      if (gameState.units.any((u) => u.province == t)) return false;
      return true;
    }).toList();

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(isFleet ? Icons.sailing : Icons.shield, size: 18),
                const SizedBox(width: 4),
                Text('${unit.type.label} $name'),
                if (hasOrder) ...[
                  const Spacer(),
                  const Icon(Icons.check_circle, color: Colors.green, size: 18),
                ],
              ],
            ),
            const SizedBox(height: 8),
            if (validTargets.isEmpty)
              OutlinedButton(onPressed: onDisband, child: const Text('Must disband'))
            else
              Wrap(
                spacing: 8,
                children: [
                  ...validTargets.map((t) {
                    final tName = provinces[t]?.name ?? t;
                    return ActionChip(
                      label: Text(tName),
                      onPressed: () => onRetreat(t),
                    );
                  }),
                  ActionChip(
                    avatar: const Icon(Icons.close, size: 16),
                    label: const Text('Disband'),
                    onPressed: onDisband,
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}
