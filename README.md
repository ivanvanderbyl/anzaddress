# anzaddress

Parse complete or partial Australian and New Zealand addresses from Go or the command line.
`anzaddress` normalises street, postal, locality, state, and postcode components
without a network connection.

Australian locality names are sourced from Geoscape G-NAF. New Zealand locality
names are sourced from Toitū Te Whenua / LINZ's NZ Suburbs and Localities dataset;
the recorded snapshot and required CC BY 4.0 attribution are in
`nz_localities_manifest.go`.

## Try the command

Install the binary with Go:

```bash
go install github.com/ivanvanderbyl/anzaddress/cmd/parse-address@latest
```

Parse an address directly:

```console
$ parse-address 'Level 4, 54 Wellington Street, Collingwood'
L 4 54 WELLINGTON ST
COLLINGWOOD
```

Pass one positional input containing one or more independently completed
addresses.

Add `--json` for structured output. The flag can appear before or after the
input:

```console
$ parse-address 'Level 4, 54 Wellington Street, Collingwood' --json
[{"deliveryPoints":[{"kind":"street","level":"L 4","streetNumber":"54","streetName":"WELLINGTON","streetType":"ST"}],"locality":"COLLINGWOOD"}]
```

The root command always returns a JSON array, including when it finds one
address.

Parse multiple addresses by separating them with newlines:

```console
$ parse-address 'Level 4, 54 Wellington St, Collingwood\nPO Box 234, Melbourne' --json | jq
[
  {
    "deliveryPoints": [
      {
        "kind": "street",
        "level": "L 4",
        "streetNumber": "54",
        "streetName": "WELLINGTON",
        "streetType": "ST"
      }
    ],
    "locality": "COLLINGWOOD"
  },
  {
    "deliveryPoints": [
      {
        "kind": "postal",
        "postalType": "PO BOX",
        "postalNumber": "234"
      }
    ],
    "locality": "MELBOURNE"
  }
]
```

The command accepts both literal `\n` sequences and actual newline characters
in the positional input. Without `--json`, it separates formatted addresses
with one blank line.

## Use the Go package

```bash
go get github.com/ivanvanderbyl/anzaddress
```

```go
package main

import (
    "fmt"
    "log"

    "github.com/ivanvanderbyl/anzaddress"
)

func main() {
    address, err := anzaddress.Parse("3A/45 High Street, Richmond")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(address.Unit)
    fmt.Println(address.FormatDeliveryLine())
    fmt.Println(address.Locality)
}
```

Output:

```text
3A
3A 45 HIGH ST
RICHMOND
```

## How an address is recognised

The core grammar is:

```text
Address       := DeliveryPoint+ Locality State? Postcode?
DeliveryPoint := StreetDelivery | PostalDelivery
```

Parsing is greedy and left to right. Commas and line breaks are soft
boundaries. A known locality completes an address; state and postcode add
specificity but are optional. This makes `123 Main Street, Richmond` valid and
`123 Main Street` invalid.

Delivery points stay in encounter order. A street and PO box that share one
locality form one address:

```console
$ parse-address '123 Main Street, PO Box 42, Richmond VIC 3121'
123 MAIN ST
PO BOX 42
RICHMOND VIC 3121
```

The parser normalises common forms as it reads them, including `Street` to
`ST`, `Level 4` and `L4` to `L 4`, and `P.O. Box` to `PO BOX`.

## Compare addresses

Use `compare` to compare normalised components rather than raw strings:

```console
$ parse-address compare 'L4 54 Wellington Street, Collingwood' 'Level 4, 54 Wellington St, Collingwood'
exact
```

A missing unit, level, state, or postcode produces a partial match when all
populated components agree:

```console
$ parse-address compare '54 Wellington Street, Collingwood' '54 Wellington St, Collingwood VIC 3066'
partial
matched through: locality
missing from left: state, postcode
```

Conflicting populated components, such as different street numbers or
localities, produce `no match`. A valid `no match` result still exits with
status zero. Add `--json` to receive the kind, matched component, missing
components, and deterministic keys as camelCase JSON fields.

The same comparison is available from Go:

```go
left, _ := anzaddress.Parse("123 Main Street, Richmond")
right, _ := anzaddress.Parse("123 Main St, Richmond VIC 3121")

match := anzaddress.CompareAddresses(left, right)
fmt.Println(match.Kind == anzaddress.PartialMatch)                 // true
fmt.Println(match.MatchedThrough == anzaddress.MatchLocality)     // true
fmt.Println(match.MissingFromLeft[0] == anzaddress.MatchState)    // true
fmt.Println(match.MissingFromLeft[1] == anzaddress.MatchPostcode) // true
```

`ComparisonKey` exposes the canonical components as a deterministic string.
Comparison does not use edit distance, phonetic matching, or typo correction.
A bare unit from `5/20 Smith Street` matches a named `Unit 5` with the same
identifier.

`Parse` keeps the state as written. Call `WithInferredState` before comparing
to fill a missing Australian state from a single-state locality or from the
postcode, so `Sydney 2000` and `Sydney NSW 2000` share a key:

```go
left, _ := anzaddress.Parse("L8 20 Bond Street, Sydney 2000")
right, _ := anzaddress.Parse("Level 8, 20 Bond Street, Sydney NSW 2000")
fmt.Println(left.WithInferredState().ComparisonKey() == right.ComparisonKey()) // true
```

