package enrich

// PigParse REST API parser. Ported 1:1 from apps-script/src/lib/pigparse-types.ts
// (`parseToRows`, `isValidRow`, `coerceRow`, REQUIRED_KEYS/NUMERIC_KEYS, the 1%
// malformation tolerance). Schema verified against the live capture in
// testdata/pigparse-getall-1.json (7,240 rows on 2026-05-09).
//
// Pure: no network and no SQL imports. Returns ALL valid raw rows in source
// order. It does NOT dedup the t=0 (WTS) / t=1 (WTB) duplicate item_id rows —
// that filter (D-9: keep WTS t=0) lives in the daily job, not the parser, so
// the parser stays byte-parity with the TS `parseToRows` output.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
)

// malformationTolerancePct mirrors the TS MALFORMATION_TOLERANCE_PCT: if more
// than 1% of rows are malformed, ParseToRows returns an error (matches the TS
// throw); at or below 1%, malformed rows are silently skipped (and logged).
const malformationTolerancePct = 0.01

// numericKeys mirrors the TS NUMERIC_KEYS minus i/t (handled separately). These
// are coerced to 0 when absent or non-numeric, exactly as coerceRow does. The
// 90-day (t90/a90) and today (tc/ta) keys ARE coerced internally for fidelity
// but are NOT surfaced on PigparseRow (the Sheet's buildRow never wrote them —
// RESEARCH §2 / A4; surfacing them would break Sheet parity).
var numericKeys = []string{
	"tc", "ta", "t30", "a30", "t60", "a60",
	"t90", "a90", "t6m", "a6m", "ty", "ay",
}

// PigparseRow is the validated, coerced shape ParseToRows emits. The json tags
// mirror PigparseRowRaw but surface ONLY the fields the Sheet persisted plus the
// direction (T) and identity (I/N/L). T90/A90/Tc/Ta are deliberately omitted
// (RESEARCH §2 / A4: the Sheet's buildRow never wrote them — surfacing them
// would break the D-7 byte-parity proof). T is the direction flag
// (0=WTS/sell, 1=WTB/buy, 2=BOTH); the job filters to T==0 before upsert (D-9).
type PigparseRow struct {
	I   int     `json:"i"`   // EQ item ID
	T   int     `json:"t"`   // direction: 0=WTS, 1=WTB, 2=BOTH
	N   string  `json:"n"`   // item name (trimmed)
	L   string  `json:"l"`   // ISO 8601 last seen
	T30 int     `json:"t30"` // 30-day volume (count)
	A30 float64 `json:"a30"` // 30-day average pp
	T60 int     `json:"t60"`
	A60 float64 `json:"a60"`
	T6m int     `json:"t6m"`
	A6m float64 `json:"a6m"`
	Ty  int     `json:"ty"` // year volume
	Ay  float64 `json:"ay"` // year average pp
}

// ParseToRows takes the raw PigParse getall response body, unmarshals it,
// validates each row's shape, coerces numerics, and returns the validated rows.
// It returns an error if:
//   - the body is not a JSON array (mirrors the TS `Array.isArray` guard), or
//   - more than 1% of rows are malformed (mirrors the TS throw).
//
// Up to 1% malformed rows are tolerated (silently skipped + logged), matching
// the TS behavior, because PigParse occasionally carries weird historical
// entries. The function is I/O-free; the only side effect is a single
// slog.Warn when rows are skipped (counts only, never raw content — V7).
func ParseToRows(body []byte) ([]PigparseRow, error) {
	// Unmarshal into raw per-row messages first so we can replicate the TS
	// `typeof` checks (a value can be present-but-wrong-type). A non-array
	// body fails here, mirroring the TS `Array.isArray` guard.
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pigparse: response is not a JSON array: %w", err)
	}

	accepted := make([]PigparseRow, 0, len(raw))
	skipped := 0
	for _, item := range raw {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(item, &obj); err != nil {
			// Not a JSON object → malformed (mirrors `typeof item !== 'object'`).
			skipped++
			continue
		}
		row, ok := coerceRow(obj)
		if !ok {
			skipped++
			continue
		}
		accepted = append(accepted, row)
	}

	if len(raw) > 0 && float64(skipped)/float64(len(raw)) > malformationTolerancePct {
		return nil, fmt.Errorf(
			"pigparse: too many malformed rows (%d/%d skipped, threshold %g%%)",
			skipped, len(raw), malformationTolerancePct*100)
	}
	if skipped > 0 {
		slog.Warn("parseToRows", "skipped", skipped, "total", len(raw))
	}
	return accepted, nil
}

// coerceRow validates + coerces one raw row, mirroring the TS isValidRow +
// coerceRow pair. Returns (row, false) when the row is malformed (missing
// required key, non-number i, t outside {0,1,2}, or empty/non-string n).
// Numeric fields absent or non-numeric coerce to 0 (TS coerceRow), and only
// finite numbers are kept (TS `Number.isFinite`).
func coerceRow(obj map[string]json.RawMessage) (PigparseRow, bool) {
	// REQUIRED_KEYS = i, t, n — all must be present.
	for _, k := range []string{"i", "t", "n"} {
		if _, present := obj[k]; !present {
			return PigparseRow{}, false
		}
	}

	// i must be a number.
	i, ok := jsonNumber(obj["i"])
	if !ok {
		return PigparseRow{}, false
	}
	// t must be exactly 0, 1, or 2.
	tf, ok := jsonNumber(obj["t"])
	if !ok {
		return PigparseRow{}, false
	}
	t := int(tf)
	if t != 0 && t != 1 && t != 2 {
		return PigparseRow{}, false
	}
	// n must be a non-empty string.
	n, ok := jsonString(obj["n"])
	if !ok || len(n) == 0 {
		return PigparseRow{}, false
	}

	row := PigparseRow{
		I: int(i),
		T: t,
		N: strings.TrimSpace(n),
	}
	// l: string if present and a string, else "" (TS coerceRow).
	if l, ok := jsonString(obj["l"]); ok {
		row.L = l
	}

	// Coerce every numeric key to 0 when absent/non-numeric/non-finite, then
	// project the Sheet-persisted subset onto the public struct.
	nums := make(map[string]float64, len(numericKeys))
	for _, k := range numericKeys {
		v := 0.0
		if rawV, present := obj[k]; present {
			if f, ok := jsonNumber(rawV); ok && !math.IsInf(f, 0) && !math.IsNaN(f) {
				v = f
			}
		}
		nums[k] = v
	}
	row.T30 = int(nums["t30"])
	row.A30 = nums["a30"]
	row.T60 = int(nums["t60"])
	row.A60 = nums["a60"]
	row.T6m = int(nums["t6m"])
	row.A6m = nums["a6m"]
	row.Ty = int(nums["ty"])
	row.Ay = nums["ay"]
	// t90/a90/tc/ta are coerced into nums above for fidelity but intentionally
	// dropped here (Sheet parity — buildRow never wrote them).

	return row, true
}

// jsonNumber reports whether the raw JSON value is a number, returning its
// float64 value. Mirrors the TS `typeof v === 'number'` check: a JSON string
// like "not-a-number" is NOT a number and yields ok=false.
func jsonNumber(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, false
	}
	return f, true
}

// jsonString reports whether the raw JSON value is a string, returning it.
// Mirrors the TS `typeof obj.n === 'string'` check.
func jsonString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}
