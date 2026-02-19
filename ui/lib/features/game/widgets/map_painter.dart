import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../../core/map/adjacency_data.dart' show allAdjacent;
import '../../../core/map/province_data.dart';
import '../../../core/map/province_polygons.dart';
import '../../../core/models/game_state.dart';
import '../../../core/models/order.dart';
import '../../../core/theme/app_theme.dart';

/// Draws units, supply center dots, highlights, and order arrows on top of the SVG map.
/// All coordinates are in SVG viewBox space (1152x1152) and scaled to widget size via canvas transform.
class MapPainter extends CustomPainter {
  final GameState? gameState;
  final GameState? previousGameState;
  final double? animationProgress;
  final String? selectedProvince;
  final Set<String> validTargets;
  final List<PendingOrder> pendingOrders;
  final List<Order>? resolvedOrders;
  final String? myPower;
  final Set<String> newUnitProvinces;

  /// Build/disband animation state: provinces where units were built or disbanded,
  /// the animation progress (0-1), and ghost units for disbanded units that are
  /// no longer in the current game state.
  final Set<String> buildProvinces;
  final Set<String> disbandProvinces;
  final List<GameUnit> disbandUnits;
  final double? buildDisbandProgress;

  MapPainter({
    this.gameState,
    this.previousGameState,
    this.animationProgress,
    this.selectedProvince,
    this.validTargets = const {},
    this.pendingOrders = const [],
    this.resolvedOrders,
    this.myPower,
    this.newUnitProvinces = const {},
    this.buildProvinces = const {},
    this.disbandProvinces = const {},
    this.disbandUnits = const [],
    this.buildDisbandProgress,
  });

  @override
  void paint(Canvas canvas, Size size) {
    if (gameState == null) return;

    canvas.scale(size.width / svgViewBoxWidth, size.height / svgViewBoxHeight);

    _drawTerritoryShading(canvas);
    _drawProvinceLabels(canvas);
    _drawUnits(canvas);
    _drawSupplyCenters(canvas);
    _drawHighlight(canvas);
    _drawDimOverlay(canvas);
    _drawValidTargets(canvas);
    _drawPendingOrders(canvas);
    if (resolvedOrders != null) _drawResolvedOrders(canvas);
  }

  /// Shades provinces by controlling power (45% opacity fill + subtle border).
  /// During animation, uses the pre-resolution state so territories only update
  /// after units finish moving.
  void _drawTerritoryShading(Canvas canvas) {
    if (gameState == null) return;
    final shadeState = (previousGameState != null && animationProgress != null)
        ? previousGameState! : gameState!;
    final powerProvinces = <String, Set<String>>{};
    for (final entry in shadeState.supplyCenters.entries) {
      final power = entry.value;
      if (power.isNotEmpty) {
        powerProvinces.putIfAbsent(power, () => {}).add(entry.key);
      }
    }
    for (final unit in shadeState.units) {
      powerProvinces.putIfAbsent(unit.power, () => {}).add(unit.province);
    }
    final claimed = <String, String>{};
    final queue = <(String, String)>[];
    for (final entry in powerProvinces.entries) {
      for (final prov in entry.value) {
        claimed[prov] = entry.key;
        queue.add((prov, entry.key));
      }
    }

    // Adjacency pairs where the flood fill should not cross.
    // naf/tun: visually distant provinces that share a game adjacency.
    // fin, ukr: large non-SC buffer provinces that border multiple powers
    // and should not tint when an adjacent territory is owned.
    const blockedFloodFill = {
      ('naf', 'tun'), ('tun', 'naf'),
      ('swe', 'fin'), ('fin', 'swe'),
      ('nwy', 'fin'), ('fin', 'nwy'),
      ('stp', 'fin'), ('fin', 'stp'),
      ('rum', 'ukr'), ('ukr', 'rum'),
      ('gal', 'ukr'), ('ukr', 'gal'),
      ('sev', 'ukr'), ('ukr', 'sev'),
      ('war', 'ukr'), ('ukr', 'war'),
      ('mos', 'ukr'), ('ukr', 'mos'),
      ('lon', 'wal'), ('wal', 'lon'),
      ('lvp', 'cly'), ('cly', 'lvp'),
      ('edi', 'cly'), ('cly', 'edi'),
    };

    var i = 0;
    while (i < queue.length) {
      final (prov, power) = queue[i++];
      final sourceProv = provinces[prov];
      if (sourceProv != null && sourceProv.isSea) continue;
      for (final neighbor in allAdjacent(prov)) {
        if (claimed.containsKey(neighbor)) continue;
        if (blockedFloodFill.contains((prov, neighbor))) continue;
        final np = provinces[neighbor];
        if (np == null || np.isSea) continue;
        if (np.isSupplyCenter) continue;
        claimed[neighbor] = power;
        queue.add((neighbor, power));
      }
    }

    final grouped = <String, Set<String>>{};
    for (final entry in claimed.entries) {
      grouped.putIfAbsent(entry.value, () => {}).add(entry.key);
    }

    for (final entry in grouped.entries) {
      final powerColor = PowerColors.forPower(entry.key);

      for (final provId in entry.value) {
        final rings = provincePolygons[provId];
        if (rings == null || rings.isEmpty) continue;

        final isSea = provinces[provId]?.isSea ?? false;
        // Low-saturation colors (brown) and light colors (yellow) need
        // higher opacity to be visible on the light map background.
        final isLowSat = entry.key == 'germany' || entry.key == 'austria';
        final fillAlpha = isSea ? (isLowSat ? 0.40 : 0.18) : (isLowSat ? 0.65 : 0.45);
        final borderAlpha = isSea ? (isLowSat ? 0.30 : 0.12) : (isLowSat ? 0.55 : 0.35);

        for (final polygon in rings) {
          if (polygon.length < 3) continue;
          final path = Path()..moveTo(polygon[0].dx, polygon[0].dy);
          for (var i = 1; i < polygon.length; i++) {
            path.lineTo(polygon[i].dx, polygon[i].dy);
          }
          path.close();
          canvas.drawPath(path, Paint()
            ..color = powerColor.withValues(alpha: fillAlpha)
            ..style = PaintingStyle.fill);
          canvas.drawPath(path, Paint()
            ..color = powerColor.withValues(alpha: borderAlpha)
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.2);
        }
      }
    }
  }

