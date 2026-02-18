//! Province definitions and metadata for the standard Diplomacy map.
//!
//! All 75 provinces are enumerated in alphabetical order by their 3-letter ID.
//! Province metadata (name, type, supply center status, home power) is stored
//! in a compile-time lookup table indexed by the `Province` enum discriminant.

/// The number of provinces on the standard Diplomacy map.
pub const PROVINCE_COUNT: usize = 75;

/// The number of supply centers on the standard Diplomacy map.
pub const SUPPLY_CENTER_COUNT: usize = 34;

/// A province on the standard Diplomacy map.
///
/// Variants are in alphabetical order by 3-letter abbreviation.
/// The `#[repr(u8)]` attribute enables use as an array index.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
#[repr(u8)]
pub enum Province {
    Adr = 0,  // Adriatic Sea
    Aeg = 1,  // Aegean Sea
    Alb = 2,  // Albania
    Ank = 3,  // Ankara
    Apu = 4,  // Apulia
    Arm = 5,  // Armenia
    Bal = 6,  // Baltic Sea
    Bar = 7,  // Barents Sea
    Bel = 8,  // Belgium
    Ber = 9,  // Berlin
    Bla = 10, // Black Sea
    Boh = 11, // Bohemia
    Bot = 12, // Gulf of Bothnia
    Bre = 13, // Brest
    Bud = 14, // Budapest
    Bul = 15, // Bulgaria
    Bur = 16, // Burgundy
    Cly = 17, // Clyde
    Con = 18, // Constantinople
    Den = 19, // Denmark
    Eas = 20, // Eastern Mediterranean
    Edi = 21, // Edinburgh
    Eng = 22, // English Channel
    Fin = 23, // Finland
    Gal = 24, // Galicia
    Gas = 25, // Gascony
    Gol = 26, // Gulf of Lyon
    Gre = 27, // Greece
    Hel = 28, // Heligoland Bight
    Hol = 29, // Holland
    Ion = 30, // Ionian Sea
    Iri = 31, // Irish Sea
    Kie = 32, // Kiel
    Lon = 33, // London
    Lvn = 34, // Livonia
    Lvp = 35, // Liverpool
    Mao = 36, // Mid-Atlantic Ocean
    Mar = 37, // Marseilles
    Mos = 38, // Moscow
    Mun = 39, // Munich
    Naf = 40, // North Africa
    Nao = 41, // North Atlantic Ocean
    Nap = 42, // Naples
    Nrg = 43, // Norwegian Sea
    Nth = 44, // North Sea
    Nwy = 45, // Norway
    Par = 46, // Paris
    Pic = 47, // Picardy
    Pie = 48, // Piedmont
    Por = 49, // Portugal
    Pru = 50, // Prussia
    Rom = 51, // Rome
    Ruh = 52, // Ruhr
    Rum = 53, // Rumania
    Ser = 54, // Serbia
    Sev = 55, // Sevastopol
    Sil = 56, // Silesia
    Ska = 57, // Skagerrak
    Smy = 58, // Smyrna
    Spa = 59, // Spain
    Stp = 60, // St. Petersburg
    Swe = 61, // Sweden
    Syr = 62, // Syria
    Tri = 63, // Trieste
    Tun = 64, // Tunisia
    Tus = 65, // Tuscany
    Tyr = 66, // Tyrolia
    Tys = 67, // Tyrrhenian Sea
    Ukr = 68, // Ukraine
    Ven = 69, // Venice
    Vie = 70, // Vienna
    Wal = 71, // Wales
    War = 72, // Warsaw
    Wes = 73, // Western Mediterranean
    Yor = 74, // Yorkshire
}

