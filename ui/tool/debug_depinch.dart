import 'dart:io';

/// Debug tool: examine de-pinched polygon results for the 8 problematic provinces.
void main() {
  final src = File('lib/core/map/province_polygons.dart').readAsStringSync();

  String extract(String id) {
    final re = RegExp("'$id':\\s*\\[(.+?)\\],");
    final m = re.firstMatch(src);
    if (m == null) return 'NOT FOUND';
    return m.group(1)!;
  }

  List<List<double>> points(String data) {
    return RegExp(r'Offset\(([^)]+)\)')
        .allMatches(data)
        .map((m) {
      final parts = m.group(1)!.split(', ');
      return [double.parse(parts[0]), double.parse(parts[1])];
    }).toList();
  }

  // Province centers from province_data.dart
  final centers = <String, List<double>>{
    'wal': [234, 576], 'den': [480, 475], 'iri': [160, 560],
    'eng': [230, 630], 'nth': [370, 490], 'ska': [450, 410],
    'hel': [400, 510], 'bal': [530, 460],
  };

  for (final id in ['wal', 'den', 'iri', 'eng', 'nth', 'ska', 'hel', 'bal']) {
    final data = extract(id);
    if (data == 'NOT FOUND') { print('$id: NOT FOUND'); continue; }
    final pts = points(data);
    final xs = pts.map((p) => p[0]).toList()..sort();
    final ys = pts.map((p) => p[1]).toList()..sort();
    print('$id: ${pts.length} pts, bbox x=${xs.first.toStringAsFixed(0)}-${xs.last.toStringAsFixed(0)}, '
        'y=${ys.first.toStringAsFixed(0)}-${ys.last.toStringAsFixed(0)}');

    // Check if province center is inside polygon
    final c = centers[id]!;
    final inside = _pointInPolygon(c[0], c[1], pts);
    print('  center (${c[0]}, ${c[1]}) inside: $inside');
  }

  // Check which polygons are identical
  print('\nDuplicate check:');
  final all = <String, String>{};
  for (final id in ['wal', 'den', 'iri', 'eng', 'nth', 'ska', 'hel', 'bal']) {
    final data = extract(id);
    final pts = points(data);
    final key = pts.map((p) => '${p[0].toStringAsFixed(1)},${p[1].toStringAsFixed(1)}').join(';');
    for (final e in all.entries) {
      if (e.value == key) {
        print('  $id == ${e.key}');
        break;
      }
    }
    all[id] = key;
  }
}

bool _pointInPolygon(double px, double py, List<List<double>> poly) {
  var inside = false;
  for (var i = 0, j = poly.length - 1; i < poly.length; j = i++) {
    final yi = poly[i][1], yj = poly[j][1];
    if ((yi > py) != (yj > py)) {
      final xi = poly[i][0], xj = poly[j][0];
      if (px < (xj - xi) * (py - yi) / (yj - yi) + xi) {
        inside = !inside;
      }
    }
  }
  return inside;
}
