package enrich

// PigParse getdetails parser (Phase 21, WANT-05). This is the per-item auction
// feed parser — DISTINCT from ParseToRows (the getall aggregate). Do NOT reuse
// ParseToRows for this shape.
//
// ⚠ THE `t` COLLISION. In the getall feed (ParseToRows / PigparseRow) `t` is a
// DIRECTION flag (0=WTS, 1=WTB, 2=BOTH). In THIS getdetails feed `t` is a
// TIMESTAMP (an ISO-8601 / RFC3339 date-time string, the per-item diff cursor
// key) and `u` is the direction. Confusing the two yields a parser that diffs on
// direction or matches WTS on the wrong field. Filter WTS on `u ∈ {0,2}` and diff
// on `t`.
//
// Endpoint (the spike pinned this — see 21-SPIKE.md):
//
//	GET https://pigparse.azurewebsites.net/api/item/getdetails/{server}/{itemname}
//	    {server}=0 is the LIVE Blue tunnel (NOT 1 — 1 is stale); the NAME form is
//	    the only working lookup key (the id form returns 400/empty).
//
// Returns ItemDetail = { items: ItemAuctionDetail[] (nullable→empty), itemName,
// players (seller best-effort, key→auction relationship undocumented — D-05) }.
//
// Pure: NO net/http and NO database/sql imports. Returns all valid records in
// source order. Mirrors pigparse.go's malformation discipline: > 1% malformed
// records → error; ≤ 1% → skip + a single slog.Warn (counts only, never raw
// record content or the players map — V7, no PII).

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

// ItemAuctionDetail is one auction listing from the getdetails feed. U is the
// direction (0=WTS, 1=WTB, 2=BOTH — NOT the getall `t`); I is an auction-instance
// id (it varies record-to-record for the same item — it is NOT the EQ catalog
// item id and NOT a query key); P is the price in pp (NULLABLE — nil ⇒ "price
// unknown", never render 0pp); T is the auction timestamp string (RFC3339-ish,
// always +00:00 offset, variable fractional precision — the DIFF cursor key,
// kept as a raw string for lexical comparison).
type ItemAuctionDetail struct {
	U int    `json:"u"` // direction: 0=WTS, 1=WTB, 2=BOTH (NOT the getall `t`)
	I int    `json:"i"` // auction-instance id (NOT the EQ item id, NOT a query key)
	P *int   `json:"p"` // price pp — NULLABLE; nil ⇒ "price unknown"/omit (never 0pp)
	T string `json:"t"` // auction timestamp (RFC3339); the DIFF cursor key
}

// ItemDetail is the getdetails response for one item. Items is nullable in the
// API and is normalized to a non-nil empty slice. Players maps a stringified
// key → player name (seller best-effort — D-05; the key→specific-auction-record
// relationship is undocumented). ItemName is the canonical item name.
type ItemDetail struct {
	Items    []ItemAuctionDetail `json:"items"`    // nullable in the API → empty slice
	ItemName string              `json:"itemName"` // canonical item name
	Players  map[string]string   `json:"players"`  // seller best-effort (D-05)
}

// ParseItemDetail unmarshals one getdetails response body into an ItemDetail. It
// returns an error if:
//   - the body is not a JSON OBJECT (a top-level array or garbage fails here), or
//   - more than 1% of the items records are malformed.
//
// Up to 1% malformed records are tolerated (skipped + a single slog.Warn with
// counts only — never raw content, V7), matching ParseToRows. A nil/absent items
// array normalizes to a non-nil empty slice (the API returns items:null for an
// unseen item). The function is I/O-free; the only side effect is the skip Warn.
func ParseItemDetail(body []byte) (ItemDetail, error) {
	// Decode the top-level OBJECT, but capture items as raw messages so we can
	// validate/skip per-record (mirroring ParseToRows' per-row tolerance). A
	// top-level array or non-object body fails the object Unmarshal here.
	var envelope struct {
		Items    []json.RawMessage `json:"items"`
		ItemName string            `json:"itemName"`
		Players  map[string]string `json:"players"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ItemDetail{}, fmt.Errorf("pigdetails: response is not a JSON object: %w", err)
	}

	out := ItemDetail{
		// Non-nil empty slice even when items is null/absent (callers iterate cleanly).
		Items:    make([]ItemAuctionDetail, 0, len(envelope.Items)),
		ItemName: envelope.ItemName,
		Players:  envelope.Players,
	}

	skipped := 0
	for _, raw := range envelope.Items {
		rec, ok := coerceAuctionDetail(raw)
		if !ok {
			skipped++
			continue
		}
		out.Items = append(out.Items, rec)
	}

	if len(envelope.Items) > 0 && float64(skipped)/float64(len(envelope.Items)) > malformationTolerancePct {
		return ItemDetail{}, fmt.Errorf(
			"pigdetails: too many malformed records (%d/%d skipped, threshold %g%%)",
			skipped, len(envelope.Items), malformationTolerancePct*100)
	}
	if skipped > 0 {
		slog.Warn("parseItemDetail", "skipped", skipped, "total", len(envelope.Items))
	}
	return out, nil
}

// coerceAuctionDetail validates one raw record. A record is malformed (ok=false)
// when it is not a JSON object, when `t` is missing/empty/non-string, or when
// `u`/`i` are non-numeric. `p` is genuinely nullable: a JSON null OR an absent
// key yields P==nil (NOT 0) — only a present numeric `p` sets the int.
func coerceAuctionDetail(raw json.RawMessage) (ItemAuctionDetail, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ItemAuctionDetail{}, false // not a JSON object ⇒ malformed
	}

	// t must be a non-empty string (the cursor key). Reject a numeric/absent t —
	// a number here would be the getall `t`-as-direction confusion (the collision).
	tStr, ok := jsonString(obj["t"])
	if !ok || len(tStr) == 0 {
		return ItemAuctionDetail{}, false
	}

	// u and i must be numbers.
	uf, ok := jsonNumber(obj["u"])
	if !ok {
		return ItemAuctionDetail{}, false
	}
	idf, ok := jsonNumber(obj["i"])
	if !ok {
		return ItemAuctionDetail{}, false
	}

	rec := ItemAuctionDetail{
		U: int(uf),
		I: int(idf),
		T: tStr,
	}

	// p is NULLABLE: present-and-numeric ⇒ *int; null/absent/non-numeric ⇒ nil
	// (never coerce to 0 — a null price must render "price unknown", not 0pp).
	// NOTE: json.Unmarshal of the literal `null` into a float64 succeeds with a
	// zero value, so guard the explicit null BEFORE jsonNumber, else null→0pp.
	if rawP, present := obj["p"]; present && string(rawP) != "null" {
		if pf, ok := jsonNumber(rawP); ok {
			p := int(pf)
			rec.P = &p
		}
	}

	return rec, true
}