/// All province variants in index order.
pub const ALL_PROVINCES: [Province; PROVINCE_COUNT] = [
    Province::Adr, Province::Aeg, Province::Alb, Province::Ank,
    Province::Apu, Province::Arm, Province::Bal, Province::Bar,
    Province::Bel, Province::Ber, Province::Bla, Province::Boh,
    Province::Bot, Province::Bre, Province::Bud, Province::Bul,
    Province::Bur, Province::Cly, Province::Con, Province::Den,
    Province::Eas, Province::Edi, Province::Eng, Province::Fin,
    Province::Gal, Province::Gas, Province::Gol, Province::Gre,
    Province::Hel, Province::Hol, Province::Ion, Province::Iri,
    Province::Kie, Province::Lon, Province::Lvn, Province::Lvp,
    Province::Mao, Province::Mar, Province::Mos, Province::Mun,
    Province::Naf, Province::Nao, Province::Nap, Province::Nrg,
    Province::Nth, Province::Nwy, Province::Par, Province::Pic,
    Province::Pie, Province::Por, Province::Pru, Province::Rom,
    Province::Ruh, Province::Rum, Province::Ser, Province::Sev,
    Province::Sil, Province::Ska, Province::Smy, Province::Spa,
    Province::Stp, Province::Swe, Province::Syr, Province::Tri,
    Province::Tun, Province::Tus, Province::Tyr, Province::Tys,
    Province::Ukr, Province::Ven, Province::Vie, Province::Wal,
    Province::War, Province::Wes, Province::Yor,
];

impl Province {
    /// Returns the 3-letter abbreviation for this province.
    pub const fn abbr(self) -> &'static str {
        PROVINCE_INFO[self as usize].abbr
    }

    /// Returns the full display name for this province.
    pub const fn name(self) -> &'static str {
        PROVINCE_INFO[self as usize].name
    }

    /// Returns the province type (Land, Sea, or Coastal).
    pub const fn province_type(self) -> ProvinceType {
        PROVINCE_INFO[self as usize].province_type
    }

    /// Returns true if this province is a supply center.
    pub const fn is_supply_center(self) -> bool {
        PROVINCE_INFO[self as usize].is_supply_center
    }

    /// Returns the home power for this province, or None if neutral.
    pub const fn home_power(self) -> Option<Power> {
        PROVINCE_INFO[self as usize].home_power
    }

    /// Returns the available coasts for split-coast provinces, empty otherwise.
    pub const fn coasts(self) -> &'static [Coast] {
        PROVINCE_INFO[self as usize].coasts
    }

    /// Returns true if this province has split coasts.
    pub const fn has_coasts(self) -> bool {
        !PROVINCE_INFO[self as usize].coasts.is_empty()
    }

    /// Looks up a province by its 3-letter abbreviation.
    pub fn from_abbr(abbr: &str) -> Option<Province> {
        ABBR_TABLE.iter().find(|(a, _)| *a == abbr).map(|(_, p)| *p)
    }
}

/// Coast specifier for split-coast provinces.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Coast {
    None,
    North,
    South,
    East,
}

impl Coast {
    /// Returns the 2-letter abbreviation (empty string for None).
    pub const fn abbr(self) -> &'static str {
        match self {
            Coast::None => "",
            Coast::North => "nc",
            Coast::South => "sc",
            Coast::East => "ec",
        }
    }

    /// Parses a coast from its 2-letter abbreviation.
    pub fn from_abbr(s: &str) -> Option<Coast> {
        match s {
            "" => Some(Coast::None),
            "nc" => Some(Coast::North),
            "sc" => Some(Coast::South),
            "ec" => Some(Coast::East),
            _ => Option::None,
        }
    }
}

/// Classifies a province by terrain type.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ProvinceType {
    Land,
    Sea,
    Coastal,
}

/// One of the seven great powers.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Power {
    Austria,
    England,
    France,
    Germany,
    Italy,
    Russia,
    Turkey,
}

/// All seven powers in standard order.
pub const ALL_POWERS: [Power; 7] = [
    Power::Austria,
    Power::England,
    Power::France,
    Power::Germany,
    Power::Italy,
    Power::Russia,
    Power::Turkey,
];

