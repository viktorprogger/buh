package shared

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"buh/internal/sliprecord"
)

// KPORow is a display-model row for the KPO table: either a regular KPO entry
// or an advance invoice (shown muted, not counted in totals).
type KPORow struct {
	IsAdvanceInvoice bool
	WarnOutOfOrder   bool // true on the first entry whose date precedes the preceding entry's date (ordinal order)
	Date             time.Time
	InvoiceNum       string
	// Regular KPO entry fields:
	EntryID        uuid.UUID
	OrdinalNumber  int
	Description    string
	ProductRevenue float64
	ServiceRevenue float64
	EntryTotal     float64
	// Advance invoice fields:
	InvoiceID  uuid.UUID
	ClientName string
	TotalRSD   float64
}

// MarkFirstOutOfOrder sets WarnOutOfOrder=true on the first non-advance entry
// whose date is strictly before the preceding non-advance entry's date.
// Must be called before any re-sorting, while rows are still in ordinal order.
func MarkFirstOutOfOrder(rows []KPORow) {
	var prevDate time.Time
	var hasPrev bool
	for i := range rows {
		if rows[i].IsAdvanceInvoice {
			continue
		}
		if hasPrev && rows[i].Date.Before(prevDate) {
			rows[i].WarnOutOfOrder = true
			return
		}
		prevDate = rows[i].Date
		hasPrev = true
	}
}

type SlipYearGroup struct {
	Year    int
	Regular []sliprecord.SlipRecord
	Advance []sliprecord.SlipRecord
}

// SlipTableData is the dot passed into the "slip_table" sub-template.
type SlipTableData struct {
	EntrepreneurID uuid.UUID
	Slips          []sliprecord.SlipRecord
}

type SlipNewForm struct {
	S        string // purpose
	N        string // payee name
	R        string // payee account
	RO       string // reference
	SF       string // payment code
	Amount   string
	Currency string
	P        string // payer name
	Year     int
	Advance  bool
}

func GroupSlipsByYear(slips []sliprecord.SlipRecord) (latest SlipYearGroup, previous []SlipYearGroup) {
	yearMap := make(map[int]*SlipYearGroup)
	for _, s := range slips {
		g, ok := yearMap[s.Year]
		if !ok {
			g = &SlipYearGroup{Year: s.Year}
			yearMap[s.Year] = g
		}
		if s.Advance {
			g.Advance = append(g.Advance, s)
		} else {
			g.Regular = append(g.Regular, s)
		}
	}

	latestYear := 0
	for y, g := range yearMap {
		if len(g.Regular) > 0 && y > latestYear {
			latestYear = y
		}
	}
	if latestYear == 0 {
		for y := range yearMap {
			if y > latestYear {
				latestYear = y
			}
		}
	}

	years := make([]int, 0, len(yearMap))
	for y := range yearMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	for _, y := range years {
		if y == latestYear {
			latest = *yearMap[y]
		} else {
			previous = append(previous, *yearMap[y])
		}
	}
	return
}
