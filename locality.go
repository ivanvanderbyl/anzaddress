package anzaddress

import (
	"math/bits"
	"strings"
)

//go:generate go run ./cmd/locality-gen -source recorded -output localities_generated.go

type stateMask uint8

const (
	stateNSW stateMask = 1 << iota
	stateVIC
	stateQLD
	stateSA
	stateWA
	stateTAS
	stateACT
	stateNT
)

var stateMasks = map[string]stateMask{
	"NSW": stateNSW,
	"VIC": stateVIC,
	"QLD": stateQLD,
	"SA":  stateSA,
	"WA":  stateWA,
	"TAS": stateTAS,
	"ACT": stateACT,
	"NT":  stateNT,
}

func (mask stateMask) contains(state string) bool {
	stateBit, ok := stateMasks[state]
	return ok && mask&stateBit != 0
}

type localityMatch struct {
	name   string
	states stateMask
	next   int
}

func matchLocality(tokens []token, start int) (localityMatch, bool) {
	position := skipSoftTokens(tokens, start)
	parts := make([]string, 0, maxLocalityTokens)
	best := localityMatch{}
	found := false

	for position < len(tokens) && len(parts) < maxLocalityTokens {
		current := tokens[position]
		if current.kind != tokenWord && current.kind != tokenNumberish {
			break
		}

		parts = append(parts, current.value)
		position++

		name := strings.Join(parts, " ")
		if states, ok := localityStates[name]; ok {
			best = localityMatch{name: name, states: states, next: position}
			found = true
		}

		position = skipSoftTokens(tokens, position)
	}

	return best, found
}

func skipSoftTokens(tokens []token, position int) int {
	for position < len(tokens) {
		switch tokens[position].kind {
		case tokenComma, tokenNewline:
			position++
		default:
			return position
		}
	}
	return position
}

// inferState fills a missing state when the locality names only one, or when
// the postcode's state is one the locality spans: "Sydney 2000" is NSW.
func inferState(states stateMask, postcode string) string {
	if bits.OnesCount8(uint8(states)) == 1 {
		for name, bit := range stateMasks {
			if states == bit {
				return name
			}
		}
	}
	if state := postcodeState(postcode); state != "" && states.contains(state) {
		return state
	}
	return ""
}

// postcodeState maps an Australian postcode to its state by Australia Post's
// ranges. ACT ranges sit inside NSW's, so they are checked first.
func postcodeState(postcode string) string {
	if !isPostcode(postcode) {
		return ""
	}
	n := 0
	for _, ch := range postcode {
		n = n*10 + int(ch-'0')
	}
	switch {
	case n >= 200 && n <= 299, n >= 2600 && n <= 2618, n >= 2900 && n <= 2920:
		return "ACT"
	case n >= 800 && n <= 999:
		return "NT"
	case n >= 1000 && n <= 2999:
		return "NSW"
	case n >= 3000 && n <= 3999, n >= 8000 && n <= 8999:
		return "VIC"
	case n >= 4000 && n <= 4999, n >= 9000 && n <= 9999:
		return "QLD"
	case n >= 5000 && n <= 5999:
		return "SA"
	case n >= 6000 && n <= 6999:
		return "WA"
	case n >= 7000 && n <= 7999:
		return "TAS"
	}
	return ""
}

// WithInferredState returns a copy whose missing state is filled from the
// locality or postcode, for comparison. Parse keeps State as written.
func (a *ParsedAddress) WithInferredState() *ParsedAddress {
	if a == nil || a.State != "" || a.Country != CountryAU {
		return a
	}
	copied := *a
	copied.State = inferState(localityStates[a.Locality], a.Postcode)
	return &copied
}
