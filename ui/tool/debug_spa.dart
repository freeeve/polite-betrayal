import 'dart:io';
import 'dart:math';

/// Debug: trace faces near Spain to find if it has multiple sub-regions.
void main() {
  final provinceSrc = File('lib/core/map/province_data.dart').readAsStringSync();
  final centers = _parseProvinceCenters(provinceSrc);

  final svgSrc = File('assets/map/diplomacy_map_hq.svg').readAsStringSync();
  final graph = _PlanarGraph();

  for (final layer in ['layer2', 'layer6']) {
    final content = _extractLayerContent(svgSrc, layer);
    _addPathEdges(graph, content);
  }

  graph.addBoundaryRect(0, 0, 1152, 1152);
  graph.addEdge(graph.addVertex(983.0, 881.0), graph.addVertex(960.3, 954.6));
  graph.buildSortedAdjacency();

  // Denmark probes
  _dumpFace(graph, 'den-center', centers['den']!, centers);
  _dumpFace(graph, 'den-west', [460, 510], centers);
  _dumpFace(graph, 'den-nw', [450, 490], centers);
  _dumpFace(graph, 'den-sw', [470, 540], centers);

  // Bulgaria probes
  _dumpFace(graph, 'bul-center', centers['bul']!, centers);
  _dumpFace(graph, 'bul-south', [780, 920], centers);
  _dumpFace(graph, 'bul-east', [810, 900], centers);

  // Ankara probes
  _dumpFace(graph, 'ank-center', centers['ank']!, centers);
  _dumpFace(graph, 'ank-north', [950, 870], centers);
  _dumpFace(graph, 'ank-south', [940, 950], centers);
  _dumpFace(graph, 'ank-se', [970, 940], centers);
  _dumpFace(graph, 'ank-nw', [910, 890], centers);
}

void _dumpFace(_PlanarGraph graph, String label, List<double> center,
    Map<String, List<double>> allCenters) {
  final face = graph.traceFace(center[0], center[1]);
  if (face == null) { print('$label: no face'); return; }

  var area = 0.0;
  for (var i = 0; i < face.length; i++) {
    final j = (i + 1) % face.length;
    area += face[i][0] * face[j][1];
    area -= face[j][0] * face[i][1];
  }
  area = area.abs() / 2;

  final xs = face.map((p) => p[0]).toList()..sort();
  final ys = face.map((p) => p[1]).toList()..sort();

  print('\n$label (${face.length} pts, area=${area.toStringAsFixed(0)}):');
  print('  bbox: x=${xs.first.toStringAsFixed(0)}-${xs.last.toStringAsFixed(0)}, '
      'y=${ys.first.toStringAsFixed(0)}-${ys.last.toStringAsFixed(0)}');

  // Show which province centers are inside
  print('  Province centers inside:');
  for (final e in allCenters.entries) {
    if (_pointInPolygon(e.value[0], e.value[1], face)) {
      print('    ${e.key} (${e.value[0]}, ${e.value[1]})');
    }
  }

  // Show vertices with nearest province
  for (var i = 0; i < face.length; i++) {
    final x = face[i][0], y = face[i][1];
    var nearest = '';
    var nd = double.infinity;
    for (final e in allCenters.entries) {
      final dx = e.value[0] - x, dy = e.value[1] - y;
      final d = sqrt(dx * dx + dy * dy);
      if (d < nd) { nd = d; nearest = e.key; }
    }
    print('  [$i] (${x.toStringAsFixed(1)}, ${y.toStringAsFixed(1)}) nearest=$nearest (${nd.toStringAsFixed(0)})');
  }
}

// ================================================================
// Infrastructure (same as gen_polygons.dart)
// ================================================================

class _PlanarGraph {
  final List<List<double>> _v = [];
  final Map<int, Set<int>> _adj = {};
  Map<int, List<int>> _sorted = {};
  static const _snap = 2.0;
  static const _snap2 = _snap * _snap;

  int addVertex(double x, double y) {
    for (var i = 0; i < _v.length; i++) {
      final dx = _v[i][0] - x, dy = _v[i][1] - y;
      if (dx * dx + dy * dy < _snap2) return i;
    }
    _v.add([x, y]);
    return _v.length - 1;
  }

  void addEdge(int u, int v) {
    if (u == v) return;
    (_adj[u] ??= {}).add(v);
    (_adj[v] ??= {}).add(u);
  }

