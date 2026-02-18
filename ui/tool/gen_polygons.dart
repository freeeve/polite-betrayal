import 'dart:io';
import 'dart:math';

/// Generates province_polygons.dart by building a planar graph from all
/// SVG border edges and tracing the face containing each province center.
///
/// Run from ui/ directory:  dart run tool/gen_polygons.dart
void main() {
  final provinceSrc =
      File('lib/core/map/province_data.dart').readAsStringSync();
  final centers = _parseProvinceCenters(provinceSrc);
  print('Parsed ${centers.length} province centers');

  final svgSrc = File('assets/map/diplomacy_map_hq.svg').readAsStringSync();
  final graph = PlanarGraph();

  for (final layer in ['layer2', 'layer6']) {
    final content = _extractLayerContent(svgSrc, layer);
    final n = _addPathEdges(graph, content);
    print('$layer: $n edges');
  }

  graph.addBoundaryRect(0, 0, 1152, 1152);

  // Armenia/Ankara divider: polygon46 draws the combined ank+arm outline
  // without an internal border. Add the dividing edge between two existing
  // polygon46 vertices so each center traces its own face.
  graph.addEdge(
    graph.addVertex(983.0, 881.0), // top of divider
    graph.addVertex(960.3, 954.6), // bottom of divider
  );

  graph.buildSortedAdjacency();
  print('Graph: ${graph.vertexCount} vertices, ${graph.edgeCount} edges');

  final matched = <String, List<List<double>>>{};
  final missed = <String>[];

  for (final e in centers.entries) {
    var face = graph.traceFace(e.value[0], e.value[1]);
    if (face != null && face.length >= 3 && _hasDuplicateVertices(face)) {
      print('  ${e.key}: de-pinching self-crossing face (${face.length} pts)');
      face = _depinchFace(face, e.value[0], e.value[1]);
    }
    if (face != null && face.length >= 3 && _polygonArea(face) < 300000) {
      matched[e.key] = face;
    } else {
      missed.add(e.key);
      final reason = face == null
          ? 'no face'
          : 'area=${_polygonArea(face).toStringAsFixed(0)}';
      print('  ${e.key}: miss ($reason)');
    }
  }

  // Spain: the SVG splits Spain into 3 adjacent sub-regions (south, north,
  // and a triangular gap between them). Merge by walking exterior boundary.
  if (matched.containsKey('spa')) {
    final faces = [matched['spa']!];
    for (final probe in [const [150, 810], const [200, 845]]) {
      final f = graph.traceFace(probe[0].toDouble(), probe[1].toDouble());
      if (f != null && f.length >= 3) faces.add(f);
    }
    if (faces.length > 1) {
      matched['spa'] = _mergeAdjacentFaces(faces);
      print('  spa: merged ${faces.length} sub-regions (${matched['spa']!.length} pts)');
    }
  }

  // Constantinople: the SVG draws European and Asian sides as separate
  // sub-paths. Trace the Asian face separately and merge with a bridge.
  if (matched.containsKey('con')) {
    final asianFace = graph.traceFace(827, 950);
    if (asianFace != null && asianFace.length >= 3) {
      matched['con'] = _mergePolygons(matched['con']!, asianFace);
      print('  con: merged European + Asian sides (${matched['con']!.length} pts)');
    }
  }

  // Armenia: polygon46 was split by the manual divider, but polygon60 (the
  // eastern extension to the map edge) is a separate face. Merge it in.
  if (matched.containsKey('arm')) {
    var eastFace = graph.traceFace(1100, 910);
    if (eastFace != null && _hasDuplicateVertices(eastFace)) {
      eastFace = _depinchFace(eastFace, 1100, 910);
    }
    if (eastFace != null && eastFace.length >= 3) {
      matched['arm'] = _mergePolygons(matched['arm']!, eastFace);
      print('  arm: merged with eastern extension (${matched['arm']!.length} pts)');
    }
  }

  // Denmark: SVG splits Denmark into Jutland + islands. Keep separate
  // so the sea strait between them is visible.
  final multiPoly = <String, List<List<List<double>>>>{};
  if (matched.containsKey('den')) {
    final faces = [matched['den']!];
    final sw = graph.traceFace(470, 540);
    if (sw != null && sw.length >= 3) faces.add(sw);
    if (faces.length > 1) {
      multiPoly['den'] = faces;
      print('  den: kept ${faces.length} separate sub-regions');
    }
  }

  // Bulgaria: main body + southern coast faces.
  if (matched.containsKey('bul')) {
    final faces = [matched['bul']!];
    for (final probe in [const [780, 920], const [720, 920], const [750, 935]]) {
      final f = graph.traceFace(probe[0].toDouble(), probe[1].toDouble());
      if (f != null && f.length >= 3) faces.add(f);
    }
    if (faces.length > 1) {
      matched['bul'] = _mergeAdjacentFaces(faces);
      print('  bul: merged ${faces.length} sub-regions (${matched['bul']!.length} pts)');
    }
  }

  // St. Petersburg: Kola Peninsula is handled as a manual second ring
  // in province_polygons.dart (polygon72 from SVG layer6). The generator
  // can't auto-detect it because the Kola face centroid falls inside
  // BAR's polygon. See province_polygons.dart for the multi-ring STP entry.

  // Ankara: main body + SE piece beyond the arm/ank divider.
  if (matched.containsKey('ank')) {
    final faces = [matched['ank']!];
    final se = graph.traceFace(970, 940);
    if (se != null && se.length >= 3) faces.add(se);
    if (faces.length > 1) {
      matched['ank'] = _mergeAdjacentFaces(faces);
      print('  ank: merged ${faces.length} sub-regions (${matched['ank']!.length} pts)');
    }
  }
  // Split shared sea zone polygons. The SVG has no solid borders between
  // some adjacent sea zones (e.g. nth/ska/hel/bal), so they trace to the
  // same face. Detect this and split by nearest-center assignment.
  _splitSharedPolygons(matched, centers);

  print('Face-traced: ${matched.length}/${centers.length}');

  if (missed.isNotEmpty) {
    print('Unmatched: ${missed.join(', ')}');
  }

  _writeOutput(matched, multiPoly);
}

