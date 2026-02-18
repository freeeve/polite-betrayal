"""Province name normalization for Diplomacy datasets.

Maps variant province names from different data sources to our canonical
3-letter lowercase abbreviations matching engine/src/board/province.rs.
"""

# Canonical 3-letter province codes (lowercase, alphabetical).
PROVINCES = [
    "adr", "aeg", "alb", "ank", "apu", "arm", "bal", "bar", "bel", "ber",
    "bla", "boh", "bot", "bre", "bud", "bul", "bur", "cly", "con", "den",
    "eas", "edi", "eng", "fin", "gal", "gas", "gol", "gre", "hel", "hol",
    "ion", "iri", "kie", "lon", "lvn", "lvp", "mao", "mar", "mos", "mun",
    "naf", "nao", "nap", "nrg", "nth", "nwy", "par", "pic", "pie", "por",
    "pru", "rom", "ruh", "rum", "ser", "sev", "sil", "ska", "smy", "spa",
    "stp", "swe", "syr", "tri", "tun", "tus", "tyr", "tys", "ukr", "ven",
    "vie", "wal", "war", "wes", "yor",
]

PROVINCE_SET = frozenset(PROVINCES)

# Split-coast provinces and their valid coasts.
SPLIT_COASTS = {
    "bul": ["ec", "sc"],
    "spa": ["nc", "sc"],
    "stp": ["nc", "sc"],
}

# Maps alternate names used in datasets to our canonical abbreviation.
# The diplomacy Python library uses 3-letter uppercase codes; some datasets
# use full names or non-standard abbreviations.
ALIAS_MAP = {
    # Full names (title case)
    "adriatic sea": "adr",
    "aegean sea": "aeg",
    "albania": "alb",
    "ankara": "ank",
    "apulia": "apu",
    "armenia": "arm",
    "baltic sea": "bal",
    "barents sea": "bar",
    "belgium": "bel",
    "berlin": "ber",
    "black sea": "bla",
    "bohemia": "boh",
    "gulf of bothnia": "bot",
    "brest": "bre",
    "budapest": "bud",
    "bulgaria": "bul",
    "burgundy": "bur",
    "clyde": "cly",
    "constantinople": "con",
    "denmark": "den",
    "eastern mediterranean": "eas",
    "edinburgh": "edi",
    "english channel": "eng",
    "finland": "fin",
    "galicia": "gal",
    "gascony": "gas",
    "gulf of lyon": "gol",
    "gulf of lyons": "gol",
    "greece": "gre",
    "heligoland bight": "hel",
    "helgoland bight": "hel",
    "holland": "hol",
    "ionian sea": "ion",
    "irish sea": "iri",
    "kiel": "kie",
    "london": "lon",
    "livonia": "lvn",
    "liverpool": "lvp",
    "mid-atlantic ocean": "mao",
    "mid atlantic ocean": "mao",
    "marseilles": "mar",
    "moscow": "mos",
    "munich": "mun",
    "north africa": "naf",
    "north atlantic ocean": "nao",
    "naples": "nap",
    "norwegian sea": "nrg",
    "north sea": "nth",
    "norway": "nwy",
    "paris": "par",
    "picardy": "pic",
    "piedmont": "pie",
    "portugal": "por",
    "prussia": "pru",
    "rome": "rom",
    "ruhr": "ruh",
    "rumania": "rum",
    "romania": "rum",
    "serbia": "ser",
    "sevastopol": "sev",
    "silesia": "sil",
    "skagerrak": "ska",
    "smyrna": "smy",
    "spain": "spa",
    "st petersburg": "stp",
    "st. petersburg": "stp",
    "saint petersburg": "stp",
    "sweden": "swe",
    "syria": "syr",
    "trieste": "tri",
    "tunisia": "tun",
    "tuscany": "tus",
    "tyrolia": "tyr",
    "tyrrhenian sea": "tys",
    "ukraine": "ukr",
    "venice": "ven",
    "vienna": "vie",
    "wales": "wal",
    "warsaw": "war",
    "western mediterranean": "wes",
    "yorkshire": "yor",
    # Common alternate abbreviations from webdiplomacy / other tools
    "adr": "adr", "aeg": "aeg", "alb": "alb", "ank": "ank", "apu": "apu",
    "arm": "arm", "bal": "bal", "bar": "bar", "bel": "bel", "ber": "ber",
    "bla": "bla", "boh": "boh", "bot": "bot", "bre": "bre", "bud": "bud",
    "bul": "bul", "bur": "bur", "cly": "cly", "con": "con", "den": "den",
    "eas": "eas", "edi": "edi", "eng": "eng", "fin": "fin", "gal": "gal",
    "gas": "gas", "gol": "gol", "gre": "gre", "hel": "hel", "hol": "hol",
    "ion": "ion", "iri": "iri", "kie": "kie", "lon": "lon", "lvn": "lvn",
    "lvp": "lvp", "mao": "mao", "mar": "mar", "mos": "mos", "mun": "mun",
    "naf": "naf", "nao": "nao", "nap": "nap", "nrg": "nrg", "nth": "nth",
    "nwy": "nwy", "par": "par", "pic": "pic", "pie": "pie", "por": "por",
    "pru": "pru", "rom": "rom", "ruh": "ruh", "rum": "rum", "ser": "ser",
    "sev": "sev", "sil": "sil", "ska": "ska", "smy": "smy", "spa": "spa",
    "stp": "stp", "swe": "swe", "syr": "syr", "tri": "tri", "tun": "tun",
    "tus": "tus", "tyr": "tyr", "tys": "tys", "ukr": "ukr", "ven": "ven",
    "vie": "vie", "wal": "wal", "war": "war", "wes": "wes", "yor": "yor",
    # webDiplomacy long-form sea names
    "ech": "eng",
    "gob": "bot",
    "lyo": "gol",
    "mat": "mao",
    "nwg": "nrg",
    "wme": "wes",
    "eme": "eas",
    "ion": "ion",
    "tyn": "tys",
    "spa/nc": "spa",
    "spa/sc": "spa",
    "stp/nc": "stp",
    "stp/sc": "stp",
    "bul/ec": "bul",
    "bul/sc": "bul",
}

