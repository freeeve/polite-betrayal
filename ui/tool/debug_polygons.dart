import 'dart:io';

void main() {
  final src = File('lib/core/map/province_polygons.dart').readAsStringSync();

  String extract(String id) {
    final re = RegExp("'$id':\\s*\\[(.+?)\\],");
    final m = re.firstMatch(src);
    if (m == null) return 'NOT FOUND';
    return m.group(1)!;
  }

  List<String> points(String data) {
    return RegExp(r'Offset\(([^)]+)\)')
        .allMatches(data)
        .map((m) => m.group(1)!)
        .toList();
  }

  for (final id in ['fin', 'swe', 'con', 'arm', 'wal', 'den', 'iri', 'eng', 'nth', 'ska', 'hel', 'bal']) {
    final data = extract(id);
    final pts = points(data);
    print('$id: ${pts.length} vertices');
  }

  // Check shared vertices between fin and swe
  final finPts = points(extract('fin')).toSet();
  final swePts = points(extract('swe')).toSet();
  final shared = finPts.intersection(swePts);
  print('\nfin-swe shared vertices: ${shared.length}');
  for (final v in shared) print('  shared: Offset($v)');

  // Check polygon area approximation
  for (final id in ['fin', 'swe', 'con', 'arm', 'wal', 'den', 'iri', 'eng', 'nth', 'ska', 'hel', 'bal']) {
    final pts = points(extract(id));
    final xs = pts.map((p) => double.parse(p.split(', ')[0])).toList();
    final ys = pts.map((p) => double.parse(p.split(', ')[1])).toList();
    xs.sort();
    ys.sort();
    print('$id bbox: x=${xs.first.toStringAsFixed(0)}-${xs.last.toStringAsFixed(0)}, '
        'y=${ys.first.toStringAsFixed(0)}-${ys.last.toStringAsFixed(0)}');
  }
}
