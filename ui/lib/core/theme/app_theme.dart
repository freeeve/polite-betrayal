import 'package:flutter/material.dart';

/// Power colors for the seven great powers.
class PowerColors {
  // Classic 7-color palette matched to cultural/historical associations.
  static const austria = Color(0xFFFFEB3B);   // Yellow (Habsburg gold)
  static const england = Color(0xFFC62828);   // Red (St. George's Cross, red coats)
  static const france = Color(0xFF1565C0);    // Blue (Les Bleus, Bourbon royal blue)
  static const germany = Color(0xFF795548);   // Brown (Prussian black substitute)
  static const italy = Color(0xFF2E7D32);     // Green (Italian flag)
  static const russia = Color(0xFF7B1FA2);    // Purple (imperial Russia)
  static const turkey = Color(0xFFEF6C00);    // Orange (warm/Mediterranean)

  static Color forPower(String power) {
    switch (power.toLowerCase()) {
      case 'austria':
        return austria;
      case 'england':
        return england;
      case 'france':
        return france;
      case 'germany':
        return germany;
      case 'italy':
        return italy;
      case 'russia':
        return russia;
      case 'turkey':
        return turkey;
      default:
        return Colors.grey;
    }
  }

  static String label(String power) {
    return power[0].toUpperCase() + power.substring(1);
  }
}

/// All seven powers in standard order.
const allPowers = [
  'austria',
  'england',
  'france',
  'germany',
  'italy',
  'russia',
  'turkey',
];

class AppTheme {
  static ThemeData get light {
    return ThemeData(
      useMaterial3: true,
      colorSchemeSeed: const Color(0xFF1A237E),
      brightness: Brightness.light,
      appBarTheme: const AppBarTheme(centerTitle: true),
    );
  }
}
