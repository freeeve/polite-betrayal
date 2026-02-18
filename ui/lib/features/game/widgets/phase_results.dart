import 'package:flutter/material.dart';

import '../../../core/map/province_data.dart';
import '../../../core/models/order.dart';
import '../../../core/theme/app_theme.dart';

/// Overlay showing each order's result after phase resolution.
class PhaseResults extends StatelessWidget {
  final List<Order> orders;
  final VoidCallback? onDismiss;

  const PhaseResults({super.key, required this.orders, this.onDismiss});

  @override
  Widget build(BuildContext context) {
    // Group orders by power.
    final byPower = <String, List<Order>>{};
    for (final order in orders) {
      byPower.putIfAbsent(order.power, () => []).add(order);
    }

    return Container(
      color: Colors.black54,
      child: SafeArea(
        child: Column(
          children: [
            AppBar(
              title: const Text('Phase Results'),
              backgroundColor: Colors.transparent,
              automaticallyImplyLeading: false,
              actions: [
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed: onDismiss,
                ),
              ],
            ),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: byPower.entries.map((entry) {
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Container(
                            width: 20,
                            height: 20,
                            decoration: BoxDecoration(
                              color: PowerColors.forPower(entry.key),
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(
                            PowerColors.label(entry.key),
                            style: const TextStyle(
                              color: Colors.white,
                              fontWeight: FontWeight.bold,
                              fontSize: 16,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      ...entry.value.map((order) => _OrderResult(order: order)),
                      const SizedBox(height: 12),
                    ],
                  );
                }).toList(),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _OrderResult extends StatelessWidget {
  final Order order;
  const _OrderResult({required this.order});

  @override
  Widget build(BuildContext context) {
    final from = provinces[order.location]?.name ?? order.location;
    final resultColor = _resultColor(order.result);
    final resultIcon = _resultIcon(order.result);

    String description;
    switch (order.orderType) {
      case 'hold':
        description = '$from HOLD';
      case 'move':
        final to = provinces[order.target]?.name ?? (order.target ?? '?');
        description = '$from -> $to';
      case 'support':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        if (order.target != null && order.target!.isNotEmpty) {
          final to = provinces[order.target]?.name ?? order.target!;
          description = '$from SUPPORT $aux -> $to';
        } else {
          description = '$from SUPPORT $aux HOLD';
        }
      case 'convoy':
        final aux = provinces[order.auxLoc]?.name ?? (order.auxLoc ?? '?');
        final to = provinces[order.target]?.name ?? (order.target ?? '?');
        description = '$from CONVOY $aux -> $to';
      default:
        description = '$from ${order.orderType.toUpperCase()}';
    }

    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 2),
      child: Row(
        children: [
          Icon(resultIcon, color: resultColor, size: 18),
          const SizedBox(width: 6),
          Expanded(
            child: Text(
              description,
              style: TextStyle(color: resultColor, fontSize: 14),
            ),
          ),
          if (order.result != null)
            Text(
              order.result!,
              style: TextStyle(
                color: resultColor,
                fontSize: 12,
                fontWeight: FontWeight.w600,
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
