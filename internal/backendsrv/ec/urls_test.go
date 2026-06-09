package ec

import (
	"strings"
	"testing"
)

// TestGetDetailsBase_LiveBlueOnly pins the EC-tunnel auction monitor's getdetails
// feed (the path that polls PigParse for each wantlisted item) to server=1 — the
// LIVE P99 Blue tunnel (matches the catalog convention getall/1 = Blue). This is a
// deliberate INVARIANT: the wantlist monitor must poll ONLY Blue, NEVER Green.
//
// CORRECTED 2026-06-09: the Phase 21 spike originally pinned server=0 as "live
// Blue", but that was DISPROVEN — server=0 is GREEN (a Blue guildie got a real
// false-ping for a Green seller; a live probe showed server=1 is the fresher live
// Blue feed). PigParse server numbering for getdetails: 1 = live Blue, 0 = Green.
// If this base ever changes its server segment, this test FAILS on purpose — you
// have changed WHICH P99 server the wantlist monitor polls. Update this test ONLY
// if that widening/switch was intentional.
func TestGetDetailsBase_LiveBlueOnly(t *testing.T) {
	const wantBlue = "https://pigparse.azurewebsites.net/api/item/getdetails/1/"
	if getDetailsBase != wantBlue {
		t.Fatalf("getDetailsBase = %q, want the live-Blue (server=1) getdetails endpoint %q.\n"+
			"The wantlist EC monitor must poll ONLY P99 Blue (getdetails server 1=live Blue; 0=Green). A change "+
			"here changes which P99 server is polled — update this test ONLY if that was intended.",
			getDetailsBase, wantBlue)
	}

	// Belt-and-braces: a built per-item URL must still carry the /getdetails/1/
	// (Blue) segment — guards against the segment being moved out of the base.
	if got := getDetailsURL("Fungi Tunic"); !strings.Contains(got, "/api/item/getdetails/1/") {
		t.Fatalf("getDetailsURL built %q, which is not the live-Blue (/getdetails/1/) endpoint", got)
	}
}