  void _drawSupplyCenters(Canvas canvas) {
    final occupied = _occupiedProvinces;
    for (final entry in provinces.entries) {
      if (!entry.value.isSupplyCenter) continue;
      final owner = gameState!.supplyCenters[entry.key];
      final color = owner != null && owner.isNotEmpty
          ? PowerColors.forPower(owner)
          : Colors.white70;

      final hasUnit = occupied.contains(entry.key);
      final center = hasUnit
          ? entry.value.center + const Offset(0, 3)
          : entry.value.center + const Offset(0, -17);

      final radius = hasUnit ? 6.0 : 8.0;
      canvas.drawCircle(
        center, radius + 2,
        Paint()
          ..color = Colors.black
          ..style = PaintingStyle.fill,
      );
      canvas.drawCircle(
        center, radius,
        Paint()
          ..color = color
          ..style = PaintingStyle.fill,
      );
    }
  }

  Set<String> get _occupiedProvinces {
    final s = <String>{};
    for (final u in gameState!.units) {
      s.add(u.province);
    }
    return s;
  }

  void _drawProvinceLabels(Canvas canvas) {
    final occupied = _occupiedProvinces;
    for (final entry in provinces.entries) {
      final prov = entry.value;
      final label = entry.key.toUpperCase();
      final isSea = prov.type == ProvinceType.sea;
      final hasUnit = occupied.contains(entry.key);

      final textPainter = TextPainter(
        text: TextSpan(
          text: label,
          style: TextStyle(
            fontSize: 10,
            fontWeight: FontWeight.bold,
            fontStyle: isSea ? FontStyle.italic : FontStyle.normal,
            foreground: Paint()
              ..color = isSea ? const Color(0xCCFFFFFF) : const Color(0xFF333333),
          ),
        ),
        textDirection: TextDirection.ltr,
      )..layout();

      final yOffset = hasUnit ? 10.0 : -textPainter.height / 2;
      final offset = prov.center + Offset(-textPainter.width / 2, yOffset);

      final outlinePainter = TextPainter(
        text: TextSpan(
          text: label,
          style: TextStyle(
            fontSize: 10,
            fontWeight: FontWeight.bold,
            fontStyle: isSea ? FontStyle.italic : FontStyle.normal,
            foreground: Paint()
              ..style = PaintingStyle.stroke
              ..strokeWidth = 2.5
              ..color = isSea ? const Color(0x66000000) : Colors.white,
          ),
        ),
        textDirection: TextDirection.ltr,
      )..layout();
      outlinePainter.paint(canvas, offset);
      textPainter.paint(canvas, offset);
    }
  }

  /// Returns the draw position for a unit of the given type at the given province.
  Offset _unitCenterAt(String province, UnitType type) {
    final prov = provinces[province];
    if (prov == null) return Offset.zero;
    final isSC = prov.isSupplyCenter;
    if (type == UnitType.army) {
      return prov.center + Offset(0, isSC ? -6 : 4);
    } else {
      return prov.center + Offset(0, isSC ? -9 : 2);
    }
  }

