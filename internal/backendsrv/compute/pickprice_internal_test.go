package compute

// pickprice_internal_test.go is a white-box (package compute) unit test for the
// unexported pickPrice — the WTS-then-WTB-then-nil price pick ported from
// buildView.ts:259-265 with the Pitfall-6 TEXT-direction fix.

import "testing"

func TestPickPrice(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name   string
		prices []PriceDetail
		want   *float64
	}{
		{
			name:   "WTS present with a30>0 -> WTS a30",
			prices: []PriceDetail{{Direction: "0", A30: 4500, T30: 75}},
			want:   f(4500),
		},
		{
			name:   "WTB only with a30>0 -> WTB a30",
			prices: []PriceDetail{{Direction: "1", A30: 3000, T30: 12}},
			want:   f(3000),
		},
		{
			name: "both present -> WTS preferred over WTB",
			prices: []PriceDetail{
				{Direction: "1", A30: 3000, T30: 12},
				{Direction: "0", A30: 4500, T30: 75},
			},
			want: f(4500),
		},
		{
			name:   "WTS a30 == 0 falls through to nil (no WTB)",
			prices: []PriceDetail{{Direction: "0", A30: 0, T30: 0}},
			want:   nil,
		},
		{
			name:   "no price rows -> nil",
			prices: nil,
			want:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pickPrice(tc.prices)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got *%v, want nil", *got)
			case tc.want != nil && got == nil:
				t.Errorf("got nil, want *%v", *tc.want)
			case tc.want != nil && got != nil && *got != *tc.want:
				t.Errorf("got *%v, want *%v", *got, *tc.want)
			}
		})
	}
}