POWER_NAMES = ["austria", "england", "france", "germany", "italy", "russia", "turkey"]
POWER_SET = frozenset(POWER_NAMES)

POWER_ALIASES = {
    "austria": "austria",
    "england": "england",
    "france": "france",
    "germany": "germany",
    "italy": "italy",
    "russia": "russia",
    "turkey": "turkey",
    "austria-hungary": "austria",
    "AUSTRIA": "austria",
    "ENGLAND": "england",
    "FRANCE": "france",
    "GERMANY": "germany",
    "ITALY": "italy",
    "RUSSIA": "russia",
    "TURKEY": "turkey",
}


def normalize_province(name: str) -> str | None:
    """Normalize a province name to its canonical 3-letter code.

    Returns None if the name cannot be resolved.
    """
    key = name.strip().lower()
    if key in PROVINCE_SET:
        return key
    result = ALIAS_MAP.get(key)
    if result:
        return result
    # Try stripping coast suffix for compound names like "BUL/EC"
    if "/" in key:
        base = key.split("/")[0].strip()
        return normalize_province(base)
    return None


def extract_coast(name: str) -> str:
    """Extract coast suffix from a province name like 'SPA/NC' or 'STP/SC'.

    Returns empty string if no coast is specified.
    """
    key = name.strip().lower()
    if "/" in key:
        parts = key.split("/")
        if len(parts) == 2:
            coast = parts[1].strip()
            if coast in ("nc", "sc", "ec"):
                return coast
    return ""


def normalize_power(name: str) -> str | None:
    """Normalize a power name to lowercase canonical form."""
    key = name.strip().lower()
    if key in POWER_SET:
        return key
    return POWER_ALIASES.get(name.strip())