  /// Draws units on the map — artillery cannons for armies, ship icons for fleets.
  /// During build/disband animations, built units fade in with a star overlay and
  /// disbanded units fade out with an X overlay.
  void _drawUnits(Canvas canvas) {
    if (previousGameState != null && animationProgress != null) {
      _drawAnimatedUnits(canvas);
      return;
    }

    final isBuildDisbandAnimating = buildDisbandProgress != null
        && (buildProvinces.isNotEmpty || disbandProvinces.isNotEmpty);

    // Draw current units (with fade-in for newly built ones).
    for (final unit in gameState!.units) {
      final prov = provinces[unit.province];
      if (prov == null) continue;

      final color = PowerColors.forPower(unit.power);
      final center = _unitCenterAt(unit.province, unit.type);

      if (isBuildDisbandAnimating && buildProvinces.contains(unit.province)) {
        // Building animation: fade in from 0 to 1.
        final opacity = Curves.easeOut.transform(buildDisbandProgress!);
        canvas.saveLayer(null, Paint()..color = Color.fromRGBO(0, 0, 0, opacity));
        if (unit.type == UnitType.army) {
          _drawArtillery(canvas, center, color);
        } else {
          _drawBattleship(canvas, center, color);
        }
        canvas.restore();
        // Draw animated star overlay that scales and fades.
        _drawBuildStar(canvas, center + const Offset(16, -14), buildDisbandProgress!);
      } else {
        if (unit.type == UnitType.army) {
          _drawArtillery(canvas, center, color);
        } else {
          _drawBattleship(canvas, center, color);
        }

        if (newUnitProvinces.contains(unit.province)) {
          _drawNewUnitStar(canvas, center + const Offset(16, -14));
        }
      }
    }

    // Draw disbanded units (ghost units that fade out with X overlay).
    if (isBuildDisbandAnimating) {
      for (final unit in disbandUnits) {
        final prov = provinces[unit.province];
        if (prov == null) continue;

        final color = PowerColors.forPower(unit.power);
        final center = _unitCenterAt(unit.province, unit.type);

        // Disbanding animation: fade out from 1 to 0.
        final opacity = 1.0 - Curves.easeIn.transform(buildDisbandProgress!);
        canvas.saveLayer(null, Paint()..color = Color.fromRGBO(0, 0, 0, opacity));
        if (unit.type == UnitType.army) {
          _drawArtillery(canvas, center, color);
        } else {
          _drawBattleship(canvas, center, color);
        }
        canvas.restore();
        // Draw X overlay that fades in as unit fades out.
        _drawDisbandX(canvas, center, buildDisbandProgress!);
      }
    }
  }

  /// Draws units at interpolated positions during movement animation.
  /// Units follow the same quadratic Bezier curve that the order arrows use.
  void _drawAnimatedUnits(Canvas canvas) {
    final orderMap = <String, Order>{};
    if (resolvedOrders != null) {
      for (final order in resolvedOrders!) {
        orderMap['${order.power}:${order.location}'] = order;
      }
    }

    for (final unit in previousGameState!.units) {
      if (provinces[unit.province] == null) continue;

      final color = PowerColors.forPower(unit.power);
      final from = _unitCenterAt(unit.province, unit.type);

      final order = orderMap['${unit.power}:${unit.province}'];
      Offset center;

      if (order != null
          && (order.orderType == 'move' || order.orderType == 'retreat_move')
          && order.target != null) {
        final to = _unitCenterAt(order.target!, unit.type);
        final ctrl = _arrowControlPoint(from, to);
        if (order.result == 'succeeds') {
          final eased = Curves.easeInOut.transform(animationProgress!);
          center = _bezierPoint(from, ctrl, to, eased);
        } else {
          // Bounce: follow curve 40% of the way then return along same arc.
          final t = math.sin(animationProgress! * math.pi) * 0.4;
          center = _bezierPoint(from, ctrl, to, t);
        }
      } else if (order != null && order.orderType == 'retreat_disband') {
        // Retreat disband: unit fades out in place with X overlay.
        center = from;
        final opacity = 1.0 - Curves.easeIn.transform(animationProgress!);
        canvas.saveLayer(null, Paint()..color = Color.fromRGBO(0, 0, 0, opacity));
        if (unit.type == UnitType.army) {
          _drawArtillery(canvas, center, color);
        } else {
          _drawBattleship(canvas, center, color);
        }
        canvas.restore();
        _drawDisbandX(canvas, center, animationProgress!);
        continue;
      } else {
        center = from;
      }

      if (unit.type == UnitType.army) {
        _drawArtillery(canvas, center, color);
      } else {
        _drawBattleship(canvas, center, color);
      }

      // Draw shield overlay for hold orders during animation.
      if (order != null && order.orderType == 'hold' && animationProgress != null) {
        _drawHoldShield(canvas, center, animationProgress!, color);
      }
    }
  }

