import 'package:flutter/material.dart';

import 'order_state.dart';

/// Bottom action bar showing order type buttons, prompt, cancel, submit/ready.
class OrderActionBar extends StatelessWidget {
  final OrderInputState state;
  final void Function(String type)? onOrderType;
  final VoidCallback? onCancel;
  final VoidCallback? onSubmit;
  final VoidCallback? onEdit;
  final VoidCallback? onSkip;

  const OrderActionBar({
    super.key,
    required this.state,
    this.onOrderType,
    this.onCancel,
    this.onSubmit,
    this.onEdit,
    this.onSkip,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        border: Border(top: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Prompt text
          Text(
            state.prompt,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
          ),
          const SizedBox(height: 8),
          if (state.phase == OrderPhase.unitSelected) _buildOrderTypeButtons(),
          if (state.phase != OrderPhase.idle && state.phase != OrderPhase.unitSelected)
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                TextButton.icon(
                  onPressed: onCancel,
                  icon: const Icon(Icons.close),
                  label: const Text('Cancel'),
                ),
              ],
            ),
          if (state.phase == OrderPhase.idle) _buildSubmitRow(context),
        ],
      ),
    );
  }

  Widget _buildOrderTypeButtons() {
    return Wrap(
      spacing: 8,
      children: [
        _OrderButton(label: 'Hold', icon: Icons.shield, onTap: () => onOrderType?.call('hold')),
        _OrderButton(label: 'Move', icon: Icons.arrow_forward, onTap: () => onOrderType?.call('move')),
        _OrderButton(label: 'Support', icon: Icons.support, onTap: () => onOrderType?.call('support')),
        _OrderButton(label: 'Convoy', icon: Icons.sailing, onTap: () => onOrderType?.call('convoy')),
        TextButton.icon(
          onPressed: onCancel,
          icon: const Icon(Icons.close),
          label: const Text('Cancel'),
        ),
      ],
    );
  }

  Widget _buildSubmitRow(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        if (state.pendingOrders.isNotEmpty && !state.submitted)
          FilledButton.icon(
            onPressed: onSubmit,
            icon: const Icon(Icons.send),
            label: Text('Submit (${state.pendingOrders.length})'),
          ),
        if (state.ready) ...[
          FilledButton.icon(
            onPressed: onEdit,
            icon: const Icon(Icons.edit),
            label: const Text('Edit Orders'),
          ),
          const SizedBox(width: 12),
          const Chip(
            avatar: Icon(Icons.check_circle, color: Colors.green),
            label: Text('Ready'),
          ),
        ],
        if (state.pendingOrders.isEmpty && !state.submitted) ...[
          const Text('Select a unit to begin'),
          const SizedBox(width: 12),
          FilledButton.tonal(
            onPressed: onSkip,
            child: const Text('Skip Turn'),
          ),
        ],
      ],
    );
  }
}

class _OrderButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final VoidCallback onTap;

  const _OrderButton({required this.label, required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return FilledButton.tonal(
      onPressed: onTap,
      style: FilledButton.styleFrom(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 18),
          const SizedBox(width: 4),
          Text(label),
        ],
      ),
    );
  }
}
