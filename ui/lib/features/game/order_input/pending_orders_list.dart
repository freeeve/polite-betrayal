import 'package:flutter/material.dart';

import '../../../core/map/province_data.dart';
import '../../../core/models/order.dart';

/// Draggable bottom sheet listing pending orders with delete.
class PendingOrdersList extends StatelessWidget {
  final List<OrderInput> orders;
  final void Function(int index)? onRemove;

  const PendingOrdersList({super.key, required this.orders, this.onRemove});

  @override
  Widget build(BuildContext context) {
    if (orders.isEmpty) return const SizedBox.shrink();

    return Container(
      constraints: const BoxConstraints(maxHeight: 200),
      child: ListView.builder(
        shrinkWrap: true,
        itemCount: orders.length,
        itemBuilder: (context, i) {
          final order = orders[i];
          return ListTile(
            dense: true,
            leading: Icon(
              _orderIcon(order.orderType),
              color: _orderColor(order.orderType),
            ),
            title: Text(_describeOrder(order)),
            trailing: onRemove != null
                ? IconButton(
                    icon: const Icon(Icons.delete_outline, size: 20),
                    onPressed: () => onRemove!.call(i),
                  )
                : null,
          );
        },
      ),
    );
  }

  String _describeOrder(OrderInput order) {
    final from = provinces[order.location]?.name ?? order.location;
    final type = order.orderType.toUpperCase();

    switch (order.orderType) {
      case 'hold':
        return '$from: $type';
      case 'move':
        var to = provinces[order.target]?.name ?? (order.target ?? '?');
        if (order.targetCoast != null) to += ' (${order.targetCoast})';
        return '$from $type -> $to';
      case 'support':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        if (order.target != null) {
          final to = provinces[order.target]?.name ?? order.target!;
          return '$from $type $aux -> $to';
        }
        return '$from $type $aux HOLD';
      case 'convoy':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        final to = provinces[order.auxTarget]?.name ?? (order.auxTarget ?? '?');
        return '$from $type $aux -> $to';
      default:
        return '$from: $type';
    }
  }

  IconData _orderIcon(String type) {
    return switch (type) {
      'hold' => Icons.shield,
      'move' => Icons.arrow_forward,
      'support' => Icons.support,
      'convoy' => Icons.sailing,
      _ => Icons.help_outline,
    };
  }

  Color _orderColor(String type) {
    return switch (type) {
      'hold' => Colors.grey,
      'move' => Colors.blue,
      'support' => Colors.yellow.shade700,
      'convoy' => Colors.purple,
      _ => Colors.grey,
    };
  }
}
