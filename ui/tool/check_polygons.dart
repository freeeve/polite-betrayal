import 'dart:io';

void main() {
  final src = File('lib/core/map/province_polygons.dart').readAsStringSync();

  for (final id in ['wal', 'spa', 'iri', 'eng', 'den', 'nth', 'ska', 'hel', 'bal']) {
    final re = RegExp("'$id':\\s*\\[(.+?)\\],");
    final m = re.firstMatch(src);
    if (m == null) { print('$id: NOT FOUND'); continue; }
    final pts = RegExp(r'Offset\(([^)]+)\)')
        .allMatches(m.group(1)!)
        .map((m) {
      final parts = m.group(1)!.split(', ');
      return [double.parse(parts[0]), double.parse(parts[1])];
    }).toList();
    final xs = pts.map((p) => p[0]).toList()..sort();
    final ys = pts.map((p) => p[1]).toList()..sort();

    var area = 0.0;
    for (var i = 0; i < pts.length; i++) {
      final j = (i + 1) % pts.length;
      area += pts[i][0] * pts[j][1];
      area -= pts[j][0] * pts[i][1];
    }
    area = area.abs() / 2;

    print('$id: ${pts.length} pts, bbox x=${xs.first.toStringAsFixed(0)}-${xs.last.toStringAsFixed(0)}, '
        'y=${ys.first.toStringAsFixed(0)}-${ys.last.toStringAsFixed(0)}, area=${area.toStringAsFixed(0)}');
  }
}
