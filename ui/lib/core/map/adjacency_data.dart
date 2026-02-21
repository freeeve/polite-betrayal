// Dart port of the adjacency graph from map_data.go.
// Enables highlighting valid targets client-side.

/// Adjacency types.
enum AdjType { army, fleet, both }

class Adjacency {
  final String from;
  final String to;
  final AdjType type;
  final String fromCoast; // for split-coast fleet adjacencies
  final String toCoast;

  const Adjacency({
    required this.from,
    required this.to,
    required this.type,
    this.fromCoast = '',
    this.toCoast = '',
  });
}

/// Returns valid move targets for an army at the given province.
Set<String> armyTargets(String province) {
  final targets = <String>{};
  for (final adj in _adjacencies) {
    if (adj.type == AdjType.fleet) continue; // skip fleet-only
    if (adj.from == province) targets.add(adj.to);
    if (adj.to == province) targets.add(adj.from);
  }
  return targets;
}

/// Returns valid move targets for a fleet at the given province (optionally on a specific coast).
Set<String> fleetTargets(String province, {String coast = ''}) {
  final targets = <String>{};
  for (final adj in _adjacencies) {
    if (adj.type == AdjType.army) continue; // skip army-only
    if (adj.from == province && (coast.isEmpty || adj.fromCoast.isEmpty || adj.fromCoast == coast)) {
      targets.add(adj.to);
    }
    if (adj.to == province && (coast.isEmpty || adj.toCoast.isEmpty || adj.toCoast == coast)) {
      targets.add(adj.from);
    }
  }
  return targets;
}

/// Returns all adjacent provinces regardless of unit type.
Set<String> allAdjacent(String province) {
  final targets = <String>{};
  for (final adj in _adjacencies) {
    if (adj.from == province) targets.add(adj.to);
    if (adj.to == province) targets.add(adj.from);
  }
  return targets;
}

/// Returns all coast options for a split-coast province.
List<String> coastOptions(String target) {
  return switch (target) {
    'bul' => ['ec', 'sc'],
    'spa' => ['nc', 'sc'],
    'stp' => ['nc', 'sc'],
    _ => [],
  };
}

/// Returns only the coasts of [target] reachable by a fleet at [from] (with optional [fromCoast]).
List<String> reachableCoasts(String from, String fromCoast, String target) {
  final coasts = <String>{};
  for (final adj in _adjacencies) {
    if (adj.type == AdjType.army) continue;
    if (adj.from == target && adj.to == from &&
        (fromCoast.isEmpty || adj.toCoast.isEmpty || adj.toCoast == fromCoast)) {
      if (adj.fromCoast.isNotEmpty) coasts.add(adj.fromCoast);
    }
    if (adj.from == from && adj.to == target &&
        (fromCoast.isEmpty || adj.fromCoast.isEmpty || adj.fromCoast == fromCoast)) {
      if (adj.toCoast.isNotEmpty) coasts.add(adj.toCoast);
    }
  }
  return coasts.toList();
}

/// Check if two provinces are adjacent for a given unit type.
bool isAdjacent(String from, String to, {required bool isFleet, String coast = ''}) {
  if (isFleet) {
    return fleetTargets(from, coast: coast).contains(to);
  }
  return armyTargets(from).contains(to);
}

