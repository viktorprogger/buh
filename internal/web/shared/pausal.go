package shared

import (
	"math"
	"time"
)

// PausalAlert holds the computed milestone alert for a paušal entrepreneur.
type PausalAlert struct {
	Severity  string  // "minor", "high", "alert"
	Milestone int     // percentage threshold that triggered this (50, 70, 75, 80, 90)
	TotalFmt  string  // formatted current total
	LimitFmt  string  // formatted limit
	Percent   float64 // actual percentage
}

// ComputePausalAlert returns the most severe applicable milestone alert for the given
// total and year, or nil if no threshold is reached.
// Conditional milestones (50%, 70%) only fire when less than that percent of the year has passed.
func ComputePausalAlert(total float64, limit int64, now time.Time) *PausalAlert {
	if limit <= 0 {
		return nil
	}
	pct := total / float64(limit) * 100
	daysInYear := 365.0
	if IsLeapYear(now.Year()) {
		daysInYear = 366
	}
	yearPct := float64(now.YearDay()) / daysInYear * 100

	var severity string
	var milestone int
	switch {
	case pct >= 100:
		severity, milestone = "exceeded", 100
	case pct >= 90:
		severity, milestone = "alert", 90
	case pct >= 80:
		severity, milestone = "high", 80
	case pct >= 75:
		severity, milestone = "minor", 75
	case pct >= 70 && yearPct < 70:
		severity, milestone = "minor", 70
	case pct >= 50 && yearPct < 50:
		severity, milestone = "minor", 50
	default:
		return nil
	}
	return &PausalAlert{
		Severity:  severity,
		Milestone: milestone,
		TotalFmt:  FormatIntWithSpaces(int64(math.Round(total))),
		LimitFmt:  FormatIntWithSpaces(limit),
		Percent:   pct,
	}
}

func IsLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

// ComputeVATAlert returns a milestone alert for the VAT registration threshold.
// The rolling window has no time-based conditions, so all milestones fire on percentage alone.
func ComputeVATAlert(total float64, limit int64) *PausalAlert {
	if limit <= 0 {
		return nil
	}
	pct := total / float64(limit) * 100
	var severity string
	var milestone int
	switch {
	case pct >= 100:
		severity, milestone = "exceeded", 100
	case pct >= 90:
		severity, milestone = "alert", 90
	case pct >= 80:
		severity, milestone = "high", 80
	case pct >= 75:
		severity, milestone = "minor", 75
	case pct >= 70:
		severity, milestone = "minor", 70
	case pct >= 50:
		severity, milestone = "minor", 50
	default:
		return nil
	}
	return &PausalAlert{
		Severity:  severity,
		Milestone: milestone,
		TotalFmt:  FormatIntWithSpaces(int64(math.Round(total))),
		LimitFmt:  FormatIntWithSpaces(limit),
		Percent:   pct,
	}
}
