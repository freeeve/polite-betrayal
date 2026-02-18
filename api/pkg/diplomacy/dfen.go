package diplomacy

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// powerToChar maps a Power constant to the DFEN single-character abbreviation.
var powerToChar = map[Power]byte{
	Austria: 'A',
	England: 'E',
	France:  'F',
	Germany: 'G',
	Italy:   'I',
	Russia:  'R',
	Turkey:  'T',
	Neutral: 'N',
}

// charToPower maps a DFEN single character back to a Power constant.
var charToPower = map[byte]Power{
	'A': Austria,
	'E': England,
	'F': France,
	'G': Germany,
	'I': Italy,
	'R': Russia,
	'T': Turkey,
	'N': Neutral,
}

// powerOrder defines the canonical ordering for DFEN output.
var powerOrder = []Power{Austria, England, France, Germany, Italy, Russia, Turkey}

// seasonToChar maps Season to DFEN character.
var seasonToChar = map[Season]byte{
	Spring: 's',
	Fall:   'f',
}

// charToSeason maps DFEN character to Season.
var charToSeason = map[byte]Season{
	's': Spring,
	'f': Fall,
}

// phaseToChar maps PhaseType to DFEN character.
var phaseToChar = map[PhaseType]byte{
	PhaseMovement: 'm',
	PhaseRetreat:  'r',
	PhaseBuild:    'b',
}

// charToPhase maps DFEN character to PhaseType.
var charToPhase = map[byte]PhaseType{
	'm': PhaseMovement,
	'r': PhaseRetreat,
	'b': PhaseBuild,
}

// EncodeDFEN serializes a GameState to a DFEN string.
// The output is deterministic: units and supply centers are sorted by
// power order (A,E,F,G,I,R,T) then alphabetically within each power.
func EncodeDFEN(gs *GameState) string {
	var b strings.Builder
	b.Grow(512)

	encodePhaseInfo(&b, gs)
	b.WriteByte('/')
	encodeUnits(&b, gs)
	b.WriteByte('/')
	encodeSupplyCenters(&b, gs)
	b.WriteByte('/')
	encodeDislodged(&b, gs)

	return b.String()
}

// encodePhaseInfo writes the year+season+phase portion of DFEN.
func encodePhaseInfo(b *strings.Builder, gs *GameState) {
	b.WriteString(strconv.Itoa(gs.Year))
	b.WriteByte(seasonToChar[gs.Season])
	b.WriteByte(phaseToChar[gs.Phase])
}

// encodeUnitLocation writes a location with optional dot-separated coast for DFEN.
func encodeUnitLocation(b *strings.Builder, province string, coast Coast) {
	b.WriteString(province)
	if coast != NoCoast {
		b.WriteByte('.')
		b.WriteString(string(coast))
	}
}

// encodeUnits writes the units section, sorted by power then province.
func encodeUnits(b *strings.Builder, gs *GameState) {
	if len(gs.Units) == 0 {
		b.WriteByte('-')
		return
	}

	grouped := groupUnitsByPower(gs.Units)
	first := true
	for _, power := range powerOrder {
		units := grouped[power]
		sort.Slice(units, func(i, j int) bool {
			return units[i].Province < units[j].Province
		})
		for _, u := range units {
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.WriteByte(powerToChar[u.Power])
			if u.Type == Army {
				b.WriteByte('a')
			} else {
				b.WriteByte('f')
			}
			encodeUnitLocation(b, u.Province, u.Coast)
		}
	}

	if first {
		b.WriteByte('-')
	}
}

// encodeSupplyCenters writes the SC section, sorted by power then province.
func encodeSupplyCenters(b *strings.Builder, gs *GameState) {
	type scEntry struct {
		power Power
		prov  string
	}

	grouped := make(map[Power][]string)
	for prov, power := range gs.SupplyCenters {
		grouped[power] = append(grouped[power], prov)
	}
	for _, provs := range grouped {
		sort.Strings(provs)
	}

	allPowers := append([]Power{}, powerOrder...)
	allPowers = append(allPowers, Neutral)

	first := true
	for _, power := range allPowers {
		for _, prov := range grouped[power] {
			if !first {
				b.WriteByte(',')
			}
			first = false
			b.WriteByte(powerToChar[power])
			b.WriteString(prov)
		}
	}
}

// encodeDislodged writes the dislodged section.
func encodeDislodged(b *strings.Builder, gs *GameState) {
	if len(gs.Dislodged) == 0 {
		b.WriteByte('-')
		return
	}

	sorted := make([]DislodgedUnit, len(gs.Dislodged))
	copy(sorted, gs.Dislodged)
	sort.Slice(sorted, func(i, j int) bool {
		pi := powerToChar[sorted[i].Unit.Power]
		pj := powerToChar[sorted[j].Unit.Power]
		if pi != pj {
			return pi < pj
		}
		return sorted[i].Unit.Province < sorted[j].Unit.Province
	})

	for i, d := range sorted {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte(powerToChar[d.Unit.Power])
		if d.Unit.Type == Army {
			b.WriteByte('a')
		} else {
			b.WriteByte('f')
		}
		encodeUnitLocation(b, d.Unit.Province, d.Unit.Coast)
		b.WriteByte('<')
		b.WriteString(d.AttackerFrom)
	}
}

// groupUnitsByPower groups a slice of units by their owning power.
func groupUnitsByPower(units []Unit) map[Power][]Unit {
	grouped := make(map[Power][]Unit)
	for _, u := range units {
		grouped[u.Power] = append(grouped[u.Power], u)
	}
	return grouped
}