// ================================================================
// Planar graph with face tracing
// ================================================================

class PlanarGraph {
  final List<List<double>> _v = [];
  final Map<int, Set<int>> _adj = {};
  Map<int, List<int>> _sorted = {};
  static const _snap = 2.0;
  static const _snap2 = _snap * _snap;

  int get vertexCount => _v.length;
  int get edgeCount {
    var n = 0;
    for (final s in _adj.values) n += s.length;
    return n ~/ 2;
  }

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

  /// Adds boundary edges along the viewBox rectangle, connecting existing
  /// vertices near each boundary edge.
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

  /// Traces the face containing point (px, py) by walking half-edges.
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

    // Pick starting half-edge so the point is on the left.
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

// ================================================================
// SVG parsing
// ================================================================

String _extractLayerContent(String svg, String layerId) {
  final tag = 'id="$layerId"';
  final start = svg.indexOf(tag);
  if (start == -1) return '';
  final end = svg.indexOf('</g>', start);
  if (end == -1) return '';
  return svg.substring(start, end);
}

/// Adds edges from path elements to the graph. Dashed paths are skipped
/// unless [skipDashed] is false (used for sea-zone boundary layers).
int _addPathEdges(PlanarGraph graph, String content, {bool skipDashed = true}) {
  var total = 0;
  final pathRe = RegExp(r'<path\s[^>]*?/>', dotAll: true);
  for (final match in pathRe.allMatches(content)) {
    final elem = match.group(0)!;
    if (skipDashed && _isDashed(elem)) continue;
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

/// Parses SVG path commands into graph edges. Returns number of edges added.
int _parsePathEdges(PlanarGraph g, String d, List<double>? mat) {
  final tokens = <String>[];
  final re = RegExp(
      r'[MmLlHhVvZzCcSsQqTtAa]|[-+]?\d*\.?\d+(?:[eE][-+]?\d+)?');
  for (final m in re.allMatches(d)) tokens.add(m.group(0)!);

  var edges = 0;
  var cx = 0.0, cy = 0.0, sx = 0.0, sy = 0.0;
  int? prev, start;
  var i = 0;
  var cmd = '';

  int vtx(double x, double y) {
    if (mat != null) {
      return g.addVertex(
        mat[0] * x + mat[2] * y + mat[4],
        mat[1] * x + mat[3] * y + mat[5],
      );
    }
    return g.addVertex(x, y);
  }

  void doMove(double x, double y) {
    cx = x; cy = y; sx = x; sy = y;
    final v = vtx(x, y);
    prev = v; start = v;
  }

  void doLine(double x, double y) {
    cx = x; cy = y;
    final v = vtx(x, y);
    if (prev != null && prev != v) { g.addEdge(prev!, v); edges++; }
    prev = v;
  }

  void doClose() {
    if (prev != null && start != null && prev != start) {
      g.addEdge(prev!, start!);
      edges++;
    }
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
        doClose(); // auto-close previous sub-path if open
        doMove(double.parse(tokens[i]), double.parse(tokens[i + 1]));
        i += 2; cmd = 'L';
      case 'm':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doClose(); // auto-close previous sub-path if open
        doMove(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1]));
        i += 2; cmd = 'l';
      case 'L':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i]), double.parse(tokens[i + 1])); i += 2;
      case 'l':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1]));
        i += 2;
      case 'H': doLine(double.parse(tokens[i]), cy); i++;
      case 'h': doLine(cx + double.parse(tokens[i]), cy); i++;
      case 'V': doLine(cx, double.parse(tokens[i])); i++;
      case 'v': doLine(cx, cy + double.parse(tokens[i])); i++;
      case 'C':
        if (i + 5 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 4]), double.parse(tokens[i + 5]));
        i += 6;
      case 'c':
        if (i + 5 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 4]);
        final ey = cy + double.parse(tokens[i + 5]);
        i += 6; doLine(ex, ey);
      case 'S': case 'Q':
        if (i + 3 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 2]), double.parse(tokens[i + 3]));
        i += 4;
      case 's': case 'q':
        if (i + 3 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 2]);
        final ey = cy + double.parse(tokens[i + 3]);
        i += 4; doLine(ex, ey);
      case 'T':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i]), double.parse(tokens[i + 1])); i += 2;
      case 't':
        if (i + 1 >= tokens.length) { i = tokens.length; break; }
        doLine(cx + double.parse(tokens[i]), cy + double.parse(tokens[i + 1]));
        i += 2;
      case 'A':
        if (i + 6 >= tokens.length) { i = tokens.length; break; }
        doLine(double.parse(tokens[i + 5]), double.parse(tokens[i + 6]));
        i += 7;
      case 'a':
        if (i + 6 >= tokens.length) { i = tokens.length; break; }
        final ex = cx + double.parse(tokens[i + 5]);
        final ey = cy + double.parse(tokens[i + 6]);
        i += 7; doLine(ex, ey);
      default:
        i++;
    }
  }
  doClose(); // auto-close last sub-path if open
  return edges;
}