  void addBoundaryRect(double x0, double y0, double x1, double y1) {
    final tol = _snap + 1;
    final sides = List.generate(4, (_) => <int>[]);
    for (var i = 0; i < _v.length; i++) {
      if ((_v[i][1] - y0).abs() < tol) sides[0].add(i);
      if ((_v[i][0] - x1).abs() < tol) sides[1].add(i);
      if ((_v[i][1] - y1).abs() < tol) sides[2].add(i);
      if ((_v[i][0] - x0).abs() < tol) sides[3].add(i);
    }
    final c = [
      addVertex(x0, y0), addVertex(x1, y0),
      addVertex(x1, y1), addVertex(x0, y1),
    ];
    sides[0].addAll([c[0], c[1]]);
    sides[1].addAll([c[1], c[2]]);
    sides[2].addAll([c[2], c[3]]);
    sides[3].addAll([c[3], c[0]]);
    for (var s = 0; s < 4; s++) {
      final ax = s.isEven ? 0 : 1;
      final unique = sides[s].toSet().toList()
        ..sort((a, b) => _v[a][ax].compareTo(_v[b][ax]));
      for (var i = 0; i < unique.length - 1; i++) {
        addEdge(unique[i], unique[i + 1]);
      }
    }
  }

  void buildSortedAdjacency() {
    _sorted = {};
    for (final u in _adj.keys) {
      final nbrs = _adj[u]!.toList();
      final ux = _v[u][0], uy = _v[u][1];
      nbrs.sort((a, b) {
        final aa = atan2(_v[a][1] - uy, _v[a][0] - ux);
        final ab = atan2(_v[b][1] - uy, _v[b][0] - ux);
        return aa.compareTo(ab);
      });
      _sorted[u] = nbrs;
    }
  }

  List<List<double>>? traceFace(double px, double py) {
    int bU = -1, bV = -1;
    var bD = double.infinity;
    for (final u in _adj.keys) {
      for (final v in _adj[u]!) {
        if (u >= v) continue;
        final d = _segDist2(px, py, _v[u], _v[v]);
        if (d < bD) { bD = d; bU = u; bV = v; }
      }
    }
    if (bU < 0) return null;
    final cross = (_v[bV][0] - _v[bU][0]) * (py - _v[bU][1]) -
                  (_v[bV][1] - _v[bU][1]) * (px - _v[bU][0]);
    var cU = cross >= 0 ? bU : bV;
    var cV = cross >= 0 ? bV : bU;
    final sU = cU, sV = cV;
    final face = <int>[cU];
    for (var step = 0; step < 5000; step++) {
      face.add(cV);
      final nbrs = _sorted[cV];
      if (nbrs == null) return null;
      final idx = nbrs.indexOf(cU);
      if (idx < 0) return null;
      cU = cV;
      cV = nbrs[(idx - 1 + nbrs.length) % nbrs.length];
      if (cU == sU && cV == sV) break;
    }
    if (face.length > 1 && face.last == face.first) face.removeLast();
    return face.map((i) => [_v[i][0], _v[i][1]]).toList();
  }

  double _segDist2(double px, double py, List<double> a, List<double> b) {
    final dx = b[0] - a[0], dy = b[1] - a[1];
    final len2 = dx * dx + dy * dy;
    if (len2 < 1e-12) {
      final ex = px - a[0], ey = py - a[1];
      return ex * ex + ey * ey;
    }
    final t = ((px - a[0]) * dx + (py - a[1]) * dy) / len2;
    final ct = t.clamp(0.0, 1.0);
    final ex = px - (a[0] + ct * dx), ey = py - (a[1] + ct * dy);
    return ex * ex + ey * ey;
  }
}

String _extractLayerContent(String svg, String layerId) {
  final tag = 'id="$layerId"';
  final start = svg.indexOf(tag);
  if (start == -1) return '';
  final end = svg.indexOf('</g>', start);
  if (end == -1) return '';
  return svg.substring(start, end);
}

int _addPathEdges(_PlanarGraph graph, String content) {
  var total = 0;
  final pathRe = RegExp(r'<path\s[^>]*?/>', dotAll: true);
  for (final match in pathRe.allMatches(content)) {
    final elem = match.group(0)!;
    if (_isDashed(elem)) continue;
    final dMatch = RegExp(r'd="([^"]*)"').firstMatch(elem);
    if (dMatch == null) continue;
    List<double>? matrix;
    final tMatch = RegExp(r'transform="matrix\(([^)]+)\)"').firstMatch(elem);
    if (tMatch != null) {
      matrix = tMatch.group(1)!
          .split(RegExp(r'[,\s]+'))
          .map((s) => double.parse(s.trim()))
          .toList();
    }
    total += _parsePathEdges(graph, dMatch.group(1)!, matrix);
  }
  return total;
}

