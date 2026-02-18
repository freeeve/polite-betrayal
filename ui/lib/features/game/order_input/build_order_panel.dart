import 'package:flutter/material.dart';

import '../../../core/map/province_data.dart';
import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';
import '../../../core/theme/app_theme.dart';

/// Build phase panel: shows needed builds or disbands, tap to place.
class BuildOrderPanel extends StatelessWidget {
  final GameState gameState;
  final String myPower;
  final List<OrderInput> pendingOrders;
  final void Function(OrderInput order)? onAddOrder;
  final void Function(int index)? onRemoveOrder;

  const BuildOrderPanel({
    super.key,
    required this.gameState,
    required this.myPower,
    required this.pendingOrders,
    this.onAddOrder,
    this.onRemoveOrder,
  });

  @override
  Widget build(BuildContext context) {
    final scCount = gameState.supplyCenterCount(myPower);
    final unitCount = gameState.unitsOf(myPower).length;
    final diff = scCount - unitCount;

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
          Row(
            children: [
              Container(
                width: 24,
                height: 24,
                decoration: BoxDecoration(
                  color: PowerColors.forPower(myPower),
                  shape: BoxShape.circle,
                ),
                alignment: Alignment.center,
                child: Text(
                  myPower[0].toUpperCase(),
                  style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.bold),
                ),
              ),
              const SizedBox(width: 8),
              Text(
                diff > 0
                    ? 'Build $diff unit${diff != 1 ? 's' : ''}'
                    : diff < 0
                        ? 'Disband ${-diff} unit${diff != -1 ? 's' : ''}'
                        : 'No builds or disbands needed',
                style: Theme.of(context).textTheme.titleSmall,
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text('SC: $scCount | Units: $unitCount', style: Theme.of(context).textTheme.bodySmall),
          if (diff > 0) ...[
            const SizedBox(height: 8),
            Text('Tap a home supply center to build:', style: Theme.of(context).textTheme.bodySmall),
            Wrap(
              spacing: 8,
              children: _homeSupplyCenters().map((sc) {
                final name = provinces[sc]?.name ?? sc;
                return ActionChip(
                  label: Text(name),
                  onPressed: () => _showBuildDialog(context, sc),
                );
              }).toList(),
            ),
          ],
          if (diff < 0) ...[
            const SizedBox(height: 8),
            Text('Tap a unit to disband:', style: Theme.of(context).textTheme.bodySmall),
            Wrap(
              spacing: 8,
              children: gameState.unitsOf(myPower).map((unit) {
                final name = provinces[unit.province]?.name ?? unit.province;
                return ActionChip(
                  avatar: Text(unit.type.label),
                  label: Text(name),
                  onPressed: () {
                    onAddOrder?.call(OrderInput(
                      unitType: unit.type.fullName,
                      location: unit.province,
                      orderType: 'disband',
                    ));
                  },
                );
              }).toList(),
            ),
          ],
          if (pendingOrders.isNotEmpty) ...[
            const Divider(),
            ...pendingOrders.asMap().entries.map((e) {
              final order = e.value;
              return ListTile(
                dense: true,
                title: Text('${order.orderType.toUpperCase()} ${order.unitType} at ${order.location}'),
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

  void _showBuildDialog(BuildContext context, String province) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Build at ${provinces[province]?.name ?? province}'),
        content: const Text('Which unit type?'),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              onAddOrder?.call(OrderInput(
                unitType: 'army',
                location: province,
                orderType: 'build',
              ));
            },
            child: const Text('Army'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              onAddOrder?.call(OrderInput(
                unitType: 'fleet',
                location: province,
                orderType: 'build',
              ));
            },
            child: const Text('Fleet'),
          ),
        ],
      ),
    );
  }

  List<String> _homeSupplyCenters() {
    // Home SCs that are owned and have no unit on them.
    final homeScs = _homeScsForPower(myPower);
    return homeScs.where((sc) {
      return gameState.supplyCenters[sc] == myPower &&
          !gameState.units.any((u) => u.province == sc);
    }).toList();
  }

  List<String> _homeScsForPower(String power) {
    return switch (power) {
      'austria' => ['vie', 'bud', 'tri'],
      'england' => ['lon', 'edi', 'lvp'],
      'france' => ['bre', 'par', 'mar'],
      'germany' => ['ber', 'kie', 'mun'],
      'italy' => ['nap', 'rom', 'ven'],
      'russia' => ['mos', 'sev', 'stp', 'war'],
      'turkey' => ['ank', 'con', 'smy'],
      _ => [],
    };
  }
}