// DecodeDFEN parses a DFEN string into a GameState.
func DecodeDFEN(s string) (*GameState, error) {
	parts := strings.SplitN(s, "/", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("dfen: expected 4 sections separated by '/', got %d", len(parts))
	}

	gs := &GameState{}

	if err := decodePhaseInfo(parts[0], gs); err != nil {
		return nil, err
	}
	if err := decodeUnits(parts[1], gs); err != nil {
		return nil, err
	}
	if err := decodeSupplyCenters(parts[2], gs); err != nil {
		return nil, err
	}
	if err := decodeDislodged(parts[3], gs); err != nil {
		return nil, err
	}

	return gs, nil
}

// decodePhaseInfo parses "1901sm" into year, season, phase.
func decodePhaseInfo(s string, gs *GameState) error {
	if len(s) < 3 {
		return fmt.Errorf("dfen: phase info too short: %q", s)
	}

	phaseChar := s[len(s)-1]
	seasonChar := s[len(s)-2]
	yearStr := s[:len(s)-2]

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return fmt.Errorf("dfen: invalid year %q: %w", yearStr, err)
	}

	season, ok := charToSeason[seasonChar]
	if !ok {
		return fmt.Errorf("dfen: invalid season %q", string(seasonChar))
	}

	phase, ok := charToPhase[phaseChar]
	if !ok {
		return fmt.Errorf("dfen: invalid phase %q", string(phaseChar))
	}

	gs.Year = year
	gs.Season = season
	gs.Phase = phase
	return nil
}

// decodeUnits parses "Aavie,Aabud,Aftri,..." or "-".
func decodeUnits(s string, gs *GameState) error {
	if s == "-" {
		return nil
	}

	for entry := range strings.SplitSeq(s, ",") {
		u, err := parseUnitEntry(entry)
		if err != nil {
			return fmt.Errorf("dfen: unit %q: %w", entry, err)
		}
		gs.Units = append(gs.Units, u)
	}
	return nil
}

// parseUnitEntry parses a single unit entry like "Aavie" or "Rfstp.sc".
func parseUnitEntry(s string) (Unit, error) {
	if len(s) < 5 {
		return Unit{}, fmt.Errorf("too short")
	}

	power, ok := charToPower[s[0]]
	if !ok || power == Neutral {
		return Unit{}, fmt.Errorf("invalid power char %q", string(s[0]))
	}

	var unitType UnitType
	switch s[1] {
	case 'a':
		unitType = Army
	case 'f':
		unitType = Fleet
	default:
		return Unit{}, fmt.Errorf("invalid unit type %q", string(s[1]))
	}

	loc := s[2:]
	province, coast, err := parseDFENLocation(loc)
	if err != nil {
		return Unit{}, err
	}

	return Unit{
		Type:     unitType,
		Power:    power,
		Province: province,
		Coast:    coast,
	}, nil
}

// parseDFENLocation parses a DFEN location like "vie" or "stp.sc".
func parseDFENLocation(s string) (string, Coast, error) {
	parts := strings.SplitN(s, ".", 2)
	province := parts[0]
	if len(province) != 3 {
		return "", NoCoast, fmt.Errorf("invalid province id %q (must be 3 lowercase letters)", province)
	}

	coast := NoCoast
	if len(parts) == 2 {
		c := Coast(parts[1])
		switch c {
		case NorthCoast, SouthCoast, EastCoast:
			coast = c
		default:
			return "", NoCoast, fmt.Errorf("invalid coast %q", parts[1])
		}
	}

	return province, coast, nil
}

// decodeSupplyCenters parses "Abud,Atri,Avie,...".
func decodeSupplyCenters(s string, gs *GameState) error {
	gs.SupplyCenters = make(map[string]Power)
	for entry := range strings.SplitSeq(s, ",") {
		if len(entry) < 4 {
			return fmt.Errorf("dfen: sc entry too short: %q", entry)
		}
		power, ok := charToPower[entry[0]]
		if !ok {
			return fmt.Errorf("dfen: invalid power in sc %q", entry)
		}
		prov := entry[1:]
		if len(prov) != 3 {
			return fmt.Errorf("dfen: invalid province in sc %q", entry)
		}
		gs.SupplyCenters[prov] = power
	}
	return nil
}

// decodeDislodged parses "Aaser<bul,Rfsev<bla" or "-".
func decodeDislodged(s string, gs *GameState) error {
	if s == "-" {
		return nil
	}

	for entry := range strings.SplitSeq(s, ",") {
		d, err := parseDislodgedEntry(entry)
		if err != nil {
			return fmt.Errorf("dfen: dislodged %q: %w", entry, err)
		}
		gs.Dislodged = append(gs.Dislodged, d)
	}
	return nil
}

// parseDislodgedEntry parses "Aaser<bul" or "Rfstp.sc<rum".
func parseDislodgedEntry(s string) (DislodgedUnit, error) {
	unitPart, attackerFrom, ok := strings.Cut(s, "<")
	if !ok {
		return DislodgedUnit{}, fmt.Errorf("missing '<' separator")
	}

	if len(attackerFrom) != 3 {
		return DislodgedUnit{}, fmt.Errorf("invalid attacker province %q", attackerFrom)
	}

	u, err := parseUnitEntry(unitPart)
	if err != nil {
		return DislodgedUnit{}, err
	}

	return DislodgedUnit{
		Unit:          u,
		DislodgedFrom: u.Province,
		AttackerFrom:  attackerFrom,
	}, nil
}
