import 'dart:ui';

import 'province_data.dart';
import 'province_polygons.dart';

/// Tests if a point is inside a polygon using the ray casting algorithm.
bool _pointInPolygon(Offset point, List<Offset> polygon) {
  var inside = false;
  for (var i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    final xi = polygon[i].dx, yi = polygon[i].dy;
    final xj = polygon[j].dx, yj = polygon[j].dy;
    if (((yi > point.dy) != (yj > point.dy)) &&
        (point.dx < (xj - xi) * (point.dy - yi) / (yj - yi) + xi)) {
      inside = !inside;
    }
  }
  return inside;
}

/// Finds the province at the tap point using polygon containment, with
/// nearest-center fallback for provinces without polygon data.
String? hitTestProvince(Offset tap, {double threshold = 45.0}) {
  // Try polygon hit test first (each province may have multiple rings).
  for (final entry in provincePolygons.entries) {
    for (final ring in entry.value) {
      if (ring.length >= 3 && _pointInPolygon(tap, ring)) {
        return entry.key;
      }
    }
  }

  // Fallback to nearest center for provinces without polygons.
  String? closest;
  double closestDist = double.infinity;
  for (final entry in provinces.entries) {
    final dist = (entry.value.center - tap).distance;
    if (dist < closestDist && dist <= threshold) {
      closestDist = dist;
      closest = entry.key;
    }
  }
  return closest;
}

/// Finds the nearest land province (for selecting units, not sea zones).
String? hitTestLandProvince(Offset tap, {double threshold = 45.0}) {
  // Try polygon hit test first (each province may have multiple rings).
  for (final entry in provincePolygons.entries) {
    for (final ring in entry.value) {
      if (ring.length >= 3 && _pointInPolygon(tap, ring)) {
        final prov = provinces[entry.key];
        if (prov != null && !prov.isSea) return entry.key;
      }
    }
  }

  // Fallback to nearest center.
  String? closest;
  double closestDist = double.infinity;
  for (final entry in provinces.entries) {
    if (entry.value.isSea) continue;
    final dist = (entry.value.center - tap).distance;
    if (dist < closestDist && dist <= threshold) {
      closestDist = dist;
      closest = entry.key;
    }
  }
  return closest;
}
