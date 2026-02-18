import 'package:flutter/material.dart';

import '../../core/theme/app_theme.dart';

/// Small colored badge displaying a power's first letter.
class PowerBadge extends StatelessWidget {
  final String power;
  final double size;

  const PowerBadge({super.key, required this.power, this.size = 28});

  @override
  Widget build(BuildContext context) {
    final color = PowerColors.forPower(power);
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: color,
        shape: BoxShape.circle,
      ),
      alignment: Alignment.center,
      child: Text(
        power.isNotEmpty ? power[0].toUpperCase() : '?',
        style: TextStyle(
          color: Colors.white,
          fontSize: size * 0.5,
          fontWeight: FontWeight.bold,
        ),
      ),
    );
  }
}