impl Power {
    /// Returns the lowercase full name of this power.
    pub const fn name(self) -> &'static str {
        match self {
            Power::Austria => "austria",
            Power::England => "england",
            Power::France => "france",
            Power::Germany => "germany",
            Power::Italy => "italy",
            Power::Russia => "russia",
            Power::Turkey => "turkey",
        }
    }

    /// Returns the single-character DUI protocol abbreviation.
    pub const fn dui_char(self) -> char {
        match self {
            Power::Austria => 'A',
            Power::England => 'E',
            Power::France => 'F',
            Power::Germany => 'G',
            Power::Italy => 'I',
            Power::Russia => 'R',
            Power::Turkey => 'T',
        }
    }

    /// Parses a power from its lowercase full name.
    pub fn from_name(name: &str) -> Option<Power> {
        match name {
            "austria" => Some(Power::Austria),
            "england" => Some(Power::England),
            "france" => Some(Power::France),
            "germany" => Some(Power::Germany),
            "italy" => Some(Power::Italy),
            "russia" => Some(Power::Russia),
            "turkey" => Some(Power::Turkey),
            _ => Option::None,
        }
    }

    /// Parses a power from its single-character DUI abbreviation.
    pub fn from_dui_char(c: char) -> Option<Power> {
        match c {
            'A' => Some(Power::Austria),
            'E' => Some(Power::England),
            'F' => Some(Power::France),
            'G' => Some(Power::Germany),
            'I' => Some(Power::Italy),
            'R' => Some(Power::Russia),
            'T' => Some(Power::Turkey),
            _ => Option::None,
        }
    }
}

/// Static metadata for a province.
pub struct ProvinceInfo {
    pub abbr: &'static str,
    pub name: &'static str,
    pub province_type: ProvinceType,
    pub is_supply_center: bool,
    pub home_power: Option<Power>,
    pub coasts: &'static [Coast],
}