// All adjacencies, mirroring map_data.go.
// Each pair is listed once (bidirectional lookup done in accessor functions).
const _adjacencies = <Adjacency>[
  // === INLAND (army-only) adjacencies ===
  // Bohemia
  Adjacency(from: 'boh', to: 'gal', type: AdjType.army),
  Adjacency(from: 'boh', to: 'mun', type: AdjType.army),
  Adjacency(from: 'boh', to: 'sil', type: AdjType.army),
  Adjacency(from: 'boh', to: 'tyr', type: AdjType.army),
  Adjacency(from: 'boh', to: 'vie', type: AdjType.army),
  // Budapest
  Adjacency(from: 'bud', to: 'gal', type: AdjType.army),
  Adjacency(from: 'bud', to: 'rum', type: AdjType.army),
  Adjacency(from: 'bud', to: 'ser', type: AdjType.army),
  Adjacency(from: 'bud', to: 'tri', type: AdjType.army),
  Adjacency(from: 'bud', to: 'vie', type: AdjType.army),
  // Burgundy
  Adjacency(from: 'bur', to: 'gas', type: AdjType.army),
  Adjacency(from: 'bur', to: 'mar', type: AdjType.army),
  Adjacency(from: 'bur', to: 'mun', type: AdjType.army),
  Adjacency(from: 'bur', to: 'par', type: AdjType.army),
  Adjacency(from: 'bur', to: 'pic', type: AdjType.army),
  Adjacency(from: 'bur', to: 'ruh', type: AdjType.army),
  Adjacency(from: 'bur', to: 'bel', type: AdjType.army),
  // Galicia
  Adjacency(from: 'gal', to: 'rum', type: AdjType.army),
  Adjacency(from: 'gal', to: 'sil', type: AdjType.army),
  Adjacency(from: 'gal', to: 'ukr', type: AdjType.army),
  Adjacency(from: 'gal', to: 'vie', type: AdjType.army),
  Adjacency(from: 'gal', to: 'war', type: AdjType.army),
  // Moscow
  Adjacency(from: 'mos', to: 'lvn', type: AdjType.army),
  Adjacency(from: 'mos', to: 'sev', type: AdjType.army),
  Adjacency(from: 'mos', to: 'stp', type: AdjType.army),
  Adjacency(from: 'mos', to: 'ukr', type: AdjType.army),
  Adjacency(from: 'mos', to: 'war', type: AdjType.army),
  // Munich
  Adjacency(from: 'mun', to: 'kie', type: AdjType.army),
  Adjacency(from: 'mun', to: 'sil', type: AdjType.army),
  Adjacency(from: 'mun', to: 'tyr', type: AdjType.army),
  Adjacency(from: 'mun', to: 'ber', type: AdjType.army),
  // Paris
  Adjacency(from: 'par', to: 'bre', type: AdjType.army),
  Adjacency(from: 'par', to: 'gas', type: AdjType.army),
  Adjacency(from: 'par', to: 'pic', type: AdjType.army),
  // Ruhr
  Adjacency(from: 'ruh', to: 'bel', type: AdjType.army),
  Adjacency(from: 'ruh', to: 'hol', type: AdjType.army),
  Adjacency(from: 'ruh', to: 'kie', type: AdjType.army),
  Adjacency(from: 'ruh', to: 'mun', type: AdjType.army),
  // Serbia
  Adjacency(from: 'ser', to: 'alb', type: AdjType.army),
  Adjacency(from: 'ser', to: 'bul', type: AdjType.army),
  Adjacency(from: 'ser', to: 'gre', type: AdjType.army),
  Adjacency(from: 'ser', to: 'rum', type: AdjType.army),
  Adjacency(from: 'ser', to: 'tri', type: AdjType.army),
  // Silesia
  Adjacency(from: 'sil', to: 'ber', type: AdjType.army),
  Adjacency(from: 'sil', to: 'pru', type: AdjType.army),
  Adjacency(from: 'sil', to: 'war', type: AdjType.army),
  // Tyrolia
  Adjacency(from: 'tyr', to: 'pie', type: AdjType.army),
  Adjacency(from: 'tyr', to: 'tri', type: AdjType.army),
  Adjacency(from: 'tyr', to: 'ven', type: AdjType.army),
  Adjacency(from: 'tyr', to: 'vie', type: AdjType.army),
  // Ukraine
  Adjacency(from: 'ukr', to: 'rum', type: AdjType.army),
  Adjacency(from: 'ukr', to: 'sev', type: AdjType.army),
  Adjacency(from: 'ukr', to: 'war', type: AdjType.army),
  // Vienna
  Adjacency(from: 'vie', to: 'tri', type: AdjType.army),
  // Warsaw
  Adjacency(from: 'war', to: 'lvn', type: AdjType.army),
  Adjacency(from: 'war', to: 'pru', type: AdjType.army),

  // === COASTAL (both army+fleet) adjacencies ===
  Adjacency(from: 'alb', to: 'gre', type: AdjType.both),
  Adjacency(from: 'alb', to: 'tri', type: AdjType.both),
  Adjacency(from: 'ank', to: 'arm', type: AdjType.both),
  Adjacency(from: 'ank', to: 'con', type: AdjType.both),
  Adjacency(from: 'apu', to: 'nap', type: AdjType.both),
  Adjacency(from: 'apu', to: 'ven', type: AdjType.both),
  Adjacency(from: 'arm', to: 'sev', type: AdjType.both),
  Adjacency(from: 'bel', to: 'hol', type: AdjType.both),
  Adjacency(from: 'bel', to: 'pic', type: AdjType.both),
  Adjacency(from: 'ber', to: 'kie', type: AdjType.both),
  Adjacency(from: 'ber', to: 'pru', type: AdjType.both),
  Adjacency(from: 'bre', to: 'gas', type: AdjType.both),
  Adjacency(from: 'bre', to: 'pic', type: AdjType.both),
  Adjacency(from: 'cly', to: 'edi', type: AdjType.both),
  Adjacency(from: 'cly', to: 'lvp', type: AdjType.both),
  Adjacency(from: 'con', to: 'smy', type: AdjType.both),
  Adjacency(from: 'den', to: 'kie', type: AdjType.both),
  Adjacency(from: 'den', to: 'swe', type: AdjType.both),
  Adjacency(from: 'hol', to: 'kie', type: AdjType.both),
  Adjacency(from: 'edi', to: 'yor', type: AdjType.both),
  Adjacency(from: 'fin', to: 'swe', type: AdjType.both),
  Adjacency(from: 'gas', to: 'mar', type: AdjType.army),
  Adjacency(from: 'lon', to: 'wal', type: AdjType.both),
  Adjacency(from: 'lon', to: 'yor', type: AdjType.both),
  Adjacency(from: 'lvn', to: 'pru', type: AdjType.both),
  Adjacency(from: 'lvp', to: 'wal', type: AdjType.both),
  Adjacency(from: 'mar', to: 'pie', type: AdjType.both),
  Adjacency(from: 'naf', to: 'tun', type: AdjType.both),
  Adjacency(from: 'nap', to: 'rom', type: AdjType.both),
  Adjacency(from: 'nwy', to: 'swe', type: AdjType.both),
  Adjacency(from: 'pie', to: 'tus', type: AdjType.both),
  Adjacency(from: 'rom', to: 'tus', type: AdjType.both),
  Adjacency(from: 'rum', to: 'sev', type: AdjType.both),
  Adjacency(from: 'smy', to: 'syr', type: AdjType.both),
  Adjacency(from: 'tri', to: 'ven', type: AdjType.both),

  // === ARMY-ONLY: coastal-to-inland or coastal-to-split-coast ===
  Adjacency(from: 'con', to: 'bul', type: AdjType.army),
  Adjacency(from: 'fin', to: 'stp', type: AdjType.army),
  Adjacency(from: 'gas', to: 'spa', type: AdjType.army),
  Adjacency(from: 'gre', to: 'bul', type: AdjType.army),
  Adjacency(from: 'lvn', to: 'stp', type: AdjType.army),
  Adjacency(from: 'mar', to: 'spa', type: AdjType.army),
  Adjacency(from: 'nwy', to: 'stp', type: AdjType.army),
  Adjacency(from: 'por', to: 'spa', type: AdjType.army),
  Adjacency(from: 'rum', to: 'bul', type: AdjType.army),
  // Coastal-to-coastal army-only: land border but different seas
  Adjacency(from: 'ank', to: 'smy', type: AdjType.army),
  Adjacency(from: 'apu', to: 'rom', type: AdjType.army),
  Adjacency(from: 'arm', to: 'smy', type: AdjType.army),
  Adjacency(from: 'arm', to: 'syr', type: AdjType.army),
  Adjacency(from: 'edi', to: 'lvp', type: AdjType.army),
  Adjacency(from: 'fin', to: 'nwy', type: AdjType.army),
  Adjacency(from: 'lvp', to: 'yor', type: AdjType.army),
  Adjacency(from: 'pie', to: 'ven', type: AdjType.army),
  Adjacency(from: 'rom', to: 'ven', type: AdjType.army),
  Adjacency(from: 'tus', to: 'ven', type: AdjType.army),
  Adjacency(from: 'wal', to: 'yor', type: AdjType.army),

  // === FLEET-ONLY: coastal to sea zone adjacencies ===
  Adjacency(from: 'alb', to: 'adr', type: AdjType.fleet),
  Adjacency(from: 'alb', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'ank', to: 'bla', type: AdjType.fleet),
  Adjacency(from: 'apu', to: 'adr', type: AdjType.fleet),
  Adjacency(from: 'apu', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'arm', to: 'bla', type: AdjType.fleet),
  Adjacency(from: 'bel', to: 'eng', type: AdjType.fleet),
  Adjacency(from: 'bel', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'ber', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'bre', to: 'eng', type: AdjType.fleet),
  Adjacency(from: 'bre', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'cly', to: 'nao', type: AdjType.fleet),
  Adjacency(from: 'cly', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'con', to: 'aeg', type: AdjType.fleet),
  Adjacency(from: 'con', to: 'bla', type: AdjType.fleet),
  Adjacency(from: 'den', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'den', to: 'hel', type: AdjType.fleet),
  Adjacency(from: 'den', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'den', to: 'ska', type: AdjType.fleet),
  Adjacency(from: 'edi', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'edi', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'fin', to: 'bot', type: AdjType.fleet),
  Adjacency(from: 'gre', to: 'aeg', type: AdjType.fleet),
  Adjacency(from: 'gre', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'hol', to: 'hel', type: AdjType.fleet),
  Adjacency(from: 'hol', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'kie', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'kie', to: 'hel', type: AdjType.fleet),
  Adjacency(from: 'lon', to: 'eng', type: AdjType.fleet),
  Adjacency(from: 'lon', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'lvn', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'lvn', to: 'bot', type: AdjType.fleet),
  Adjacency(from: 'lvp', to: 'iri', type: AdjType.fleet),
  Adjacency(from: 'lvp', to: 'nao', type: AdjType.fleet),
  Adjacency(from: 'naf', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'naf', to: 'wes', type: AdjType.fleet),
  Adjacency(from: 'nap', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'nap', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'nwy', to: 'bar', type: AdjType.fleet),
  Adjacency(from: 'nwy', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'nwy', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'nwy', to: 'ska', type: AdjType.fleet),
  Adjacency(from: 'pic', to: 'eng', type: AdjType.fleet),
  Adjacency(from: 'pie', to: 'gol', type: AdjType.fleet),
  Adjacency(from: 'pru', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'rom', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'rum', to: 'bla', type: AdjType.fleet),
  Adjacency(from: 'sev', to: 'bla', type: AdjType.fleet),
  Adjacency(from: 'smy', to: 'aeg', type: AdjType.fleet),
  Adjacency(from: 'smy', to: 'eas', type: AdjType.fleet),
  Adjacency(from: 'swe', to: 'bal', type: AdjType.fleet),
  Adjacency(from: 'swe', to: 'bot', type: AdjType.fleet),
  Adjacency(from: 'swe', to: 'ska', type: AdjType.fleet),
  Adjacency(from: 'syr', to: 'eas', type: AdjType.fleet),
  Adjacency(from: 'tri', to: 'adr', type: AdjType.fleet),
  Adjacency(from: 'tun', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'tun', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'tun', to: 'wes', type: AdjType.fleet),
  Adjacency(from: 'tus', to: 'gol', type: AdjType.fleet),
  Adjacency(from: 'tus', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'ven', to: 'adr', type: AdjType.fleet),
  Adjacency(from: 'wal', to: 'eng', type: AdjType.fleet),
  Adjacency(from: 'wal', to: 'iri', type: AdjType.fleet),
  Adjacency(from: 'yor', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'por', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'gas', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'mar', to: 'gol', type: AdjType.fleet),

  // === SEA-TO-SEA adjacencies (fleet only) ===
  Adjacency(from: 'adr', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'aeg', to: 'eas', type: AdjType.fleet),
  Adjacency(from: 'aeg', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'bal', to: 'bot', type: AdjType.fleet),
  Adjacency(from: 'bar', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'eas', to: 'ion', type: AdjType.fleet),
  Adjacency(from: 'eng', to: 'iri', type: AdjType.fleet),
  Adjacency(from: 'eng', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'eng', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'gol', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'gol', to: 'wes', type: AdjType.fleet),
  Adjacency(from: 'hel', to: 'nth', type: AdjType.fleet),
  Adjacency(from: 'ion', to: 'tys', type: AdjType.fleet),
  Adjacency(from: 'iri', to: 'mao', type: AdjType.fleet),
  Adjacency(from: 'iri', to: 'nao', type: AdjType.fleet),
  Adjacency(from: 'mao', to: 'nao', type: AdjType.fleet),
  Adjacency(from: 'mao', to: 'wes', type: AdjType.fleet),
  Adjacency(from: 'nao', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'nth', to: 'nrg', type: AdjType.fleet),
  Adjacency(from: 'nth', to: 'ska', type: AdjType.fleet),
  Adjacency(from: 'tys', to: 'wes', type: AdjType.fleet),

  // === SPLIT-COAST fleet adjacencies ===
  // Bulgaria East Coast
  Adjacency(from: 'bul', to: 'bla', type: AdjType.fleet, fromCoast: 'ec'),
  Adjacency(from: 'bul', to: 'con', type: AdjType.fleet, fromCoast: 'ec'),
  Adjacency(from: 'bul', to: 'rum', type: AdjType.fleet, fromCoast: 'ec'),
  // Bulgaria South Coast
  Adjacency(from: 'bul', to: 'aeg', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'bul', to: 'con', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'bul', to: 'gre', type: AdjType.fleet, fromCoast: 'sc'),
  // Spain North Coast
  Adjacency(from: 'spa', to: 'mao', type: AdjType.fleet, fromCoast: 'nc'),
  Adjacency(from: 'spa', to: 'gas', type: AdjType.fleet, fromCoast: 'nc'),
  Adjacency(from: 'spa', to: 'por', type: AdjType.fleet, fromCoast: 'nc'),
  // Spain South Coast
  Adjacency(from: 'spa', to: 'mao', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'spa', to: 'mar', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'spa', to: 'gol', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'spa', to: 'wes', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'spa', to: 'por', type: AdjType.fleet, fromCoast: 'sc'),
  // St. Petersburg North Coast
  Adjacency(from: 'stp', to: 'bar', type: AdjType.fleet, fromCoast: 'nc'),
  Adjacency(from: 'stp', to: 'nwy', type: AdjType.fleet, fromCoast: 'nc'),
  // St. Petersburg South Coast
  Adjacency(from: 'stp', to: 'bot', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'stp', to: 'fin', type: AdjType.fleet, fromCoast: 'sc'),
  Adjacency(from: 'stp', to: 'lvn', type: AdjType.fleet, fromCoast: 'sc'),
];