  /// Computes the Bezier control point for the curve between two points,
  /// matching the curvature used by _drawArrow.
  Offset _arrowControlPoint(Offset from, Offset to) {
    final dir = to - from;
    final len = dir.distance;
    if (len < 1) return (from + to) / 2;
    final unitDir = dir / len;
    final perp = Offset(-unitDir.dy, unitDir.dx);
    final mid = (from + to) / 2;
    return mid + perp * (len * 0.18);
  }

  /// Evaluates a quadratic Bezier at parameter t: B(t) = (1-t)^2*p0 + 2(1-t)t*ctrl + t^2*p2.
  Offset _bezierPoint(Offset p0, Offset ctrl, Offset p2, double t) {
    final u = 1 - t;
    return p0 * (u * u) + ctrl * (2 * u * t) + p2 * (t * t);
  }

  /// Draws an artillery cannon icon with carriage, wheels, barrel, and shield detail.
  void _drawArtillery(Canvas canvas, Offset center, Color color) {
    canvas.save();
    canvas.translate(center.dx, center.dy);

    final outline = Paint()
      ..color = Colors.black
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.8;
    final bodyFill = Paint()..color = color;
    final darkBody = Paint()..color = Color.lerp(color, Colors.black, 0.25)!;
    final lightBody = Paint()..color = Color.lerp(color, Colors.white, 0.3)!;
    final metalGray = Paint()..color = const Color(0xFF999999);
    final darkGray = Paint()..color = const Color(0xFF555555);

    // Wheels — two circles at base of carriage.
    canvas.drawCircle(const Offset(-8, 3), 5, darkGray);
    canvas.drawCircle(const Offset(-8, 3), 5, outline);
    canvas.drawCircle(const Offset(-8, 3), 2, metalGray);
    canvas.drawCircle(const Offset(5, 3), 5, darkGray);
    canvas.drawCircle(const Offset(5, 3), 5, outline);
    canvas.drawCircle(const Offset(5, 3), 2, metalGray);

    // Axle bar connecting wheels.
    canvas.drawLine(const Offset(-8, 3), const Offset(5, 3), Paint()
      ..color = const Color(0xFF444444)
      ..strokeWidth = 2.5);

    // Body — blocky trapezoidal housing.
    final body = Path()
      ..moveTo(-14, 0)
      ..lineTo(-8, -12)
      ..lineTo(8, -12)
      ..lineTo(10, 0)
      ..close();
    canvas.drawPath(body, bodyFill);
    canvas.drawPath(body, outline);

    // Shield panel on front of body.
    final shield = Path()
      ..moveTo(-12, -1)
      ..lineTo(-9, -10)
      ..lineTo(-2, -10)
      ..lineTo(-4, -1)
      ..close();
    canvas.drawPath(shield, darkBody);
    canvas.drawPath(shield, outline);

    // Rivet detail on shield.
    canvas.drawCircle(const Offset(-7, -4), 1.2, lightBody);

    // Barrel — angled tube extending upper-right.
    canvas.save();
    canvas.translate(2, -12);
    canvas.rotate(-0.52); // ~30 degrees
    final barrel = Path()
      ..addRect(const Rect.fromLTWH(0, -2.5, 18, 5));
    canvas.drawPath(barrel, darkBody);
    canvas.drawPath(barrel, outline);
    // Barrel reinforcement band.
    final band = Path()
      ..addRect(const Rect.fromLTWH(10, -3, 2.5, 6));
    canvas.drawPath(band, metalGray);
    canvas.drawPath(band, outline);
    // Muzzle tip.
    final muzzle = Path()
      ..addRect(const Rect.fromLTWH(16, -3.5, 4, 7));
    canvas.drawPath(muzzle, metalGray);
    canvas.drawPath(muzzle, outline);
    canvas.restore();

    canvas.restore();
  }

