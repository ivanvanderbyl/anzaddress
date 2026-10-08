package anzaddress

import "strings"

// MatchKind classifies the relationship between two parsed addresses.
type MatchKind uint8

const (
	// NoMatch means at least one populated identity component conflicts or an input is invalid.
	NoMatch MatchKind = iota
	// PartialMatch means populated components agree but one side omits specificity.
	PartialMatch
	// ExactMatch means both addresses have identical canonical components.
	ExactMatch
)

// MatchComponent identifies one ordered address component in a comparison result.
type MatchComponent uint8

const (
	// MatchNone means no populated component matched.
	MatchNone MatchComponent = iota
	// MatchDeliveryPoint identifies the presence of a compatible delivery-point kind.
	MatchDeliveryPoint
	// MatchUnit identifies a street unit.
	MatchUnit
	// MatchLevel identifies a building level.
	MatchLevel
	// MatchStreetNumber identifies a street number.
	MatchStreetNumber
	// MatchStreetName identifies a street name.
	MatchStreetName
	// MatchStreetType identifies a street type.
	MatchStreetType
	// MatchStreetSuffix identifies a directional street suffix.
	MatchStreetSuffix
	// MatchPostalType identifies a postal delivery type.
	MatchPostalType
	// MatchPostalNumber identifies a postal delivery identifier.
	MatchPostalNumber
	// MatchLocality identifies a locality.
	MatchLocality
	// MatchState identifies a state or territory.
	MatchState
	// MatchPostcode identifies a postcode.
	MatchPostcode
)

// AddressMatch explains an exact, partial, or conflicting address comparison.
type AddressMatch struct {
	Kind             MatchKind
	MatchedThrough   MatchComponent
	MissingFromLeft  []MatchComponent
	MissingFromRight []MatchComponent
	LeftKey          string
	RightKey         string
}

// ComparisonKey returns a deterministic key containing canonical and missing components.
func (a *ParsedAddress) ComparisonKey() string {
	if a == nil {
		return ""
	}

	var key strings.Builder
	if a.Country == CountryNZ {
		key.WriteString("NZ|")
	}
	for index, point := range comparisonDeliveryPoints(a) {
		if index > 0 {
			key.WriteByte('+')
		}
		switch point.Kind {
		case DeliveryPointStreet:
			key.WriteString("STREET{")
			writeKeyField(&key, "UNIT", point.Street.Unit)
			writeKeyField(&key, "LEVEL", point.Street.Level)
			writeKeyField(&key, "NUMBER", point.Street.StreetNumber)
			writeKeyField(&key, "NAME", point.Street.StreetName)
			writeKeyField(&key, "TYPE", point.Street.StreetType)
			writeFinalKeyField(&key, "SUFFIX", point.Street.StreetSuffix)
			key.WriteByte('}')
		case DeliveryPointPostal:
			key.WriteString("POSTAL{")
			writeKeyField(&key, "TYPE", point.Postal.Type)
			writeFinalKeyField(&key, "NUMBER", point.Postal.Number)
			key.WriteByte('}')
		}
	}
	writeAddressKeyField(&key, "LOCALITY", a.Locality)
	writeAddressKeyField(&key, "STATE", a.State)
	writeAddressKeyField(&key, "POSTCODE", a.Postcode)
	return key.String()
}