// ================================================================
// Helpers
// ================================================================

/// Returns true if any vertex coordinate pair appears more than once,
/// indicating a self-crossing face (e.g. wrapping around coastal features).
bool _hasDuplicateVertices(List<List<double>> face) {
  final seen = <String>{};
  for (final v in face) {
    final key = '${v[0].toStringAsFixed(1)},${v[1].toStringAsFixed(1)}';
    if (!seen.add(key)) return true;
  }
  return false;
}

/// Merges two polygons by connecting them at their closest vertex pair.
/// Returns a single polygon: polyA up to closest-A → closest-B → polyB → closest-B → closest-A → rest of polyA.
List<List<double>> _mergePolygons(List<List<double>> a, List<List<double>> b) {
  var bestDist = double.infinity;
  var ai = 0, bi = 0;
  for (var i = 0; i < a.length; i++) {
    for (var j = 0; j < b.length; j++) {
      final dx = a[i][0] - b[j][0], dy = a[i][1] - b[j][1];
      final d = dx * dx + dy * dy;
      if (d < bestDist) { bestDist = d; ai = i; bi = j; }
    }
  }
  // Build merged polygon: A[0..ai] → bridge → B[bi..bi-1 wrapping] → bridge back → A[ai+1..end]
  final result = <List<double>>[];
  for (var i = 0; i <= ai; i++) result.add(a[i]);
  for (var i = 0; i < b.length; i++) result.add(b[(bi + i) % b.length]);
  result.add(b[bi]); // close B loop back to bridge point
  result.add(a[ai]); // bridge back
  for (var i = ai + 1; i < a.length; i++) result.add(a[i]);
  return result;
}