bool _isDashed(String elem) {
  final m = RegExp(r'stroke-dasharray:([^;]+)').firstMatch(elem);
  if (m == null) return false;
  return m.group(1)!.trim() != 'none';
}

int _parsePathEdges(_PlanarGraph g, String d, List<double>? mat) {
  final tokens = <String>[];
  final re = RegExp(r'[MmLlHhVvZzCcSsQqTtAa]|[-+]?\d*\.?\d+(?:[eE][-+]?\d+)?');
  for (final m in re.allMatches(d)) tokens.add(m.group(0)!);
  var edges = 0;
  var cx = 0.0, cy = 0.0, sx = 0.0, sy = 0.0;
  int? prev, start;
  var i = 0;
  var cmd = '';

  int vtx(double x, double y) {
    if (mat != null) {
      return g.addVertex(mat[0] * x + mat[2] * y + mat[4], mat[1] * x + mat[3] * y + mat[5]);
    }
    return g.addVertex(x, y);
  }
  void doMove(double x, double y) {
    cx = x; cy = y; sx = x; sy = y;
    final v = vtx(x, y); prev = v; start = v;
  }
  void doLine(double x, double y) {
    cx = x; cy = y;
    final v = vtx(x, y);
    if (prev != null && prev != v) { g.addEdge(prev!, v); edges++; }
    prev = v;
  }
  void doClose() {
    if (prev != null && start != null && prev != start) { g.addEdge(prev!, start!); edges++; }
    cx = sx; cy = sy; prev = start;
  }

  while (i < tokens.length) {
    final t = tokens[i];
    if (t.length == 1 && RegExp(r'[A-Za-z]').hasMatch(t)) {
      cmd = t; i++;
      if (cmd == 'Z' || cmd == 'z') { doClose(); continue; }
      continue;
    }
    if (cmd.isEmpty || i >= tokens.length) { i++; continue; }
    switch (cmd) {
      case 'M':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doClose();
        doMove(double.parse(tokens[i]), double.parse(tokens[i + 1])); i += 2; cmd = 'L';
      case 'm':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doClose();
        doMove(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1])); i += 2; cmd = 'l';
      case 'L':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i]), double.parse(tokens[i + 1])); i += 2;
      case 'l':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1])); i += 2;
      case 'H': doLine(double.parse(tokens[i]), cy); i++;
      case 'h': doLine(cx + double.parse(tokens[i]), cy); i++;
      case 'V': doLine(cx, double.parse(tokens[i])); i++;
      case 'v': doLine(cx, cy + double.parse(tokens[i])); i++;
      case 'C':
        if (i + 5 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 4]), double.parse(tokens[i + 5])); i += 6;
      case 'c':
        if (i + 5 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 4]); final ey = cy + double.parse(tokens[i + 5]);
        i += 6; doLine(ex, ey);
      case 'S': case 'Q':
        if (i + 3 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 2]), double.parse(tokens[i + 3])); i += 4;
      case 's': case 'q':
        if (i + 3 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 2]); final ey = cy + double.parse(tokens[i + 3]);
        i += 4; doLine(ex, ey);
      case 'T':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i]), double.parse(tokens[i + 1])); i += 2;
      case 't':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1])); i += 2;
      case 'A':
        if (i + 6 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 5]), double.parse(tokens[i + 6])); i += 7;
      case 'a':
        if (i + 6 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 5]); final ey = cy + double.parse(tokens[i + 6]);
        i += 7; doLine(ex, ey);
      default: i++;
    }
  }
  doClose();
  return edges;
}

Map<String, List<double>> _parseProvinceCenters(String source) {
  final result = <String, List<double>>{};
  final re = RegExp(
      r"'(\w+)':\s*const Province\([^)]*center:\s*Offset\(\s*([\d.]+)\s*,\s*([\d.]+)\s*\)");
  for (final match in re.allMatches(source)) {
    result[match.group(1)!] = [
      double.parse(match.group(2)!),
      double.parse(match.group(3)!),
    ];
  }
  return result;
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
