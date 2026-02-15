/** SVG viewBox dimensions — province centers are calibrated to this space. */
export const SVG_VIEWBOX = 1152;

/**
 * Province center coordinates in SVG viewbox space (1152x1152).
 * Ported from ui/lib/core/map/province_data.dart.
 * Only land/coastal provinces — sea zones excluded (tested via API).
 */
export const PROVINCE_CENTERS: Record<string, { x: number; y: number }> = {
  // === BRITISH ISLES ===
  cly: { x: 255, y: 440 },
  edi: { x: 298, y: 440 },
  lvp: { x: 274, y: 500 },
  yor: { x: 300, y: 540 },
  wal: { x: 234, y: 576 },
  lon: { x: 305, y: 588 },

  // === SCANDINAVIA ===
  nwy: { x: 500, y: 390 },
  swe: { x: 590, y: 400 },
  den: { x: 500, y: 505 },
  fin: { x: 694, y: 340 },
  stp: { x: 775, y: 420 },

  // === FRANCE ===
  bre: { x: 210, y: 660 },
  pic: { x: 300, y: 660 },
  par: { x: 315, y: 690 },
  bur: { x: 350, y: 740 },
  gas: { x: 240, y: 800 },
  mar: { x: 345, y: 825 },

  // === IBERIA ===
  spa: { x: 154, y: 872 },
  por: { x: 42, y: 870 },

  // === LOW COUNTRIES ===
  bel: { x: 342, y: 620 },
  hol: { x: 380, y: 598 },

  // === GERMANY ===
  ruh: { x: 417, y: 665 },
  kie: { x: 470, y: 575 },
  ber: { x: 525, y: 605 },
  mun: { x: 470, y: 715 },
  sil: { x: 560, y: 640 },
  pru: { x: 603, y: 590 },

  // === AUSTRIA-HUNGARY ===
  tyr: { x: 470, y: 765 },
  boh: { x: 530, y: 690 },
  vie: { x: 570, y: 733 },
  bud: { x: 625, y: 758 },
  tri: { x: 540, y: 800 },
  gal: { x: 670, y: 694 },

  // === ITALY ===
  pie: { x: 394, y: 810 },
  ven: { x: 475, y: 800 },
  tus: { x: 445, y: 850 },
  rom: { x: 485, y: 890 },
  apu: { x: 534, y: 900 },
  nap: { x: 530, y: 932 },

  // === BALKANS ===
  ser: { x: 650, y: 845 },
  alb: { x: 610, y: 905 },
  gre: { x: 680, y: 985 },
  bul: { x: 720, y: 885 },
  rum: { x: 760, y: 815 },

  // === TURKEY ===
  con: { x: 805, y: 912 },
  ank: { x: 920, y: 920 },
  smy: { x: 810, y: 990 },
  arm: { x: 1030, y: 900 },
  syr: { x: 1020, y: 1015 },

  // === RUSSIA ===
  mos: { x: 920, y: 490 },
  war: { x: 655, y: 625 },
  ukr: { x: 778, y: 700 },
  sev: { x: 912, y: 795 },
  lvn: { x: 704, y: 524 },

  // === NORTH AFRICA ===
  naf: { x: 198, y: 1051 },
  tun: { x: 410, y: 1058 },

  // === SEA ZONES (needed for convoy tests) ===
  nth: { x: 358, y: 498 },
  eng: { x: 230, y: 630 },
  mao: { x: 44, y: 750 },
};
