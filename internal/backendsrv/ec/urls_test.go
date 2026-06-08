package ec

import (
	"strings"
	"testing"
)

// TestGetDetailsBase_LiveBlueOnly pins the EC-tunnel auction monitor's getdetails
// feed (the path that polls PigParse for each wantlisted item) to server=0 — the
// LIVE P99 Blue tunnel. 21-SPIKE.md found server=1 here is a ~11h-stale Blue
// snapshot, so 0 was chosen for freshness. This is a deliberate INVARIANT: the
// wantlist monitor must poll ONLY Blue, NEVER Green.
//
// PigParse server numbering for getdetails: 0 = live Blue, 1 = stale Blue. If this
// base ever changes its server segment, this test FAILS on purpose — you have
// changed WHICH P99 server the wantlist monitor polls. Update this test ONLY if
// that widening/switch was intentional.
func TestGetDetailsBase_LiveBlueOnly(t *testing.T) {
	const wantBlue = "https://pigparse.azurewebsites.net/api/item/getdetails/0/"
	if getDetailsBase != wantBlue {
		t.Fatalf("getDetailsBase = %q, want the live-Blue (server=0) getdetails endpoint %q.\n"+
			"The wantlist EC monitor must poll ONLY P99 Blue (getdetails server 0=live Blue). A change "+
			"here changes which P99 server is polled — update this test ONLY if that was intended.",
			getDetailsBase, wantBlue)
	}

	// Belt-and-braces: a built per-item URL must still carry the /getdetails/0/
	// (Blue) segment — guards against the segment being moved out of the base.
	if got := getDetailsURL("Fungi Tunic"); !strings.Contains(got, "/api/item/getdetails/0/") {
		t.Fatalf("getDetailsURL built %q, which is not the live-Blue (/getdetails/0/) endpoint", got)
	}
}