/// Compile-time lookup table: index by `Province as usize`.
pub static PROVINCE_INFO: [ProvinceInfo; PROVINCE_COUNT] = [
    // 0: Adr - Adriatic Sea
    ProvinceInfo { abbr: "adr", name: "Adriatic Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 1: Aeg - Aegean Sea
    ProvinceInfo { abbr: "aeg", name: "Aegean Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 2: Alb - Albania
    ProvinceInfo { abbr: "alb", name: "Albania", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 3: Ank - Ankara
    ProvinceInfo { abbr: "ank", name: "Ankara", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Turkey), coasts: &[] },
    // 4: Apu - Apulia
    ProvinceInfo { abbr: "apu", name: "Apulia", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 5: Arm - Armenia
    ProvinceInfo { abbr: "arm", name: "Armenia", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 6: Bal - Baltic Sea
    ProvinceInfo { abbr: "bal", name: "Baltic Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 7: Bar - Barents Sea
    ProvinceInfo { abbr: "bar", name: "Barents Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 8: Bel - Belgium
    ProvinceInfo { abbr: "bel", name: "Belgium", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 9: Ber - Berlin
    ProvinceInfo { abbr: "ber", name: "Berlin", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Germany), coasts: &[] },
    // 10: Bla - Black Sea
    ProvinceInfo { abbr: "bla", name: "Black Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 11: Boh - Bohemia
    ProvinceInfo { abbr: "boh", name: "Bohemia", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 12: Bot - Gulf of Bothnia
    ProvinceInfo { abbr: "bot", name: "Gulf of Bothnia", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 13: Bre - Brest
    ProvinceInfo { abbr: "bre", name: "Brest", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::France), coasts: &[] },
    // 14: Bud - Budapest
    ProvinceInfo { abbr: "bud", name: "Budapest", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::Austria), coasts: &[] },
    // 15: Bul - Bulgaria
    ProvinceInfo { abbr: "bul", name: "Bulgaria", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[Coast::East, Coast::South] },
    // 16: Bur - Burgundy
    ProvinceInfo { abbr: "bur", name: "Burgundy", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 17: Cly - Clyde
    ProvinceInfo { abbr: "cly", name: "Clyde", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 18: Con - Constantinople
    ProvinceInfo { abbr: "con", name: "Constantinople", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Turkey), coasts: &[] },
    // 19: Den - Denmark
    ProvinceInfo { abbr: "den", name: "Denmark", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 20: Eas - Eastern Mediterranean
    ProvinceInfo { abbr: "eas", name: "Eastern Mediterranean", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 21: Edi - Edinburgh
    ProvinceInfo { abbr: "edi", name: "Edinburgh", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::England), coasts: &[] },
    // 22: Eng - English Channel
    ProvinceInfo { abbr: "eng", name: "English Channel", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 23: Fin - Finland
    ProvinceInfo { abbr: "fin", name: "Finland", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 24: Gal - Galicia
    ProvinceInfo { abbr: "gal", name: "Galicia", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 25: Gas - Gascony
    ProvinceInfo { abbr: "gas", name: "Gascony", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 26: Gol - Gulf of Lyon
    ProvinceInfo { abbr: "gol", name: "Gulf of Lyon", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 27: Gre - Greece
    ProvinceInfo { abbr: "gre", name: "Greece", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 28: Hel - Heligoland Bight
    ProvinceInfo { abbr: "hel", name: "Heligoland Bight", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 29: Hol - Holland
    ProvinceInfo { abbr: "hol", name: "Holland", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 30: Ion - Ionian Sea
    ProvinceInfo { abbr: "ion", name: "Ionian Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 31: Iri - Irish Sea
    ProvinceInfo { abbr: "iri", name: "Irish Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 32: Kie - Kiel
    ProvinceInfo { abbr: "kie", name: "Kiel", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Germany), coasts: &[] },
    // 33: Lon - London
    ProvinceInfo { abbr: "lon", name: "London", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::England), coasts: &[] },
    // 34: Lvn - Livonia
    ProvinceInfo { abbr: "lvn", name: "Livonia", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 35: Lvp - Liverpool
    ProvinceInfo { abbr: "lvp", name: "Liverpool", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::England), coasts: &[] },
    // 36: Mao - Mid-Atlantic Ocean
    ProvinceInfo { abbr: "mao", name: "Mid-Atlantic Ocean", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 37: Mar - Marseilles
    ProvinceInfo { abbr: "mar", name: "Marseilles", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::France), coasts: &[] },
    // 38: Mos - Moscow
    ProvinceInfo { abbr: "mos", name: "Moscow", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::Russia), coasts: &[] },
    // 39: Mun - Munich
    ProvinceInfo { abbr: "mun", name: "Munich", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::Germany), coasts: &[] },
    // 40: Naf - North Africa
    ProvinceInfo { abbr: "naf", name: "North Africa", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 41: Nao - North Atlantic Ocean
    ProvinceInfo { abbr: "nao", name: "North Atlantic Ocean", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 42: Nap - Naples
    ProvinceInfo { abbr: "nap", name: "Naples", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Italy), coasts: &[] },
    // 43: Nrg - Norwegian Sea
    ProvinceInfo { abbr: "nrg", name: "Norwegian Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 44: Nth - North Sea
    ProvinceInfo { abbr: "nth", name: "North Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 45: Nwy - Norway
    ProvinceInfo { abbr: "nwy", name: "Norway", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 46: Par - Paris
    ProvinceInfo { abbr: "par", name: "Paris", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::France), coasts: &[] },
    // 47: Pic - Picardy
    ProvinceInfo { abbr: "pic", name: "Picardy", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 48: Pie - Piedmont
    ProvinceInfo { abbr: "pie", name: "Piedmont", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 49: Por - Portugal
    ProvinceInfo { abbr: "por", name: "Portugal", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 50: Pru - Prussia
    ProvinceInfo { abbr: "pru", name: "Prussia", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 51: Rom - Rome
    ProvinceInfo { abbr: "rom", name: "Rome", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Italy), coasts: &[] },
    // 52: Ruh - Ruhr
    ProvinceInfo { abbr: "ruh", name: "Ruhr", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 53: Rum - Rumania
    ProvinceInfo { abbr: "rum", name: "Rumania", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 54: Ser - Serbia
    ProvinceInfo { abbr: "ser", name: "Serbia", province_type: ProvinceType::Land, is_supply_center: true, home_power: None, coasts: &[] },
    // 55: Sev - Sevastopol
    ProvinceInfo { abbr: "sev", name: "Sevastopol", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Russia), coasts: &[] },
    // 56: Sil - Silesia
    ProvinceInfo { abbr: "sil", name: "Silesia", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 57: Ska - Skagerrak
    ProvinceInfo { abbr: "ska", name: "Skagerrak", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 58: Smy - Smyrna
    ProvinceInfo { abbr: "smy", name: "Smyrna", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Turkey), coasts: &[] },
    // 59: Spa - Spain
    ProvinceInfo { abbr: "spa", name: "Spain", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[Coast::North, Coast::South] },
    // 60: Stp - St. Petersburg
    ProvinceInfo { abbr: "stp", name: "St. Petersburg", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Russia), coasts: &[Coast::North, Coast::South] },
    // 61: Swe - Sweden
    ProvinceInfo { abbr: "swe", name: "Sweden", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 62: Syr - Syria
    ProvinceInfo { abbr: "syr", name: "Syria", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 63: Tri - Trieste
    ProvinceInfo { abbr: "tri", name: "Trieste", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Austria), coasts: &[] },
    // 64: Tun - Tunisia
    ProvinceInfo { abbr: "tun", name: "Tunisia", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: None, coasts: &[] },
    // 65: Tus - Tuscany
    ProvinceInfo { abbr: "tus", name: "Tuscany", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 66: Tyr - Tyrolia
    ProvinceInfo { abbr: "tyr", name: "Tyrolia", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 67: Tys - Tyrrhenian Sea
    ProvinceInfo { abbr: "tys", name: "Tyrrhenian Sea", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 68: Ukr - Ukraine
    ProvinceInfo { abbr: "ukr", name: "Ukraine", province_type: ProvinceType::Land, is_supply_center: false, home_power: None, coasts: &[] },
    // 69: Ven - Venice
    ProvinceInfo { abbr: "ven", name: "Venice", province_type: ProvinceType::Coastal, is_supply_center: true, home_power: Some(Power::Italy), coasts: &[] },
    // 70: Vie - Vienna
    ProvinceInfo { abbr: "vie", name: "Vienna", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::Austria), coasts: &[] },
    // 71: Wal - Wales
    ProvinceInfo { abbr: "wal", name: "Wales", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
    // 72: War - Warsaw
    ProvinceInfo { abbr: "war", name: "Warsaw", province_type: ProvinceType::Land, is_supply_center: true, home_power: Some(Power::Russia), coasts: &[] },
    // 73: Wes - Western Mediterranean
    ProvinceInfo { abbr: "wes", name: "Western Mediterranean", province_type: ProvinceType::Sea, is_supply_center: false, home_power: None, coasts: &[] },
    // 74: Yor - Yorkshire
    ProvinceInfo { abbr: "yor", name: "Yorkshire", province_type: ProvinceType::Coastal, is_supply_center: false, home_power: None, coasts: &[] },
];

/// Abbreviation-to-Province lookup table (sorted alphabetically).
static ABBR_TABLE: [(&str, Province); PROVINCE_COUNT] = [
    ("adr", Province::Adr), ("aeg", Province::Aeg), ("alb", Province::Alb),
    ("ank", Province::Ank), ("apu", Province::Apu), ("arm", Province::Arm),
    ("bal", Province::Bal), ("bar", Province::Bar), ("bel", Province::Bel),
    ("ber", Province::Ber), ("bla", Province::Bla), ("boh", Province::Boh),
    ("bot", Province::Bot), ("bre", Province::Bre), ("bud", Province::Bud),
    ("bul", Province::Bul), ("bur", Province::Bur), ("cly", Province::Cly),
    ("con", Province::Con), ("den", Province::Den), ("eas", Province::Eas),
    ("edi", Province::Edi), ("eng", Province::Eng), ("fin", Province::Fin),
    ("gal", Province::Gal), ("gas", Province::Gas), ("gol", Province::Gol),
    ("gre", Province::Gre), ("hel", Province::Hel), ("hol", Province::Hol),
    ("ion", Province::Ion), ("iri", Province::Iri), ("kie", Province::Kie),
    ("lon", Province::Lon), ("lvn", Province::Lvn), ("lvp", Province::Lvp),
    ("mao", Province::Mao), ("mar", Province::Mar), ("mos", Province::Mos),
    ("mun", Province::Mun), ("naf", Province::Naf), ("nao", Province::Nao),
    ("nap", Province::Nap), ("nrg", Province::Nrg), ("nth", Province::Nth),
    ("nwy", Province::Nwy), ("par", Province::Par), ("pic", Province::Pic),
    ("pie", Province::Pie), ("por", Province::Por), ("pru", Province::Pru),
    ("rom", Province::Rom), ("ruh", Province::Ruh), ("rum", Province::Rum),
    ("ser", Province::Ser), ("sev", Province::Sev), ("sil", Province::Sil),
    ("ska", Province::Ska), ("smy", Province::Smy), ("spa", Province::Spa),
    ("stp", Province::Stp), ("swe", Province::Swe), ("syr", Province::Syr),
    ("tri", Province::Tri), ("tun", Province::Tun), ("tus", Province::Tus),
    ("tyr", Province::Tyr), ("tys", Province::Tys), ("ukr", Province::Ukr),
    ("ven", Province::Ven), ("vie", Province::Vie), ("wal", Province::Wal),
    ("war", Province::War), ("wes", Province::Wes), ("yor", Province::Yor),
];

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn province_count_is_75() {
        assert_eq!(ALL_PROVINCES.len(), 75);
        assert_eq!(PROVINCE_COUNT, 75);
    }

    #[test]
    fn supply_center_count_is_34() {
        let sc_count = ALL_PROVINCES.iter()
            .filter(|p| p.is_supply_center())
            .count();
        assert_eq!(sc_count, SUPPLY_CENTER_COUNT);
    }

    #[test]
    fn province_indices_are_sequential() {
        for (i, p) in ALL_PROVINCES.iter().enumerate() {
            assert_eq!(*p as usize, i, "Province {:?} has wrong index", p);
        }
    }

    #[test]
    fn abbr_roundtrip() {
        for p in ALL_PROVINCES.iter() {
            let abbr = p.abbr();
            let roundtrip = Province::from_abbr(abbr)
                .unwrap_or_else(|| panic!("Failed to look up abbreviation '{}'", abbr));
            assert_eq!(*p, roundtrip);
        }
    }

    #[test]
    fn province_type_counts() {
        let land = ALL_PROVINCES.iter().filter(|p| p.province_type() == ProvinceType::Land).count();
        let sea = ALL_PROVINCES.iter().filter(|p| p.province_type() == ProvinceType::Sea).count();
        let coastal = ALL_PROVINCES.iter().filter(|p| p.province_type() == ProvinceType::Coastal).count();
        assert_eq!(land, 14, "Expected 14 inland provinces");
        assert_eq!(sea, 19, "Expected 19 sea provinces");
        assert_eq!(coastal, 42, "Expected 42 coastal provinces (39 + 3 split-coast)");
        assert_eq!(land + sea + coastal, 75);
    }

    #[test]
    fn split_coast_provinces() {
        assert_eq!(Province::Bul.coasts(), &[Coast::East, Coast::South]);
        assert_eq!(Province::Spa.coasts(), &[Coast::North, Coast::South]);
        assert_eq!(Province::Stp.coasts(), &[Coast::North, Coast::South]);

        let split_count = ALL_PROVINCES.iter().filter(|p| p.has_coasts()).count();
        assert_eq!(split_count, 3);
    }

    #[test]
    fn home_supply_center_counts() {
        let count_for = |power: Power| -> usize {
            ALL_PROVINCES.iter()
                .filter(|p| p.is_supply_center() && p.home_power() == Some(power))
                .count()
        };
        assert_eq!(count_for(Power::Austria), 3); // bud, tri, vie
        assert_eq!(count_for(Power::England), 3); // edi, lon, lvp
        assert_eq!(count_for(Power::France), 3);  // bre, mar, par
        assert_eq!(count_for(Power::Germany), 3);  // ber, kie, mun
        assert_eq!(count_for(Power::Italy), 3);    // nap, rom, ven
        assert_eq!(count_for(Power::Russia), 4);   // mos, sev, stp, war
        assert_eq!(count_for(Power::Turkey), 3);   // ank, con, smy

        let neutral_sc = ALL_PROVINCES.iter()
            .filter(|p| p.is_supply_center() && p.home_power().is_none())
            .count();
        assert_eq!(neutral_sc, 12);
    }

    #[test]
    fn all_powers() {
        assert_eq!(ALL_POWERS.len(), 7);
        for p in &ALL_POWERS {
            let name = p.name();
            assert_eq!(Power::from_name(name), Some(*p));
        }
    }

    #[test]
    fn coast_abbr_roundtrip() {
        for c in &[Coast::None, Coast::North, Coast::South, Coast::East] {
            let abbr = c.abbr();
            let roundtrip = Coast::from_abbr(abbr).unwrap();
            assert_eq!(*c, roundtrip);
        }
    }

    #[test]
    fn unknown_abbr_returns_none() {
        assert_eq!(Province::from_abbr("xyz"), None);
        assert_eq!(Province::from_abbr(""), None);
    }
}