/// Merges adjacent faces that share edges into a single polygon by walking
/// only the exterior boundary (edges that appear in exactly one face).
List<List<double>> _mergeAdjacentFaces(List<List<List<double>>> faces) {
  String vKey(List<double> v) =>
      '${v[0].toStringAsFixed(1)},${v[1].toStringAsFixed(1)}';

  // Collect all directed edges; count undirected occurrences.
  final edgeCount = <String, int>{};
  final exteriorNext = <String, String>{}; // from_key → to_key
  final vertexMap = <String, List<double>>{};

  for (final face in faces) {
    for (var i = 0; i < face.length; i++) {
      final j = (i + 1) % face.length;
      final a = vKey(face[i]), b = vKey(face[j]);
      vertexMap[a] = face[i];
      vertexMap[b] = face[j];
      final undirected = a.compareTo(b) < 0 ? '$a|$b' : '$b|$a';
      edgeCount[undirected] = (edgeCount[undirected] ?? 0) + 1;
    }
  }

  // Keep only exterior edges (count == 1).
  for (final face in faces) {
    for (var i = 0; i < face.length; i++) {
      final j = (i + 1) % face.length;
      final a = vKey(face[i]), b = vKey(face[j]);
      final undirected = a.compareTo(b) < 0 ? '$a|$b' : '$b|$a';
      if (edgeCount[undirected] == 1) {
        exteriorNext[a] = b;
      }
    }
  }

  if (exteriorNext.isEmpty) return faces.first;

  // Walk the exterior boundary.
  final startKey = exteriorNext.keys.first;
  final result = <List<double>>[vertexMap[startKey]!];
  var current = exteriorNext[startKey]!;
  for (var safety = 0; safety < 5000 && current != startKey; safety++) {
    result.add(vertexMap[current]!);
    final next = exteriorNext[current];
    if (next == null) break;
    current = next;
  }
  return result;
}

double _polygonArea(List<List<double>> poly) {
  var area = 0.0;
  final n = poly.length;
  for (var i = 0; i < n; i++) {
    final j = (i + 1) % n;
    area += poly[i][0] * poly[j][1];
    area -= poly[j][0] * poly[i][1];
  }
  return area.abs() / 2;
}

/// Ray-casting point-in-polygon test.
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

