// Package clock injects time so cutoff, lead-time and expiry rules are testable
// to the minute. Nothing in internal/domain calls time.Now directly
// (docs/05 §3.3).
package clock

import "time"

// Clock reports the current instant.
type Clock interface {
	Now() time.Time
}

// System is the production clock.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }

// Fixed is a frozen clock for tests.
type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T.UTC() }

// NewFixed builds a frozen clock from an RFC3339 string, panicking on a bad
// literal — acceptable because it is only ever called from test setup.
func NewFixed(rfc3339 string) Fixed {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		panic("clock: bad fixed time: " + rfc3339)
	}
	return Fixed{T: t}
}

// Jakarta is the business timezone for every store by default (BR-1.3.1).
// It is loaded once; a missing tzdata is a deployment error, not a runtime one.
var Jakarta = mustLoad("Asia/Jakarta")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("clock: cannot load timezone " + name + ": " + err.Error())
	}
	return loc
}

// Location resolves a store's timezone name, falling back to Jakarta when the
// name is empty or unknown rather than failing a customer's request.
func Location(name string) *time.Location {
	if name == "" {
		return Jakarta
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return Jakarta
	}
	return loc
}

// BusinessDate returns the calendar date of an instant in a store's timezone
// (BR-1.3.3).
func BusinessDate(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}