// CompareAddresses compares canonical populated components without fuzzy matching.
func CompareAddresses(left, right *ParsedAddress) AddressMatch {
	match := AddressMatch{}
	if left != nil {
		match.LeftKey = left.ComparisonKey()
	}
	if right != nil {
		match.RightKey = right.ComparisonKey()
	}
	if left == nil || right == nil {
		return match
	}
	if !left.IsValid() || !right.IsValid() {
		return match
	}
	if left.Country != right.Country {
		return match
	}

	leftPoints := comparisonDeliveryPoints(left)
	rightPoints := comparisonDeliveryPoints(right)
	commonPoints := min(len(leftPoints), len(rightPoints))
	conflict := false

	for index := 0; index < commonPoints; index++ {
		leftPoint := leftPoints[index]
		rightPoint := rightPoints[index]
		if leftPoint.Kind != rightPoint.Kind {
			conflict = true
			continue
		}
		match.MatchedThrough = MatchDeliveryPoint
		switch leftPoint.Kind {
		case DeliveryPointStreet:
			leftUnit, rightUnit := comparableUnits(leftPoint.Street.Unit, rightPoint.Street.Unit)
			compareComponent(&match, MatchUnit, leftUnit, rightUnit, &conflict)
			compareComponent(&match, MatchLevel, leftPoint.Street.Level, rightPoint.Street.Level, &conflict)
			compareComponent(&match, MatchStreetNumber, leftPoint.Street.StreetNumber, rightPoint.Street.StreetNumber, &conflict)
			compareComponent(&match, MatchStreetName, leftPoint.Street.StreetName, rightPoint.Street.StreetName, &conflict)
			compareComponent(&match, MatchStreetType, leftPoint.Street.StreetType, rightPoint.Street.StreetType, &conflict)
			compareComponent(&match, MatchStreetSuffix, leftPoint.Street.StreetSuffix, rightPoint.Street.StreetSuffix, &conflict)
		case DeliveryPointPostal:
			compareComponent(&match, MatchPostalType, leftPoint.Postal.Type, rightPoint.Postal.Type, &conflict)
			compareComponent(&match, MatchPostalNumber, leftPoint.Postal.Number, rightPoint.Postal.Number, &conflict)
		default:
			conflict = true
		}
	}

	for index := commonPoints; index < len(leftPoints); index++ {
		appendMissing(&match.MissingFromRight, MatchDeliveryPoint)
	}
	for index := commonPoints; index < len(rightPoints); index++ {
		appendMissing(&match.MissingFromLeft, MatchDeliveryPoint)
	}

	compareComponent(&match, MatchLocality, left.Locality, right.Locality, &conflict)
	compareComponent(&match, MatchState, left.State, right.State, &conflict)
	compareComponent(&match, MatchPostcode, left.Postcode, right.Postcode, &conflict)

	if conflict {
		match.Kind = NoMatch
		return match
	}
	if len(match.MissingFromLeft) > 0 || len(match.MissingFromRight) > 0 {
		match.Kind = PartialMatch
		return match
	}
	match.Kind = ExactMatch
	return match
}

func comparisonDeliveryPoints(addr *ParsedAddress) []DeliveryPoint {
	if len(addr.DeliveryPoints) > 0 {
		return addr.DeliveryPoints
	}

	points := make([]DeliveryPoint, 0, 2)
	if addr.StreetNumber != "" || addr.StreetName != "" {
		points = append(points, DeliveryPoint{
			Kind: DeliveryPointStreet,
			Street: StreetDelivery{
				Unit:         addr.Unit,
				Level:        addr.Level,
				StreetNumber: addr.StreetNumber,
				StreetName:   addr.StreetName,
				StreetType:   addr.StreetType,
				StreetSuffix: addr.StreetSuffix,
			},
		})
	}
	if addr.IsPoBox || addr.PoBoxType != "" || addr.PoBoxNumber != "" {
		points = append(points, DeliveryPoint{
			Kind: DeliveryPointPostal,
			Postal: PostalDelivery{
				Type:   addr.PoBoxType,
				Number: addr.PoBoxNumber,
			},
		})
	}
	return points
}

func compareComponent(match *AddressMatch, component MatchComponent, left, right string, conflict *bool) {
	switch {
	case left == "" && right == "":
		return
	case left == "":
		appendMissing(&match.MissingFromLeft, component)
	case right == "":
		appendMissing(&match.MissingFromRight, component)
	case left != right:
		*conflict = true
	default:
		match.MatchedThrough = component
	}
}

func appendMissing(components *[]MatchComponent, component MatchComponent) {
	for _, existing := range *components {
		if existing == component {
			return
		}
	}
	*components = append(*components, component)
}

func writeKeyField(key *strings.Builder, name, value string) {
	key.WriteString(name)
	key.WriteByte('=')
	key.WriteString(value)
	key.WriteByte(';')
}

func writeFinalKeyField(key *strings.Builder, name, value string) {
	key.WriteString(name)
	key.WriteByte('=')
	key.WriteString(value)
}

func writeAddressKeyField(key *strings.Builder, name, value string) {
	key.WriteByte('|')
	writeFinalKeyField(key, name, value)
}

// comparableUnits compares a bare "5" (from "5/20") with "UNIT 5" by
// identifier, since the slash form never names the unit type.
func comparableUnits(left, right string) (string, string) {
	leftType, leftID := splitUnit(left)
	rightType, rightID := splitUnit(right)
	if leftType == "" || rightType == "" {
		return leftID, rightID
	}
	return left, right
}

func splitUnit(unit string) (string, string) {
	if unitType, identifier, ok := strings.Cut(unit, " "); ok {
		return unitType, identifier
	}
	return "", unit
}