/// Splits a self-crossing face at duplicate vertices and returns the
/// simple sub-polygon containing the province center (px, py).
List<List<double>> _depinchFace(List<List<double>> face, double px, double py) {
  var result = face;
  var changed = true;
  while (changed) {
    changed = false;
    final coordIndex = <String, int>{};
    for (var i = 0; i < result.length; i++) {
      final key = '${result[i][0].toStringAsFixed(1)},${result[i][1].toStringAsFixed(1)}';
      if (coordIndex.containsKey(key)) {
        final firstIdx = coordIndex[key]!;
        // Split into the loop (firstIdx..i) and the remainder
        final subLoop = result.sublist(firstIdx, i);
        final remainder = [...result.sublist(0, firstIdx), ...result.sublist(i)];
        // Keep the part containing the province center
        if (subLoop.length >= 3 && _pointInPolygon(px, py, subLoop)) {
          result = subLoop;
        } else if (remainder.length >= 3 && _pointInPolygon(px, py, remainder)) {
          result = remainder;
        } else if (subLoop.length >= 3) {
          // Neither contains center — keep the smaller one (more likely a province)
          result = subLoop.length <= remainder.length ? subLoop : remainder;
        } else {
          break;
        }
        changed = true;
        break;
      }
      coordIndex[key] = i;
    }
  }
  return result;
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

/// Detects polygons that contain multiple province centers and splits them
/// into separate sub-polygons using nearest-center assignment.
void _splitSharedPolygons(
  Map<String, List<List<double>>> matched,
  Map<String, List<double>> centers,
) {
  final processed = <String>{};
  for (final id in matched.keys.toList()) {
    if (processed.contains(id)) continue;
    final poly = matched[id]!;
    final group = <String>[id];
    for (final other in matched.keys) {
      if (other == id || processed.contains(other)) continue;
      // Mutual containment: both centers must be inside each other's polygon.
      // This prevents land provinces (with their own polygon) from being
      // included just because their center is geographically inside a sea face.
      if (_pointInPolygon(centers[other]![0], centers[other]![1], poly) &&
          _pointInPolygon(centers[id]![0], centers[id]![1], matched[other]!)) {
        group.add(other);
      }
    }
    if (group.length <= 1) continue;

    // Use the polygon with the most vertices (best boundary)
    var bestPoly = poly;
    for (final g in group) {
      if (matched[g]!.length > bestPoly.length) bestPoly = matched[g]!;
    }

    final groupCenters = <String, List<double>>{};
    for (final g in group) {
      groupCenters[g] = centers[g]!;
    }

    final split = _splitPolygonByNearestCenter(bestPoly, groupCenters);
    for (final e in split.entries) {
      matched[e.key] = e.value;
      processed.add(e.key);
    }
    print('  Split shared polygon into: ${group.join(', ')} '
        '(${group.map((g) => '${matched[g]!.length} pts').join(', ')})');
  }
}

/// Splits a polygon containing multiple province centers into sub-polygons.
/// Walks the boundary, assigning each vertex to its nearest center. At
/// transitions between centers, interpolates the Voronoi boundary point.
Map<String, List<List<double>>> _splitPolygonByNearestCenter(
  List<List<double>> polygon,
  Map<String, List<double>> centers,
) {
  String nearestCenter(double x, double y) {
    String best = '';
    double bestDist = double.infinity;
    for (final e in centers.entries) {
      final dx = e.value[0] - x, dy = e.value[1] - y;
      final d = dx * dx + dy * dy;
      if (d < bestDist) {
        bestDist = d;
        best = e.key;
      }
    }
    return best;
  }

  final assignments =
      polygon.map((p) => nearestCenter(p[0], p[1])).toList();

  final subPolygons = <String, List<List<double>>>{};
  for (final id in centers.keys) {
    subPolygons[id] = [];
  }

  for (var i = 0; i < polygon.length; i++) {
    final j = (i + 1) % polygon.length;
    final ci = assignments[i];
    final cj = assignments[j];

    subPolygons[ci]!.add(polygon[i]);

    if (ci != cj) {
      // Interpolate the Voronoi boundary point on segment p→q.
      // Find t where dist(lerp(t), centerA) == dist(lerp(t), centerB).
      final p = polygon[i], q = polygon[j];
      final cA = centers[ci]!, cB = centers[cj]!;
      final ex = q[0] - p[0], ey = q[1] - p[1];
      final fx = cA[0] - cB[0], fy = cA[1] - cB[1];
      final mx = (cA[0] + cB[0]) / 2, my = (cA[1] + cB[1]) / 2;
      final gx = p[0] - mx, gy = p[1] - my;
      final denom = ex * fx + ey * fy;
      var t = 0.5;
      if (denom.abs() > 1e-12) {
        t = -(gx * fx + gy * fy) / denom;
        t = t.clamp(0.01, 0.99);
      }
      final midPt = [p[0] + t * ex, p[1] + t * ey];
      subPolygons[ci]!.add(midPt);
      subPolygons[cj]!.add(midPt);
    }
  }

  return subPolygons;
}

void _writeOutput(
  Map<String, List<List<double>>> matched,
  Map<String, List<List<List<double>>>> multiPoly,
) {
  const w = 1152.0, h = 1152.0;

  String fmtRing(List<List<double>> ring) {
    return ring.map((p) {
      final x = p[0].clamp(0.0, w);
      final y = p[1].clamp(0.0, h);
      return 'Offset(${x.toStringAsFixed(1)}, ${y.toStringAsFixed(1)})';
    }).join(', ');
  }

  final buf = StringBuffer()
    ..writeln("import 'dart:ui';")
    ..writeln()
    ..writeln('/// Province polygon boundaries for territory shading.')
    ..writeln('/// Each province maps to a list of polygon rings (most have one).')
    ..writeln('/// Generated by tool/gen_polygons.dart — do not edit by hand.')
    ..writeln('final Map<String, List<List<Offset>>> provincePolygons = {');

  final keys = matched.keys.toList()..sort();
  for (final id in keys) {
    if (multiPoly.containsKey(id)) {
      final rings = multiPoly[id]!;
      buf.writeln("  '$id': [");
      for (final ring in rings) {
        buf.writeln('    [${fmtRing(ring)}],');
      }
      buf.writeln('  ],');
    } else {
      buf.writeln("  '$id': [[${fmtRing(matched[id]!)}]],");
    }
  }
  buf.writeln('};');

  final outFile = File('lib/core/map/province_polygons.dart');
  outFile.writeAsStringSync(buf.toString());
  print('Wrote ${outFile.path} (${keys.length} provinces)');
}