  /// Draws a battleship icon with hull, superstructure, turret, mast, and smokestack detail.
  void _drawBattleship(Canvas canvas, Offset center, Color color) {
    canvas.save();
    canvas.translate(center.dx, center.dy);
    canvas.scale(-1, 1); // flip horizontally
    canvas.rotate(-0.35); // ~20° — bow points upper-right

    final outline = Paint()
      ..color = Colors.black
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.8;
    final hullFill = Paint()..color = color;
    final darkHull = Paint()..color = Color.lerp(color, Colors.black, 0.3)!;
    final rimColor = Paint()..color = Color.lerp(color, Colors.white, 0.45)!;
    final metalGray = Paint()..color = const Color(0xFF999999);
    final darkGray = Paint()..color = const Color(0xFF555555);

    // Hull — boat shape, bow pointing left.
    final hull = Path()
      ..moveTo(-18, 0)      // bow tip
      ..lineTo(-12, -6)     // bow deck edge
      ..lineTo(14, -6)      // deck stern
      ..lineTo(16, -3)      // stern top
      ..lineTo(16, 3)       // stern bottom
      ..quadraticBezierTo(0, 7, -18, 0) // hull bottom curve
      ..close();
    canvas.drawPath(hull, hullFill);
    canvas.drawPath(hull, outline);

    // Hull waterline stripe — darker band along the bottom.
    final waterline = Path()
      ..moveTo(-16, 1)
      ..quadraticBezierTo(0, 7, 16, 3)
      ..lineTo(16, 1)
      ..quadraticBezierTo(0, 5, -16, 1)
      ..close();
    canvas.drawPath(waterline, darkHull);

    // Portholes along the hull.
    for (final px in [-8.0, -2.0, 4.0, 10.0]) {
      canvas.drawCircle(Offset(px, -1.5), 1.3, rimColor);
    }

    // Deck rim — lighter strip along top edge.
    final rim = Path()
      ..moveTo(-14, -6)
      ..lineTo(14, -6)
      ..lineTo(14, -4)
      ..lineTo(-12, -4)
      ..close();
    canvas.drawPath(rim, rimColor);
    canvas.drawPath(rim, outline);

    // Forward gun turret — small circular base with barrel.
    canvas.drawCircle(const Offset(-8, -6), 3, darkHull);
    canvas.drawCircle(const Offset(-8, -6), 3, outline);
    canvas.drawLine(const Offset(-8, -6), const Offset(-14, -8), Paint()
      ..color = darkGray.color
      ..strokeWidth = 2.2);

    // Bridge / superstructure.
    final bridge = Path()
      ..moveTo(-4, -6)
      ..lineTo(-3, -14)
      ..lineTo(5, -14)
      ..lineTo(6, -6)
      ..close();
    canvas.drawPath(bridge, darkHull);
    canvas.drawPath(bridge, outline);

    // Bridge window stripe.
    final window = Path()
      ..moveTo(-2, -12)
      ..lineTo(4, -12)
      ..lineTo(4, -10)
      ..lineTo(-2, -10)
      ..close();
    canvas.drawPath(window, rimColor);

    // Mast — vertical pole with crossbar and crow's nest.
    canvas.drawLine(
      const Offset(1, -14), const Offset(1, -21), outline);
    canvas.drawLine(
      const Offset(-3, -18), const Offset(5, -18), outline);
    canvas.drawCircle(const Offset(1, -20), 1.5, metalGray);

    // Smokestack with smoke wisps.
    final stack = Path()
      ..moveTo(8, -6)
      ..lineTo(8.5, -12)
      ..lineTo(11.5, -12)
      ..lineTo(12, -6)
      ..close();
    canvas.drawPath(stack, metalGray);
    canvas.drawPath(stack, outline);
    // Smoke band on stack.
    canvas.drawLine(const Offset(8.8, -10), const Offset(11.2, -10), Paint()
      ..color = darkGray.color
      ..strokeWidth = 1.2);

    canvas.restore();
  }


