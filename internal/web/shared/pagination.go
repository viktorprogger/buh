package shared

import (
	"math"
	"net/http"
	"strconv"
)

const PerPage = 10

// ListState carries the current sort column, direction, and page for a paginated list.
type ListState struct {
	SortCol  string
	SortDir  string
	Page     int
	LastPage int
	Total    int
	BaseURL  string
}

// ParseListParams extracts sort and page query params, falling back to defaults.
func ParseListParams(r *http.Request, defaultSort, defaultDir string) (col, dir string, page int) {
	col = r.URL.Query().Get("sort")
	if col == "" {
		col = defaultSort
	}
	dir = r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = defaultDir
	}
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	return
}

// NewListState builds a ListState after sorting, before slicing.
func NewListState(col, dir string, page, total int, baseURL string) ListState {
	lastPage := int(math.Ceil(float64(total) / PerPage))
	if lastPage < 1 {
		lastPage = 1
	}
	if page > lastPage {
		page = lastPage
	}
	return ListState{
		SortCol:  col,
		SortDir:  dir,
		Page:     page,
		LastPage: lastPage,
		Total:    total,
		BaseURL:  baseURL,
	}
}

// PageSlice returns the subset of items for the current page.
func PageSlice[T any](items []T, page int) []T {
	total := len(items)
	start := (page - 1) * PerPage
	if start >= total {
		return nil
	}
	end := start + PerPage
	if end > total {
		end = total
	}
	return items[start:end]
}

func (s ListState) HasPrev() bool { return s.Page > 1 }
func (s ListState) HasNext() bool { return s.Page < s.LastPage }

func (s ListState) PrevURL() string { return s.urlFor(s.SortCol, s.SortDir, s.Page-1) }
func (s ListState) NextURL() string { return s.urlFor(s.SortCol, s.SortDir, s.Page+1) }

// SortLink returns the URL for sorting by col, defaulting to asc on first click.
func (s ListState) SortLink(col string) string {
	dir := "asc"
	if s.SortCol == col && s.SortDir == "asc" {
		dir = "desc"
	}
	return s.urlFor(col, dir, 1)
}

// SortLinkDesc returns the URL for sorting by col, defaulting to desc on first click.
func (s ListState) SortLinkDesc(col string) string {
	dir := "desc"
	if s.SortCol == col && s.SortDir == "desc" {
		dir = "asc"
	}
	return s.urlFor(col, dir, 1)
}

// SortIcon returns a Unicode indicator for the column's sort state.
func (s ListState) SortIcon(col string) string {
	if s.SortCol != col {
		return "⇅"
	}
	if s.SortDir == "asc" {
		return "↑"
	}
	return "↓"
}

func (s ListState) urlFor(col, dir string, page int) string {
	u := s.BaseURL + "?sort=" + col + "&dir=" + dir
	if page > 1 {
		u += "&page=" + strconv.Itoa(page)
	}
	return u
}

// LessStr compares two strings respecting sort direction.
func LessStr(a, b, dir string) bool {
	if dir == "desc" {
		return a > b
	}
	return a < b
}

// LessFloat compares two float64 values respecting sort direction.
func LessFloat(a, b float64, dir string) bool {
	if dir == "desc" {
		return a > b
	}
	return a < b
}

// LessBool compares two bool values (true > false) respecting sort direction.
func LessBool(a, b bool, dir string) bool {
	if a == b {
		return false
	}
	if dir == "desc" {
		return a
	}
	return b
}
