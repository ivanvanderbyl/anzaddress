package anzaddress

import "strings"

type keywordTable struct {
	values   map[string]string
	maxWords int
}

var (
	postalKeywords = newKeywordTable(deliveryPointKeywords)
	unitKeywords   = newKeywordTable(unitTypes)
	levelKeywords  = newKeywordTable(levelTypes)
	stateKeywords  = newKeywordTable(stateTypes)
)

func newKeywordTable(source map[string]string) keywordTable {
	table := keywordTable{values: make(map[string]string, len(source))}
	for keyword, normalized := range source {
		keyword = normalizeAddressAtom(strings.Join(strings.Fields(keyword), " "))
		table.values[keyword] = normalized
		if words := len(strings.Fields(keyword)); words > table.maxWords {
			table.maxWords = words
		}
	}
	return table
}

func matchKeyword(tokens []token, start, limit int, table keywordTable) (string, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	parts := make([]string, 0, table.maxWords)
	bestValue := ""
	bestNext := position

	for position < limit && len(parts) < table.maxWords {
		current := tokens[position]
		if current.kind != tokenWord && current.kind != tokenNumberish {
			break
		}

		parts = append(parts, current.value)
		position++
		if value, ok := table.values[strings.Join(parts, " ")]; ok {
			bestValue = value
			bestNext = position
		}
		position = skipSoftTokensBefore(tokens, position, limit)
	}

	return bestValue, bestNext, bestValue != ""
}

func recognizePostal(tokens []token, start, limit int) (DeliveryPoint, int, bool) {
	postalType, position, ok := matchKeyword(tokens, start, limit, postalKeywords)
	if !ok {
		return DeliveryPoint{}, start, false
	}

	position = skipSoftTokensBefore(tokens, position, limit)
	if position >= limit || !isAddressAtom(tokens[position]) {
		return DeliveryPoint{}, start, false
	}

	delivery := DeliveryPoint{
		Kind: DeliveryPointPostal,
		Postal: PostalDelivery{
			Type:   postalType,
			Number: tokens[position].value,
		},
	}
	return delivery, position + 1, true
}

func recognizeStreet(tokens []token, start, limit int) (DeliveryPoint, int, bool) {
	start = skipSoftTokensBefore(tokens, start, limit)
	streetLimit := nextPostalStart(tokens, start+1, limit)
	if streetLimit < 0 {
		streetLimit = limit
	}

	position := start
	delivery := StreetDelivery{}

	first, firstNext, ok := consumeAtom(tokens, position, streetLimit)
	if !ok {
		return DeliveryPoint{}, start, false
	}
	if first.kind == tokenNumberish {
		slashPosition := skipSoftTokensBefore(tokens, firstNext, streetLimit)
		if slashPosition < streetLimit && tokens[slashPosition].kind == tokenSlash {
			streetNumber, streetNumberNext, numberOK := consumeAtom(tokens, slashPosition+1, streetLimit)
			if !numberOK || streetNumber.kind != tokenNumberish {
				return DeliveryPoint{}, start, false
			}
			// "L8/20": a compact level before the slash is a level, not a unit.
			if level, ok := compactLevel(first.value); ok {
				delivery.Level = level
			} else if unit, ok := compactUnit(first.value); ok {
				delivery.Unit = unit
			} else {
				delivery.Unit = first.value
			}
			delivery.StreetNumber = streetNumber.value
			position = streetNumberNext
		}
	}

	if delivery.StreetNumber == "" {
		// Unit and level may come in either order: "Suite 3, Level 2" or
		// "Level 2 Suite 3".
		for range 2 {
			if delivery.Unit == "" {
				if unit, next, matched := matchUnit(tokens, position, streetLimit); matched {
					delivery.Unit = unit
					position = next
					continue
				}
			}
			if delivery.Level == "" {
				if levelType, next, matched := matchKeyword(tokens, position, streetLimit, levelKeywords); matched {
					if levelNeedsIdentifier(levelType) {
						identifier, identifierNext, identifierOK := consumeAtom(tokens, next, streetLimit)
						if !identifierOK {
							return DeliveryPoint{}, start, false
						}
						delivery.Level = strings.TrimSpace(levelType + " " + identifier.value)
						position = identifierNext
					} else {
						delivery.Level = levelType
						position = next
					}
					continue
				} else if level, next, matched := matchCompactLevel(tokens, position, streetLimit); matched {
					delivery.Level = level
					position = next
					continue
				}
			}
			break
		}
		// "Level 8 / 20" and "Unit 5/20": a slash separates a named unit or
		// level from the street number.
		if delivery.Unit != "" || delivery.Level != "" {
			if slash := skipSoftTokensBefore(tokens, position, streetLimit); slash < streetLimit && tokens[slash].kind == tokenSlash {
				position = slash + 1
			}
		}

		streetNumber, next, numberOK := consumeAtom(tokens, position, streetLimit)
		if !numberOK || streetNumber.kind != tokenNumberish {
			return DeliveryPoint{}, start, false
		}
		delivery.StreetNumber = streetNumber.value
		position = next
	}

	atoms := make([]token, 0, streetLimit-position)
	for position < streetLimit {
		current := tokens[position]
		position++
		if isSoftToken(current) {
			continue
		}
		if !isAddressAtom(current) {
			return DeliveryPoint{}, start, false
		}
		atoms = append(atoms, current)
	}
	if len(atoms) == 0 {
		return DeliveryPoint{}, start, false
	}

	if normalized, suffix := streetSuffixes[atoms[len(atoms)-1].value]; suffix && len(atoms) > 1 {
		delivery.StreetSuffix = normalized
		atoms = atoms[:len(atoms)-1]
	}
	if normalized, streetType := streetTypes[atoms[len(atoms)-1].value]; streetType && len(atoms) > 1 {
		delivery.StreetType = normalized
		atoms = atoms[:len(atoms)-1]
	}
	if len(atoms) == 0 {
		return DeliveryPoint{}, start, false
	}

	name := make([]string, len(atoms))
	for i, atom := range atoms {
		name[i] = atom.value
	}
	delivery.StreetName = strings.Join(name, " ")

	return DeliveryPoint{Kind: DeliveryPointStreet, Street: delivery}, streetLimit, true
}

