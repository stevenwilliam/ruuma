package money

import (
	"math"
	"testing"
)

// TestApplyRate_BR_1_1_3 pins the half-up rounding rule
// floor((amount*bps + 5000)/10000) — the single place tax and service charge
// can disagree with the accountant.
func TestApplyRate_BR_1_1_3(t *testing.T) {
	cases := []struct {
		name   string
		amount Rupiah
		bps    Bps
		want   Rupiah
	}{
		{"zero amount", 0, 1000, 0},
		{"zero rate", 150000, 0, 0},
		{"PB1 10% of 150.000", 150000, 1000, 15000},
		{"PB1 10% of 1", 1, 1000, 0},      // 0.1 → 0
		{"PB1 10% of 5", 5, 1000, 1},      // 0.5 → 1 (half-up)
		{"PB1 10% of 4", 4, 1000, 0},      // 0.4 → 0
		{"PB1 10% of 15", 15, 1000, 2},    // 1.5 → 2 (half-up)
		{"PB1 10% of 25", 25, 1000, 3},    // 2.5 → 3 (half-up)
		{"5% of 33333", 33333, 500, 1667}, // 1666.65 → 1667
		{"exact .5 boundary", 10, 500, 1}, // 0.5 → 1
		{"just below .5", 9, 500, 0},      // 0.45 → 0
		{"just above .5", 11, 500, 1},     // 0.55 → 1
		{"100%", 87654, 10000, 87654},
		{"negative symmetric", -5, 1000, -1},
		{"negative large", -150000, 1000, -15000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ApplyRate(tc.amount, tc.bps); got != tc.want {
				t.Fatalf("ApplyRate(%d, %d) = %d, want %d", tc.amount, tc.bps, got, tc.want)
			}
		})
	}
}

// TestApplyRate_MatchesFloatFreeFormula_BR_1_1_2 walks a wide range and asserts
// the result equals the documented integer formula exactly — no float is used
// or trusted anywhere.
func TestApplyRate_MatchesFloatFreeFormula_BR_1_1_2(t *testing.T) {
	for amount := int64(0); amount <= 200000; amount += 997 {
		for _, bps := range []int64{0, 100, 500, 1000, 1100, 10000} {
			want := (amount*bps + 5000) / 10000
			if got := ApplyRate(Rupiah(amount), Bps(bps)); int64(got) != want {
				t.Fatalf("ApplyRate(%d,%d)=%d want %d", amount, bps, got, want)
			}
		}
	}
}

func TestMul_BR_2_5_2(t *testing.T) {
	cases := []struct {
		unit Rupiah
		qty  int
		want Rupiah
	}{
		{0, 5, 0},
		{65000, 0, 0},
		{65000, 1, 65000},
		{65000, 3, 195000},
		{75000, 2, 150000}, // (item 65000 + options 10000) × 2
	}
	for _, tc := range cases {
		got, err := Mul(tc.unit, tc.qty)
		if err != nil {
			t.Fatalf("Mul(%d,%d) unexpected error: %v", tc.unit, tc.qty, err)
		}
		if got != tc.want {
			t.Fatalf("Mul(%d,%d) = %d, want %d", tc.unit, tc.qty, got, tc.want)
		}
	}
}

func TestOverflowIsRefused_BR_1_1_2(t *testing.T) {
	if _, err := Add(Rupiah(math.MaxInt64), 1); err != ErrOverflow {
		t.Fatalf("Add overflow: got %v, want ErrOverflow", err)
	}
	if _, err := Sub(Rupiah(math.MinInt64), 1); err != ErrOverflow {
		t.Fatalf("Sub overflow: got %v, want ErrOverflow", err)
	}
	if _, err := Mul(Rupiah(math.MaxInt64/2), 3); err != ErrOverflow {
		t.Fatalf("Mul overflow: got %v, want ErrOverflow", err)
	}
}

func TestSum_BR_2_5_3(t *testing.T) {
	got, err := Sum(150000, 45000, 12500)
	if err != nil {
		t.Fatalf("Sum: %v", err)
	}
	if got != 207500 {
		t.Fatalf("Sum = %d, want 207500", got)
	}
	empty, err := Sum()
	if err != nil || empty != 0 {
		t.Fatalf("Sum() = %d, %v; want 0, nil", empty, err)
	}
}

func TestClampAndCaps_BR_2_5_4_BR_2_5_10(t *testing.T) {
	if got := ClampNonNegative(-1); got != 0 {
		t.Fatalf("ClampNonNegative(-1) = %d, want 0", got)
	}
	if got := ClampNonNegative(500); got != 500 {
		t.Fatalf("ClampNonNegative(500) = %d, want 500", got)
	}
	// A fixed discount is capped at the eligible subtotal so a total can never
	// go negative (BR-2.5.10).
	if got := Min(200000, 150000); got != 150000 {
		t.Fatalf("Min = %d, want 150000", got)
	}
}

func TestPercentToBps(t *testing.T) {
	if got := PercentToBps(10); got != 1000 {
		t.Fatalf("PercentToBps(10) = %d, want 1000", got)
	}
	if got := PercentToBps(100); got != FullBps {
		t.Fatalf("PercentToBps(100) = %d, want %d", got, FullBps)
	}
}
