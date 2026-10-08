package anzaddress

import "testing"

// Office and apartment addresses are written many ways. Each case below is a
// spelling seen in email signatures that should parse to the same unit and
// level as its plain form.
func TestParseUnitAndLevelVariants(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		unit   string
		level  string
		number string
		street string
	}{
		{
			name:   "level then slash with spaces",
			input:  "Level 8 / 20 Bond Street Sydney NSW 2000",
			level:  "L 8",
			number: "20",
			street: "BOND",
		},
		{
			name:   "level then slash",
			input:  "Level 8/20 Bond Street, Sydney NSW 2000",
			level:  "L 8",
			number: "20",
			street: "BOND",
		},
		{
			name:   "compact level then slash",
			input:  "L8/20 Bond Street Sydney NSW 2000",
			level:  "L 8",
			number: "20",
			street: "BOND",
		},
		{
			name:   "named unit then slash",
			input:  "Unit 5/20 Smith Street, Fitzroy VIC 3065",
			unit:   "UNIT 5",
			number: "20",
			street: "SMITH",
		},
		{
			name:   "named unit then spaced slash",
			input:  "Unit 5 / 20 Smith Street Fitzroy VIC 3065",
			unit:   "UNIT 5",
			number: "20",
			street: "SMITH",
		},
		{
			name:   "suite then slash",
			input:  "Suite 3/100 Collins Street Melbourne VIC 3000",
			unit:   "SUITE 3",
			number: "100",
			street: "COLLINS",
		},
		{
			name:   "compact unit",
			input:  "U5 20 Smith St Fitzroy VIC 3065",
			unit:   "UNIT 5",
			number: "20",
			street: "SMITH",
		},
		{
			name:   "level before suite",
			input:  "Level 2 Suite 3 100 Collins Street Melbourne VIC 3000",
			unit:   "SUITE 3",
			level:  "L 2",
			number: "100",
			street: "COLLINS",
		},
		{
			name:   "bare slash stays a unit",
			input:  "8/20 Bond Street Sydney NSW 2000",
			unit:   "8",
			number: "20",
			street: "BOND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.input, err)
			}
			if addr.Unit != tt.unit {
				t.Errorf("Unit = %q, want %q", addr.Unit, tt.unit)
			}
			if addr.Level != tt.level {
				t.Errorf("Level = %q, want %q", addr.Level, tt.level)
			}
			if addr.StreetNumber != tt.number {
				t.Errorf("StreetNumber = %q, want %q", addr.StreetNumber, tt.number)
			}
			if addr.StreetName != tt.street {
				t.Errorf("StreetName = %q, want %q", addr.StreetName, tt.street)
			}
			if len(addr.NameLines) != 0 {
				t.Errorf("NameLines = %q, want none", addr.NameLines)
			}
		})
	}
}

// "5/20 Smith Street" is the usual way to write "Unit 5, 20 Smith Street", so
// a bare unit number matches a named unit with the same identifier.
func TestCompareBareUnitWithNamedUnit(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  MatchKind
	}{
		{
			name:  "bare and named unit",
			left:  "5/20 Smith Street, Fitzroy VIC 3065",
			right: "Unit 5, 20 Smith Street, Fitzroy VIC 3065",
			want:  ExactMatch,
		},
		{
			name:  "different unit numbers",
			left:  "6/20 Smith Street, Fitzroy VIC 3065",
			right: "Unit 5, 20 Smith Street, Fitzroy VIC 3065",
			want:  NoMatch,
		},
		{
			name:  "different named unit types",
			left:  "Apartment 5, 20 Smith Street, Fitzroy VIC 3065",
			right: "Unit 5, 20 Smith Street, Fitzroy VIC 3065",
			want:  NoMatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, err := Parse(tt.left)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.left, err)
			}
			right, err := Parse(tt.right)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.right, err)
			}
			if got := CompareAddresses(left, right).Kind; got != tt.want {
				t.Errorf("CompareAddresses kind = %v, want %v", got, tt.want)
			}
		})
	}
}