## Parse more than one address from Go

Call `ParseAll` to parse multiple addresses in Go:

```go
addresses, err := anzaddress.ParseAll(`School Infrastructure NSW
Level 8, 259 George Street, Sydney, NSW 2000
GPO Box 33, Sydney, NSW 2001`)
if err != nil {
    return err
}

fmt.Println(len(addresses))         // 2
fmt.Println(addresses[0].Postcode)  // 2000
fmt.Println(addresses[1].PoBoxType) // GPO BOX
fmt.Println(addresses[1].Postcode)  // 2001
```

Physical lines do not determine the result count. Each independently
locality-terminated delivery sequence becomes one result. Shared recipient or
organisation lines are copied to each result as `NameLines`.

`DeliveryPoints` is the canonical ordered representation when encounter order
or multiple delivery points matter. The flat street and PO box fields remain
populated for compatibility and represent the first delivery point of each
kind.

## Handle parsing failures

The default parser is lenient. It returns recognised structure and records the
first grammar or validation failure in `ParsedAddress.Errors`:

```go
address, err := anzaddress.Parse(input)
if err != nil {
    return err // Empty input is always returned directly.
}
if len(address.Errors) > 0 {
    // Inspect a lenient parsing failure.
}
```

Strict mode returns the first failure directly. With `ParseAll`, it also
returns any complete addresses parsed before that failure:

```go
parser := anzaddress.NewParser(anzaddress.WithStrict(true))
addresses, err := parser.ParseAll(input)
```

An address is valid when it has no recorded errors, at least one delivery
point, and a recognised locality:

```go
if address.IsValid() {
    // The input satisfies the grammar.
}
```

The command-line tool uses strict parsing. Failures go to standard error and
exit non-zero. If any address fails, the command writes no successful addresses
to standard output. With `--json`, failures use the same machine-readable mode:

```json
{"error":"invalid address format"}
```

## Supported components

- States and territories: full names and abbreviations, normalised to NSW, VIC,
  QLD, SA, WA, TAS, ACT, and NT.
- Street types: G-NAF forms including STREET/ST, ROAD/RD, AVENUE/AV,
  DRIVE/DR, PLACE/PL, COURT/CT, and their normalised abbreviations.
- Unit types: UNIT, FLAT, APARTMENT/APT, VILLA, LOT, SHOP, SUITE, ROOM,
  OFFICE, FACTORY, WAREHOUSE, and related G-NAF forms.
- Level types: LEVEL/L, FLOOR/FL, GROUND/G, BASEMENT/B, MEZZANINE/M,
  LOWER GROUND/LG, UPPER GROUND/UG, and related forms.
- Postal delivery types: PO BOX, GPO BOX, LOCKED BAG, PRIVATE BAG, REPLY
  PAID, RMB, CMB, RSD, MS, CMA, CPA, and CARE PO.

## Locality data

The package embeds 21,852 normalised primary and alias locality names from the
Geoscape Geocoded National Address File (G-NAF). Each name stores an eight-bit
state mask, so duplicate locality names across states occupy one index entry.
Parsing does not need a network connection.

Refresh the index from the latest GDA2020 release published in the official
data.gov.au package metadata:

```bash
task gnaf:update
```

The generator reads only the locality tables from the remote ZIP by using HTTP
range requests. It records the resolved archive URL in
`localities_generated.go`.

## Scope and limitations

Pass the address text you want to parse. The package does not search arbitrary
prose or an email signature to locate an address.

The parser validates grammar, known locality names, locality and state
compatibility, and four-digit postcode shape. It does not prove that a street
address exists, validate that a postcode belongs to a locality, query
Australia Post's Postal Address File, handle international addresses, or
provide Address Matching Approval System certification.

## Development and releases

```bash
task test
task gnaf:update
task gnaf:verify
task release VERSION=v1.2.3
task release:push VERSION=v1.2.3
```

Every pull request merged into `main` creates the next patch version tag and a
GitHub Release with generated notes. While a pull request is open, its workflow
summary shows the proposed release tag.

`task test` checks Go formatting, runs `go vet`, runs all Go tests, and tests
the release-version calculator and ordered release reconciliation.

`gnaf:verify` regenerates the locality index from the source URL recorded in the
generated file and compares it byte for byte.

`release` and `release:push` remain available for manual recovery, rather than
the normal release process. `release` runs the G-NAF check, requires a clean
worktree, and creates an annotated local semantic-version tag. It never pushes.
It also does not create a GitHub Release. `release:push` is the separate explicit
push step; rerun release reconciliation afterward to publish the matching
GitHub Release. A manually selected version must be greater than the preceding
release, after which automatic releases continue by incrementing its patch.

## References

- [Australia Post addressing guidelines](https://auspost.com.au/sending/guidelines/addressing-guidelines)
- [Australia Post addressing standards](https://auspost.com.au/content/dam/auspost_corp/media/documents/australia-post-addressing-standards-1999.pdf)
- [AMAS developer guide](https://auspost.com.au/content/dam/auspost_corp/media/documents/amas-developer-guide.pdf)
- [Geocoded National Address File](https://data.gov.au/data/dataset/geocoded-national-address-file-g-naf)

## Licence

MIT
