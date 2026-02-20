import 'package:flutter/material.dart';

import '../../../core/map/province_data.dart';
import '../../../core/models/order.dart';
import '../../../core/theme/app_theme.dart';

/// Compact scrollable list of resolved orders grouped by power,
/// designed to fit in the 280px side panel.
class OrderListPanel extends StatelessWidget {
  final List<Order> orders;

  const OrderListPanel({super.key, required this.orders});

  @override
  Widget build(BuildContext context) {
    final byPower = <String, List<Order>>{};
    for (final order in orders) {
      byPower.putIfAbsent(order.power, () => []).add(order);
    }

    return ListView(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 4),
          child: Text('Orders', style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: Colors.grey.shade400,
          )),
        ),
        for (final entry in byPower.entries) ...[
          _PowerHeader(power: entry.key),
          for (final order in entry.value)
            _CompactOrderRow(order: order),
          const SizedBox(height: 4),
        ],
      ],
    );
  }
}

class _PowerHeader extends StatelessWidget {
  final String power;
  const _PowerHeader({required this.power});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 2, bottom: 1),
      child: Row(
        children: [
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(
              color: PowerColors.forPower(power),
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 5),
          Text(
            PowerColors.label(power),
            style: const TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 11,
            ),
          ),
        ],
      ),
    );
  }
}

class _CompactOrderRow extends StatelessWidget {
  final Order order;
  const _CompactOrderRow({required this.order});

  @override
  Widget build(BuildContext context) {
    final from = provinces[order.location]?.name ?? order.location;
    final color = _resultColor(order.result);
    final icon = _resultIcon(order.result);

    String desc;
    switch (order.orderType) {
      case 'hold':
        desc = '$from HOLD';
      case 'move':
        final to = provinces[order.target]?.name ?? (order.target ?? '?');
        desc = '$from -> $to';
      case 'support':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        if (order.target != null && order.target!.isNotEmpty) {
          final to = provinces[order.target]?.name ?? order.target!;
          desc = '$from S $aux -> $to';
        } else {
          desc = '$from S $aux H';
        }
      case 'convoy':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        final to = provinces[order.target]?.name ?? (order.target ?? '?');
        desc = '$from C $aux -> $to';
      default:
        desc = '$from ${order.orderType.toUpperCase()}';
    }

    return Padding(
      padding: const EdgeInsets.only(left: 15, top: 1, bottom: 1),
      child: Row(
        children: [
          Icon(icon, color: color, size: 13),
          const SizedBox(width: 4),
          Expanded(
            child: Text(
              desc,
              style: TextStyle(color: color, fontSize: 11),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }

  Color _resultColor(String? result) {
    return switch (result) {
      'succeeds' => Colors.green.shade300,
      'bounced' || 'fails' => Colors.red.shade300,
      'cut' => Colors.orange.shade300,
      'void' => Colors.grey.shade400,
      _ => Colors.white70,
    };
  }

  IconData _resultIcon(String? result) {
    return switch (result) {
      'succeeds' => Icons.check_circle_outline,
      'bounced' || 'fails' => Icons.cancel_outlined,
      'cut' => Icons.warning_outlined,
      _ => Icons.remove_circle_outline,
    };
  }
}
