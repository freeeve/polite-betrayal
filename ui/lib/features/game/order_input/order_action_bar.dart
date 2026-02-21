import 'dart:async';

import 'package:flutter/material.dart';

import 'order_state.dart';

/// Bottom action bar showing order type buttons, prompt, cancel, submit/ready.
class OrderActionBar extends StatefulWidget {
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
  State<OrderActionBar> createState() => _OrderActionBarState();
}

class _OrderActionBarState extends State<OrderActionBar> {
  bool _showConfirmation = false;
  Timer? _confirmationTimer;

  @override
  void didUpdateWidget(covariant OrderActionBar oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Detect submitted transition: false -> true
    if (widget.state.submitted && !oldWidget.state.submitted) {
      setState(() => _showConfirmation = true);
      _confirmationTimer?.cancel();
      _confirmationTimer = Timer(const Duration(milliseconds: 1500), () {
        if (mounted) {
          setState(() => _showConfirmation = false);
        }
      });
    }
  }

  /// Cancel timer and clear confirmation state for phase transitions.
  void resetForNewPhase() {
    _confirmationTimer?.cancel();
    _confirmationTimer = null;
    if (_showConfirmation) {
      setState(() => _showConfirmation = false);
    }
  }

  @override
  void dispose() {
    _confirmationTimer?.cancel();
    super.dispose();
  }

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
            widget.state.prompt,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w500,
                ),
          ),
          const SizedBox(height: 8),
          if (widget.state.phase == OrderPhase.unitSelected) _buildOrderTypeButtons(),
          if (widget.state.phase != OrderPhase.idle && widget.state.phase != OrderPhase.unitSelected)
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                TextButton.icon(
                  onPressed: widget.onCancel,
                  icon: const Icon(Icons.close),
                  label: const Text('Cancel'),
                ),
              ],
            ),
          if (widget.state.phase == OrderPhase.idle) _buildSubmitRow(context),
        ],
      ),
    );
  }

  Widget _buildOrderTypeButtons() {
    return Wrap(
      spacing: 8,
      children: [
        _OrderButton(label: 'Hold', icon: Icons.shield, onTap: () => widget.onOrderType?.call('hold')),
        _OrderButton(label: 'Move', icon: Icons.arrow_forward, onTap: () => widget.onOrderType?.call('move')),
        _OrderButton(label: 'Support', icon: Icons.support, onTap: () => widget.onOrderType?.call('support')),
        _OrderButton(label: 'Convoy', icon: Icons.sailing, onTap: () => widget.onOrderType?.call('convoy')),
        TextButton.icon(
          onPressed: widget.onCancel,
          icon: const Icon(Icons.close),
          label: const Text('Cancel'),
        ),
      ],
    );
  }

  Widget _buildSubmitRow(BuildContext context) {
    return AnimatedSwitcher(
      duration: const Duration(milliseconds: 500),
      child: _showConfirmation
          ? Row(
              key: const ValueKey('confirmation'),
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Icon(Icons.check_circle, color: Colors.green, size: 24),
                const SizedBox(width: 8),
                Text(
                  'Orders Submitted',
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: Colors.green,
                        fontWeight: FontWeight.w600,
                      ),
                ),
              ],
            )
          : Row(
              key: const ValueKey('actions'),
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                if (widget.state.pendingOrders.isNotEmpty && !widget.state.submitted)
                  FilledButton.icon(
                    onPressed: widget.onSubmit,
                    icon: const Icon(Icons.send),
                    label: Text('Submit (${widget.state.pendingOrders.length})'),
                  ),
                if (widget.state.ready) ...[
                  FilledButton.icon(
                    onPressed: widget.onEdit,
                    icon: const Icon(Icons.edit),
                    label: const Text('Edit Orders'),
                  ),
                  const SizedBox(width: 12),
                  const Chip(
                    avatar: Icon(Icons.check_circle, color: Colors.green),
                    label: Text('Ready'),
                  ),
                ],
                if (widget.state.pendingOrders.isEmpty && !widget.state.submitted) ...[
                  const Text('Select a unit to begin'),
                  const SizedBox(width: 12),
                  FilledButton.tonal(
                    onPressed: widget.onSkip,
                    child: const Text('Skip Turn'),
                  ),
                ],
              ],
            ),
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