func matchCompactLevel(tokens []token, start, limit int) (string, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	if position >= limit || tokens[position].kind != tokenNumberish {
		return "", start, false
	}
	level, ok := compactLevel(tokens[position].value)
	if !ok || !numberFollows(tokens, position+1, limit) {
		return "", start, false
	}
	return level, position + 1, true
}

// numberFollows reports a street number next, optionally after a slash or
// another unit or level, so "L8 20", "L8/20" and "U5 L2 100" all qualify.
func numberFollows(tokens []token, start, limit int) bool {
	position := skipSoftTokensBefore(tokens, start, limit)
	if position < limit && tokens[position].kind == tokenSlash {
		position = skipSoftTokensBefore(tokens, position+1, limit)
	}
	if position >= limit || tokens[position].kind != tokenNumberish {
		return false
	}
	if _, ok := compactLevel(tokens[position].value); ok {
		return numberFollows(tokens, position+1, limit)
	}
	if _, ok := compactUnit(tokens[position].value); ok {
		return numberFollows(tokens, position+1, limit)
	}
	return true
}

// matchUnit reads "Unit 5", "Suite 3" or a compact "U5".
func matchUnit(tokens []token, start, limit int) (string, int, bool) {
	if unitType, next, matched := matchKeyword(tokens, start, limit, unitKeywords); matched {
		identifier, identifierNext, identifierOK := consumeAtom(tokens, next, limit)
		if !identifierOK {
			return "", start, false
		}
		return strings.TrimSpace(unitType + " " + identifier.value), identifierNext, true
	}
	position := skipSoftTokensBefore(tokens, start, limit)
	if position >= limit || tokens[position].kind != tokenNumberish {
		return "", start, false
	}
	unit, ok := compactUnit(tokens[position].value)
	if !ok || !numberFollows(tokens, position+1, limit) {
		return "", start, false
	}
	return unit, position + 1, true
}

// compactLevel reads "L8" as level 8.
func compactLevel(value string) (string, bool) {
	return compactKeyword(value, levelKeywords, levelNeedsIdentifier, false)
}

// compactUnit reads "U5" as unit 5. The identifier must start with a digit
// so a street number such as "SE1" is not taken for a suite.
func compactUnit(value string) (string, bool) {
	return compactKeyword(value, unitKeywords, nil, true)
}

// compactKeyword splits a single-word keyword prefix from its identifier.
// The longest prefix wins.
func compactKeyword(value string, table keywordTable, needsIdentifier func(string) bool, requireDigit bool) (string, bool) {
	best, bestType := "", ""
	for keyword, normalized := range table.values {
		if strings.Contains(keyword, " ") || (needsIdentifier != nil && !needsIdentifier(normalized)) {
			continue
		}
		identifier := strings.TrimPrefix(value, keyword)
		if identifier == value || identifier == "" || !isNumberish(identifier) || (requireDigit && !startsWithDigit(identifier)) {
			continue
		}
		if len(keyword) > len(best) {
			best, bestType = keyword, normalized
		}
	}
	if best == "" {
		return "", false
	}
	return bestType + " " + strings.TrimPrefix(value, best), true
}

func startsWithDigit(value string) bool {
	return value != "" && value[0] >= '0' && value[0] <= '9'
}

func recognizeDeliverySequence(tokens []token, start, limit int) ([]DeliveryPoint, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	points := make([]DeliveryPoint, 0, 2)

	for position < limit {
		if postal, next, ok := recognizePostal(tokens, position, limit); ok {
			points = append(points, postal)
			position = skipSoftTokensBefore(tokens, next, limit)
			continue
		}
		if street, next, ok := recognizeStreet(tokens, position, limit); ok {
			points = append(points, street)
			position = skipSoftTokensBefore(tokens, next, limit)
			continue
		}
		return nil, start, false
	}

	return points, position, len(points) > 0
}

func nextPostalStart(tokens []token, start, limit int) int {
	for position := skipSoftTokensBefore(tokens, start, limit); position < limit; position++ {
		if _, _, ok := recognizePostal(tokens, position, limit); ok {
			return position
		}
	}
	return -1
}

func consumeAtom(tokens []token, start, limit int) (token, int, bool) {
	position := skipSoftTokensBefore(tokens, start, limit)
	if position >= limit || !isAddressAtom(tokens[position]) {
		return token{}, start, false
	}
	return tokens[position], position + 1, true
}

func skipSoftTokensBefore(tokens []token, position, limit int) int {
	for position < limit && isSoftToken(tokens[position]) {
		position++
	}
	return position
}

func isSoftToken(current token) bool {
	return current.kind == tokenComma || current.kind == tokenNewline
}

func isAddressAtom(current token) bool {
	return current.kind == tokenWord || current.kind == tokenNumberish
}

func levelNeedsIdentifier(levelType string) bool {
	switch levelType {
	case "L", "FL":
		return true
	default:
		return false
	}
}
