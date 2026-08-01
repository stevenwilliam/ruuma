// Package money is integer rupiah arithmetic.
//
// BR-1.1.1: every amount is whole rupiah in an int64. The rupiah has no
// circulating subunit, so there is no minor-unit multiplier — 150000 means
// Rp 150.000.
//
// BR-1.1.2: floating point is prohibited in any code path touching money. This
// package contains no float type, by design: if it is not here, it cannot creep
// into a total.
package money

import "errors"

// Rupiah is a whole-rupiah amount (BR-1.1.1).
type Rupiah int64

// Bps is a rate in basis points: 1000 bps = 10% (BR-1.1.3).
type Bps int64

const (
	// FullBps is 100%.
	FullBps Bps = 10000
)

var (
	ErrNegative = errors.New("money: amount would be negative")
	ErrOverflow = errors.New("money: arithmetic overflow")
)

// Zero is the additive identity, spelled out so call sites read as intent.
const Zero Rupiah = 0

// Add returns a+b, refusing to silently overflow.
func Add(a, b Rupiah) (Rupiah, error) {
	sum := a + b
	if (b > 0 && sum < a) || (b < 0 && sum > a) {
		return 0, ErrOverflow
	}
	return sum, nil
}

// Sub returns a-b.
func Sub(a, b Rupiah) (Rupiah, error) {
	diff := a - b
	if (b < 0 && diff < a) || (b > 0 && diff > a) {
		return 0, ErrOverflow
	}
	return diff, nil
}

// Mul multiplies an amount by a whole quantity (BR-2.5.2).
func Mul(a Rupiah, qty int) (Rupiah, error) {
	if qty == 0 || a == 0 {
		return 0, nil
	}
	product := a * Rupiah(qty)
	if product/Rupiah(qty) != a {
		return 0, ErrOverflow
	}
	return product, nil
}

// ApplyRate returns amount × bps, rounded half-up to the nearest whole rupiah
// (BR-1.1.3):
//
//	round(amount * bps / 10000) = floor((amount * bps + 5000) / 10000)
//
// Negative amounts round away from zero symmetrically, so a refund line and its
// original charge always agree to the rupiah.
func ApplyRate(amount Rupiah, bps Bps) Rupiah {
	if amount == 0 || bps == 0 {
		return 0
	}
	negative := amount < 0
	if negative {
		amount = -amount
	}
	// +5000 before the integer division is the half-up rounding step.
	result := (int64(amount)*int64(bps) + 5000) / 10000
	if negative {
		return Rupiah(-result)
	}
	return Rupiah(result)
}

// Sum adds a list of amounts (BR-2.5.3).
func Sum(amounts ...Rupiah) (Rupiah, error) {
	var total Rupiah
	for _, a := range amounts {
		next, err := Add(total, a)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}

// ClampNonNegative floors an amount at zero. Used where a rule says a value can
// never go below zero — a discount larger than the subtotal, for instance
// (BR-2.5.4, BR-2.5.10).
func ClampNonNegative(a Rupiah) Rupiah {
	if a < 0 {
		return 0
	}
	return a
}

// Min returns the smaller of two amounts, used to cap discounts (BR-2.5.10).
func Min(a, b Rupiah) Rupiah {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of two amounts.
func Max(a, b Rupiah) Rupiah {
	if a > b {
		return a
	}
	return b
}

// PercentToBps converts a whole percentage to basis points, for admin input.
func PercentToBps(percent int) Bps { return Bps(percent) * 100 }
