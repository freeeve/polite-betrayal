import 'dart:ui';

/// SVG viewBox dimensions — province centers are calibrated to this space.
const double svgViewBoxWidth = 1152.0;
const double svgViewBoxHeight = 1152.0;

enum ProvinceType { land, coastal, sea }

class Province {
  final String id;
  final String name;
  final ProvinceType type;
  final bool isSupplyCenter;
  final Offset center;
  final List<String> coasts; // non-empty for split-coast provinces

  const Province({
    required this.id,
    required this.name,
    required this.type,
    this.isSupplyCenter = false,
    required this.center,
    this.coasts = const [],
  });

  bool get isSplitCoast => coasts.isNotEmpty;
  bool get isSea => type == ProvinceType.sea;
  bool get isLand => type != ProvinceType.sea;
}

/// All 75 provinces with center coordinates calibrated to the SVG viewBox (1152x1152).
Map<String, Province> get provinces => {
  // === BRITISH ISLES ===
  'cly': const Province(id: 'cly', name: 'Clyde', type: ProvinceType.coastal, center: Offset(262, 435)),
  'edi': const Province(id: 'edi', name: 'Edinburgh', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(294, 445)),
  'lvp': const Province(id: 'lvp', name: 'Liverpool', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(274, 500)),
  'yor': const Province(id: 'yor', name: 'Yorkshire', type: ProvinceType.coastal, center: Offset(300, 535)),
  'wal': const Province(id: 'wal', name: 'Wales', type: ProvinceType.coastal, center: Offset(262, 576)),
  'lon': const Province(id: 'lon', name: 'London', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(305, 590)),

  // === SCANDINAVIA ===
  'nwy': const Province(id: 'nwy', name: 'Norway', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(500, 380)),
  'swe': const Province(id: 'swe', name: 'Sweden', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(566, 400)),
  'den': const Province(id: 'den', name: 'Denmark', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(475, 505)),
  'fin': const Province(id: 'fin', name: 'Finland', type: ProvinceType.coastal, center: Offset(704, 340)),
  'stp': const Province(id: 'stp', name: 'St. Petersburg', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(819, 390), coasts: ['nc', 'sc']),

  // === FRANCE ===
  'bre': const Province(id: 'bre', name: 'Brest', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(210, 660)),
  'pic': const Province(id: 'pic', name: 'Picardy', type: ProvinceType.coastal, center: Offset(314, 655)),
  'par': const Province(id: 'par', name: 'Paris', type: ProvinceType.land, isSupplyCenter: true, center: Offset(315, 704)),
  'bur': const Province(id: 'bur', name: 'Burgundy', type: ProvinceType.land, center: Offset(350, 740)),
  'gas': const Province(id: 'gas', name: 'Gascony', type: ProvinceType.coastal, center: Offset(252, 795)),
  'mar': const Province(id: 'mar', name: 'Marseilles', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(345, 811)),

  // === IBERIA ===
  'spa': const Province(id: 'spa', name: 'Spain', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(154, 872), coasts: ['nc', 'sc']),
  'por': const Province(id: 'por', name: 'Portugal', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(54, 870)),

  // === LOW COUNTRIES ===
  'bel': const Province(id: 'bel', name: 'Belgium', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(348, 637)),
  'hol': const Province(id: 'hol', name: 'Holland', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(392, 603)),

  // === GERMANY ===
  'ruh': const Province(id: 'ruh', name: 'Ruhr', type: ProvinceType.land, center: Offset(417, 665)),
  'kie': const Province(id: 'kie', name: 'Kiel', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(470, 605)),
  'ber': const Province(id: 'ber', name: 'Berlin', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(525, 605)),
  'mun': const Province(id: 'mun', name: 'Munich', type: ProvinceType.land, isSupplyCenter: true, center: Offset(470, 715)),
  'sil': const Province(id: 'sil', name: 'Silesia', type: ProvinceType.land, center: Offset(560, 640)),
  'pru': const Province(id: 'pru', name: 'Prussia', type: ProvinceType.coastal, center: Offset(593, 590)),

  // === AUSTRIA-HUNGARY ===
  'tyr': const Province(id: 'tyr', name: 'Tyrolia', type: ProvinceType.land, center: Offset(500, 765)),
  'boh': const Province(id: 'boh', name: 'Bohemia', type: ProvinceType.land, center: Offset(530, 690)),
  'vie': const Province(id: 'vie', name: 'Vienna', type: ProvinceType.land, isSupplyCenter: true, center: Offset(584, 733)),
  'bud': const Province(id: 'bud', name: 'Budapest', type: ProvinceType.land, isSupplyCenter: true, center: Offset(655, 758)),
  'tri': const Province(id: 'tri', name: 'Trieste', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(574, 800)),
  'gal': const Province(id: 'gal', name: 'Galicia', type: ProvinceType.land, center: Offset(700, 694)),

  // === ITALY ===
  'pie': const Province(id: 'pie', name: 'Piedmont', type: ProvinceType.coastal, center: Offset(415, 810)),
  'ven': const Province(id: 'ven', name: 'Venice', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(475, 814)),
  'tus': const Province(id: 'tus', name: 'Tuscany', type: ProvinceType.coastal, center: Offset(467, 855)),
  'rom': const Province(id: 'rom', name: 'Rome', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(497, 892)),
  'apu': const Province(id: 'apu', name: 'Apulia', type: ProvinceType.coastal, isSupplyCenter: false, center: Offset(541, 907)),
  'nap': const Province(id: 'nap', name: 'Naples', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(544, 956)),

  // === BALKANS ===
  'ser': const Province(id: 'ser', name: 'Serbia', type: ProvinceType.land, isSupplyCenter: true, center: Offset(650, 859)),
  'alb': const Province(id: 'alb', name: 'Albania', type: ProvinceType.coastal, center: Offset(640, 905)),
  'gre': const Province(id: 'gre', name: 'Greece', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(680, 985)),
  'bul': const Province(id: 'bul', name: 'Bulgaria', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(730, 885), coasts: ['ec', 'sc']),
  'rum': const Province(id: 'rum', name: 'Rumania', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(780, 815)),

  // === TURKEY ===
  'con': const Province(id: 'con', name: 'Constantinople', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(805, 912)),
  'ank': const Province(id: 'ank', name: 'Ankara', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(920, 920)),
  'smy': const Province(id: 'smy', name: 'Smyrna', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(830, 1015)),
  'arm': const Province(id: 'arm', name: 'Armenia', type: ProvinceType.coastal, center: Offset(1094, 905)),
  'syr': const Province(id: 'syr', name: 'Syria', type: ProvinceType.coastal, center: Offset(1054, 1015)),

  // === RUSSIA ===
  'mos': const Province(id: 'mos', name: 'Moscow', type: ProvinceType.land, isSupplyCenter: true, center: Offset(920, 490)),
  'war': const Province(id: 'war', name: 'Warsaw', type: ProvinceType.land, isSupplyCenter: true, center: Offset(655, 625)),
  'ukr': const Province(id: 'ukr', name: 'Ukraine', type: ProvinceType.land, center: Offset(785, 693)),
  'sev': const Province(id: 'sev', name: 'Sevastopol', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(912, 785)),
  'lvn': const Province(id: 'lvn', name: 'Livonia', type: ProvinceType.coastal, center: Offset(714, 524)),

  // === NORTH AFRICA ===
  'naf': const Province(id: 'naf', name: 'North Africa', type: ProvinceType.coastal, center: Offset(198, 1051)),
  'tun': const Province(id: 'tun', name: 'Tunisia', type: ProvinceType.coastal, isSupplyCenter: true, center: Offset(410, 1058)),

  // === SEA ZONES ===
  'nao': const Province(id: 'nao', name: 'North Atlantic Ocean', type: ProvinceType.sea, center: Offset(82, 365)),
  'mao': const Province(id: 'mao', name: 'Mid-Atlantic Ocean', type: ProvinceType.sea, center: Offset(44, 730)),
  'iri': const Province(id: 'iri', name: 'Irish Sea', type: ProvinceType.sea, center: Offset(160, 560)),
  'eng': const Province(id: 'eng', name: 'English Channel', type: ProvinceType.sea, center: Offset(240, 625)),
  'nth': const Province(id: 'nth', name: 'North Sea', type: ProvinceType.sea, center: Offset(358, 498)),
  'nrg': const Province(id: 'nrg', name: 'Norwegian Sea', type: ProvinceType.sea, center: Offset(350, 260)),
  'bar': const Province(id: 'bar', name: 'Barents Sea', type: ProvinceType.sea, center: Offset(804, 120)),
  'ska': const Province(id: 'ska', name: 'Skagerrak', type: ProvinceType.sea, center: Offset(493, 458)),
  'hel': const Province(id: 'hel', name: 'Heligoland Bight', type: ProvinceType.sea, center: Offset(434, 527)),
  'bal': const Province(id: 'bal', name: 'Baltic Sea', type: ProvinceType.sea, center: Offset(574, 537)),
  'bot': const Province(id: 'bot', name: 'Gulf of Bothnia', type: ProvinceType.sea, center: Offset(633, 383)),
  'gol': const Province(id: 'gol', name: 'Gulf of Lyon', type: ProvinceType.sea, center: Offset(318, 870)),
  'wes': const Province(id: 'wes', name: 'Western Mediterranean', type: ProvinceType.sea, center: Offset(230, 970)),
  'tys': const Province(id: 'tys', name: 'Tyrrhenian Sea', type: ProvinceType.sea, center: Offset(450, 960)),
  'ion': const Province(id: 'ion', name: 'Ionian Sea', type: ProvinceType.sea, center: Offset(577, 1048)),
  'adr': const Province(id: 'adr', name: 'Adriatic Sea', type: ProvinceType.sea, center: Offset(540, 860)),
  'aeg': const Province(id: 'aeg', name: 'Aegean Sea', type: ProvinceType.sea, center: Offset(743, 973)),
  'eas': const Province(id: 'eas', name: 'Eastern Mediterranean', type: ProvinceType.sea, center: Offset(849, 1085)),
  'bla': const Province(id: 'bla', name: 'Black Sea', type: ProvinceType.sea, center: Offset(871, 840)),
};

/// Supply center locations (subset of provinces that are SCs).
final Set<String> supplyCenters = provinces.entries
    .where((e) => e.value.isSupplyCenter)
    .map((e) => e.key)
    .toSet();