  /// Draws a small 4-pointed star to indicate a newly built unit.
  void _drawNewUnitStar(Canvas canvas, Offset center) {
    const outerR = 7.0;
    const innerR = 3.0;
    final path = Path();
    for (var i = 0; i < 8; i++) {
      final angle = (i * math.pi / 4) - math.pi / 2;
      final r = i.isEven ? outerR : innerR;
      final x = center.dx + r * math.cos(angle);
      final y = center.dy + r * math.sin(angle);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    path.close();
    canvas.drawPath(path, Paint()..color = const Color(0xFFFFD700));
    canvas.drawPath(path, Paint()
      ..color = const Color(0xFF8B6914)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.2);
  }

  /// Draws an animated star for build animation: scales up from 0 and fades in.
  void _drawBuildStar(Canvas canvas, Offset center, double progress) {
    final scale = Curves.elasticOut.transform(math.min(progress * 1.5, 1.0));
    final opacity = Curves.easeOut.transform(math.min(progress * 2.0, 1.0));
    if (scale < 0.01 || opacity < 0.01) return;

    const outerR = 9.0;
    const innerR = 4.0;
    final path = Path();
    for (var i = 0; i < 8; i++) {
      final angle = (i * math.pi / 4) - math.pi / 2;
      final r = i.isEven ? outerR : innerR;
      final x = center.dx + r * scale * math.cos(angle);
      final y = center.dy + r * scale * math.sin(angle);
      if (i == 0) {
        path.moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
    }
    path.close();
    canvas.drawPath(path, Paint()..color = const Color(0xFFFFD700).withValues(alpha: opacity));
    canvas.drawPath(path, Paint()
      ..color = const Color(0xFF8B6914).withValues(alpha: opacity)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.4);
  }

  /// Draws a shield icon overlay for hold orders during animation.
  /// The shield pops up (scales from 0 to full size) with a slight fade-in.
  void _drawHoldShield(Canvas canvas, Offset center, double progress, Color powerColor) {
    final scale = Curves.elasticOut.transform(math.min(progress * 1.8, 1.0));
    final opacity = Curves.easeOut.transform(math.min(progress * 2.5, 1.0));
    if (scale < 0.01 || opacity < 0.01) return;

    canvas.save();
    canvas.translate(center.dx, center.dy - 16);
    canvas.scale(scale);

    // Shield shape: pointed bottom, flat top with rounded shoulders.
    final shield = Path()
      ..moveTo(0, 10)        // bottom point
      ..lineTo(-8, 2)        // lower-left
      ..lineTo(-8, -6)       // upper-left
      ..lineTo(-5, -9)       // top-left shoulder
      ..lineTo(5, -9)        // top-right shoulder
      ..lineTo(8, -6)        // upper-right
      ..lineTo(8, 2)         // lower-right
      ..close();

    // Dark outline behind for contrast.
    canvas.drawPath(shield, Paint()
      ..color = Colors.black.withValues(alpha: opacity * 0.6)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 3.0);

    // Filled shield in a muted version of the power color.
    final fillColor = Color.lerp(powerColor, Colors.white, 0.3)!;
    canvas.drawPath(shield, Paint()
      ..color = fillColor.withValues(alpha: opacity * 0.7)
      ..style = PaintingStyle.fill);

    // Shield border in darker power color.
    final borderColor = Color.lerp(powerColor, Colors.black, 0.2)!;
    canvas.drawPath(shield, Paint()
      ..color = borderColor.withValues(alpha: opacity)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5);

    // Small cross/plus on the shield face for detail.
    final detailPaint = Paint()
      ..color = Colors.white.withValues(alpha: opacity * 0.8)
      ..strokeWidth = 1.5
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    canvas.drawLine(const Offset(0, -5), const Offset(0, 3), detailPaint);
    canvas.drawLine(const Offset(-4, -1), const Offset(4, -1), detailPaint);

    canvas.restore();
  }

  /// Draws an animated X overlay for disband animation: fades in red X over the unit.
  void _drawDisbandX(Canvas canvas, Offset center, double progress) {
    final opacity = Curves.easeIn.transform(math.min(progress * 1.5, 1.0));
    if (opacity < 0.01) return;

    const size = 14.0;
    final paint = Paint()
      ..color = const Color(0xFFFF0000).withValues(alpha: opacity)
      ..strokeWidth = 4.0
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;
    final bgPaint = Paint()
      ..color = const Color(0xFF000000).withValues(alpha: opacity * 0.5)
      ..strokeWidth = 6.0
      ..strokeCap = StrokeCap.round
      ..style = PaintingStyle.stroke;

    // Draw black outline behind the X for contrast.
    canvas.drawLine(
      center + const Offset(-size, -size),
      center + const Offset(size, size),
      bgPaint,
    );
    canvas.drawLine(
      center + const Offset(size, -size),
      center + const Offset(-size, size),
      bgPaint,
    );
    // Draw the red X.
    canvas.drawLine(
      center + const Offset(-size, -size),
      center + const Offset(size, size),
      paint,
    );
    canvas.drawLine(
      center + const Offset(size, -size),
      center + const Offset(-size, size),
      paint,
    );
  }

  void _drawHighlight(Canvas canvas) {
    if (selectedProvince == null) return;
    final prov = provinces[selectedProvince];
    if (prov == null) return;

    final hasUnit = _occupiedProvinces.contains(selectedProvince);
    final center = hasUnit ? prov.center + const Offset(0, -8) : prov.center;

    canvas.drawCircle(
      center,
      24,
      Paint()
        ..color = Colors.yellow.withValues(alpha: 0.4)
        ..style = PaintingStyle.fill,
    );
    canvas.drawCircle(
      center,
      24,
      Paint()
        ..color = Colors.yellow.shade700
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3,
    );
  }

  /// Dims all provinces except valid targets and the selected province during move selection.
  void _drawDimOverlay(Canvas canvas) {
    if (validTargets.isEmpty) return;

    final fullRect = Path()
      ..addRect(Rect.fromLTWH(0, 0, svgViewBoxWidth, svgViewBoxHeight));

    var brightPath = Path();
    final brightProvinces = {...validTargets};
    if (selectedProvince != null) brightProvinces.add(selectedProvince!);

    for (final provId in brightProvinces) {
      final rings = provincePolygons[provId];
      if (rings == null || rings.isEmpty) continue;
      for (final polygon in rings) {
        if (polygon.length < 3) continue;
        final p = Path()..moveTo(polygon[0].dx, polygon[0].dy);
        for (var i = 1; i < polygon.length; i++) {
          p.lineTo(polygon[i].dx, polygon[i].dy);
        }
        p.close();
        brightPath = Path.combine(PathOperation.union, brightPath, p);
      }
    }

    final dimPath = Path.combine(PathOperation.difference, fullRect, brightPath);
    canvas.drawPath(dimPath, Paint()..color = const Color(0x88000000));
  }

  void _drawValidTargets(Canvas canvas) {
    for (final id in validTargets) {
      final prov = provinces[id];
      if (prov == null) continue;

      canvas.drawCircle(
        prov.center,
        22,
        Paint()
          ..color = Colors.green.withValues(alpha: 0.3)
          ..style = PaintingStyle.fill,
      );
      canvas.drawCircle(
        prov.center,
        22,
        Paint()
          ..color = Colors.green
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2.5,
      );
    }
  }

  void _drawPendingOrders(Canvas canvas) {
    for (final order in pendingOrders) {
      final from = provinces[order.location]?.center;
      if (from == null) continue;

      if (order.orderType == 'support' && order.auxLoc != null && order.auxTarget != null) {
        _drawSupportArrow(canvas, from, order.auxLoc!, order.auxTarget!, _orderColor('support'));
      } else if (order.target != null) {
        final to = provinces[order.target]?.center;
        if (to == null) continue;
        _drawArrow(canvas, from, to, _orderColor(order.orderType), _orderDash(order.orderType));
      }
    }
  }

  void _drawResolvedOrders(Canvas canvas) {
    for (final order in resolvedOrders!) {
      final from = provinces[order.location]?.center;
      if (from == null) continue;

      final succeeded = order.result == 'succeeds';

      if (order.orderType == 'hold') {
        // Draw a fully visible shield for hold orders (no animation when static).
        final color = PowerColors.forPower(order.power);
        final unitType = order.unitType == 'fleet' ? UnitType.fleet : UnitType.army;
        final unitCenter = _unitCenterAt(order.location, unitType);
        _drawHoldShield(canvas, unitCenter, 1.0, color);
        continue;
      }

      if (order.orderType == 'support' && order.auxLoc != null && order.auxTarget != null) {
        // Support: always yellow dotted
        _drawSupportArrow(canvas, from, order.auxLoc!, order.auxTarget!, Colors.yellow.shade700);
      } else if (order.target != null && order.target!.isNotEmpty) {
        final to = provinces[order.target]?.center;
        if (to == null) continue;

        if (order.orderType == 'retreat_move') {
          _drawArrow(canvas, from, to, Colors.blue.shade600, !succeeded);
        } else if (order.orderType == 'move' && !succeeded) {
          _drawArrow(canvas, from, to, Colors.pink.shade300, true);
        } else if (order.orderType == 'move' && succeeded) {
          _drawArrow(canvas, from, to, Colors.green.shade700, false);
        } else {
          final color = succeeded ? Colors.grey.shade500 : Colors.grey.shade400;
          _drawArrow(canvas, from, to, color, !succeeded);
        }
      }
    }
  }

  Color _orderColor(String type) {
    return switch (type) {
      'move' => Colors.green.shade700,
      'support' => Colors.yellow.shade700,
      'convoy' => Colors.purple.shade700,
      'retreat_move' => Colors.blue.shade600,
      _ => Colors.grey,
    };
  }

  bool _orderDash(String type) {
    return type == 'support' || type == 'convoy';
  }

  /// Draws a curved Bezier arrow between two provinces.
  /// Arrow tip size and curvature scale with arrow length so short arrows
  /// remain legible and don't have oversized heads or excessive bending.
  void _drawArrow(Canvas canvas, Offset from, Offset to, Color color, bool dashed) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 4.5
      ..style = PaintingStyle.stroke;

    final dir = to - from;
    final len = dir.distance;
    if (len < 20) return;
    final unitDir = dir / len;

    // Scale the inset from unit circles based on arrow length so short
    // arrows don't collapse to nothing.
    final inset = math.min(18.0, len * 0.2);
    final start = from + unitDir * inset;
    final end = to - unitDir * inset;

    // Curvature scales with length but tapers off for short arrows to
    // keep them readable. Long arrows curve at 18%; short ones less.
    final curveFactor = len < 80 ? 0.06 + 0.12 * ((len - 20) / 60).clamp(0.0, 1.0) : 0.18;
    final perp = Offset(-unitDir.dy, unitDir.dx);
    final mid = Offset((start.dx + end.dx) / 2, (start.dy + end.dy) / 2);
    final cp = mid + perp * (len * curveFactor);

    if (dashed) {
      _drawDashedBezier(canvas, start, cp, end, paint);
    } else {
      final path = Path()
        ..moveTo(start.dx, start.dy)
        ..quadraticBezierTo(cp.dx, cp.dy, end.dx, end.dy);
      canvas.drawPath(path, paint);
    }

    // Arrowhead scales with arrow length: 19px at full size, down to 10px minimum.
    final tangent = end - cp;
    final tangentLen = tangent.distance;
    if (tangentLen < 1) return;
    final angle = math.atan2(tangent.dy, tangent.dx);
    final arrowSize = (len * 0.12).clamp(10.0, 19.0);
    const arrowSpread = 2.5;
    final p1 = end + Offset(
      arrowSize * math.cos(angle + arrowSpread),
      arrowSize * math.sin(angle + arrowSpread),
    );
    final p2 = end + Offset(
      arrowSize * math.cos(angle - arrowSpread),
      arrowSize * math.sin(angle - arrowSpread),
    );
    final arrowPath = Path()
      ..moveTo(end.dx, end.dy)
      ..lineTo(p1.dx, p1.dy)
      ..lineTo(p2.dx, p2.dy)
      ..close();
    canvas.drawPath(arrowPath, Paint()..color = color);
  }

  /// Draws a dashed Bezier from the supporter to the midpoint of the supported move,
  /// with a filled circle at the attachment point.
  void _drawSupportArrow(Canvas canvas, Offset from, String auxLoc, String auxTarget, Color color) {
    final auxFrom = provinces[auxLoc]?.center;
    final auxTo = provinces[auxTarget]?.center;
    if (auxFrom == null || auxTo == null) return;

    // Target the midpoint of the supported move's Bezier curve, not the
    // straight line. For a quadratic Bezier with control point offset by
    // perp * (len * 0.18), the curve at t=0.5 is the straight midpoint
    // shifted by half the curvature: perp * (len * 0.09).
    final moveDir = auxTo - auxFrom;
    final moveLen = moveDir.distance;
    final straightMid = Offset(
      (auxFrom.dx + auxTo.dx) / 2,
      (auxFrom.dy + auxTo.dy) / 2,
    );
    Offset midpoint;
    if (moveLen > 1) {
      final moveUnit = moveDir / moveLen;
      final movePerp = Offset(-moveUnit.dy, moveUnit.dx);
      midpoint = straightMid + movePerp * (moveLen * 0.09);
    } else {
      midpoint = straightMid;
    }

    final paint = Paint()
      ..color = color
      ..strokeWidth = 4.5
      ..style = PaintingStyle.stroke;

    final dir = midpoint - from;
    final len = dir.distance;
    if (len < 30) return;
    final unitDir = dir / len;
    final start = from + unitDir * 18;

    final perp = Offset(-unitDir.dy, unitDir.dx);
    final mid = Offset((start.dx + midpoint.dx) / 2, (start.dy + midpoint.dy) / 2);
    final cp = mid + perp * (len * 0.18);

    _drawDashedBezier(canvas, start, cp, midpoint, paint);

    // Filled circle at midpoint.
    canvas.drawCircle(midpoint, 5, Paint()..color = color);
    canvas.drawCircle(midpoint, 5, Paint()
      ..color = Color.lerp(color, Colors.black, 0.35)!
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5);
  }

  /// Draws a dashed quadratic Bezier curve using path metrics for even dash spacing.
  void _drawDashedBezier(Canvas canvas, Offset start, Offset cp, Offset end, Paint paint) {
    final path = Path()
      ..moveTo(start.dx, start.dy)
      ..quadraticBezierTo(cp.dx, cp.dy, end.dx, end.dy);
    const dashLen = 11.0;
    const gapLen = 8.0;
    for (final metric in path.computeMetrics()) {
      var d = 0.0;
      while (d < metric.length) {
        final dashEnd = math.min(d + dashLen, metric.length);
        canvas.drawPath(metric.extractPath(d, dashEnd), paint);
        d += dashLen + gapLen;
      }
    }
  }

  @override
  bool shouldRepaint(covariant MapPainter oldDelegate) {
    return oldDelegate.gameState != gameState ||
        oldDelegate.previousGameState != previousGameState ||
        oldDelegate.animationProgress != animationProgress ||
        oldDelegate.selectedProvince != selectedProvince ||
        oldDelegate.validTargets != validTargets ||
        oldDelegate.pendingOrders != pendingOrders ||
        oldDelegate.resolvedOrders != resolvedOrders ||
        oldDelegate.myPower != myPower ||
        oldDelegate.newUnitProvinces != newUnitProvinces ||
        oldDelegate.buildProvinces != buildProvinces ||
        oldDelegate.disbandProvinces != disbandProvinces ||
        oldDelegate.disbandUnits != disbandUnits ||
        oldDelegate.buildDisbandProgress != buildDisbandProgress;
  }
}

/// A pending order for drawing on the map.
class PendingOrder {
  final String location;
  final String orderType;
  final String? target;
  final String? auxLoc;
  final String? auxTarget;

  const PendingOrder({
    required this.location,
    required this.orderType,
    this.target,
    this.auxLoc,
    this.auxTarget,
  });
}
