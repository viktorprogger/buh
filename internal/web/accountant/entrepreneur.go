package accountant

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/web/shared"
)

func (h *Handler) findOwnedEntrepreneur(w http.ResponseWriter, r *http.Request, id uuid.UUID) (entrepreneur.Entrepreneur, bool) {
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		h.renderError(w, http.StatusForbidden)
		return entrepreneur.Entrepreneur{}, false
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		h.renderError(w, http.StatusNotFound)
		return entrepreneur.Entrepreneur{}, false
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return entrepreneur.Entrepreneur{}, false
	}
	if e.AccountantID != accountantID {
		h.renderError(w, http.StatusForbidden)
		return entrepreneur.Entrepreneur{}, false
	}
	return e, true
}

func (h *Handler) handleAccountantIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.renderError(w, http.StatusNotFound)
		return
	}
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	entrepreneurs, err := h.entrepreneurs.ListByAccountant(context.Background(), accountantID)
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return
	}
	shared.RenderTemplate(w, h.tmpl.Index, map[string]any{
		"Entrepreneurs": entrepreneurs,
	})
}

func (h *Handler) handleEntrepreneur(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}

	selectedYear := time.Now().Year()
	if ys := r.URL.Query().Get("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}

	currentBook, err := h.kpoBooks.FindByYear(r.Context(), id, selectedYear)
	if err != nil && !errors.Is(err, kpo.ErrNotFound) {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	var entries []kpo.Entry
	if currentBook.ID != (uuid.UUID{}) {
		entries, err = h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
		if err != nil {
			http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
			return
		}
	}

	books, err := h.kpoBooks.ListByManagedEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}
	// Always show at least the selected year in the year select, even before the book is created.
	hasSelectedYear := false
	for _, b := range books {
		if b.Year == selectedYear {
			hasSelectedYear = true
			break
		}
	}
	if !hasSelectedYear {
		books = append([]kpo.Book{{Year: selectedYear}}, books...)
	}

	var advanceInvoices []invoice.Invoice
	if e.EntrepreneurUserID != nil {
		advanceInvoices, err = h.invoices.ListAdvanceByEntrepreneurUserYear(r.Context(), *e.EntrepreneurUserID, selectedYear)
		if err != nil {
			http.Error(w, "Грешка при учитавању авансних фактура", http.StatusInternalServerError)
			return
		}
	}

	var kpoRows []shared.KPORow
	for i, en := range entries {
		kpoRows = append(kpoRows, shared.KPORow{
			IsAdvanceInvoice: false,
			Date:             en.CollectionDate,
			InvoiceNum:       en.InvoiceNumber,
			EntryID:          en.ID,
			OrdinalNumber:    i + 1,
			ProductRevenue:   en.ProductRevenue,
			ServiceRevenue:   en.ServiceRevenue,
			EntryTotal:       en.Total(),
		})
	}
	for _, inv := range advanceInvoices {
		kpoRows = append(kpoRows, shared.KPORow{
			IsAdvanceInvoice: true,
			Date:             inv.IssueDate,
			InvoiceNum:       inv.InvoiceNumber,
			InvoiceID:        inv.ID,
			ClientName:       inv.ClientName,
			TotalRSD:         inv.TotalRSD,
		})
	}
	sort.Slice(kpoRows, func(i, j int) bool {
		return kpoRows[i].Date.Before(kpoRows[j].Date)
	})

	slips, err := h.slips.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатница", http.StatusInternalServerError)
		return
	}
	latestSlipYear, prevSlipYears := shared.GroupSlipsByYear(slips)

	var totalProduct, totalService float64
	for _, en := range entries {
		totalProduct += en.ProductRevenue
		totalService += en.ServiceRevenue
	}

	currentYear := time.Now().Year()
	pausalalLimit := shared.PausalalLimitForYear(currentYear)
	var pausalalTotal float64
	var pausalalHasData bool
	if selectedYear == currentYear {
		pausalalTotal = totalProduct + totalService
		pausalalHasData = len(entries) > 0
	} else {
		pausalalTotal, pausalalHasData, err = h.kpoBooks.SumForYear(r.Context(), id, currentYear)
		if err != nil {
			http.Error(w, "Грешка при учитавању паушалног прага", http.StatusInternalServerError)
			return
		}
	}
	var pausalalPercent float64
	if pausalalLimit > 0 {
		pausalalPercent = pausalalTotal / float64(pausalalLimit) * 100
	}

	shared.RenderTemplate(w, h.tmpl.Entrepreneur, map[string]any{
		"Entrepreneur":     e,
		"KPOBooks":         books,
		"CurrentBook":      currentBook,
		"KPOEntries":       entries,
		"KPORows":          kpoRows,
		"SelectedYear":     selectedYear,
		"TotalProduct":     totalProduct,
		"TotalService":     totalService,
		"TotalAll":         totalProduct + totalService,
		"LatestSlipYear":   latestSlipYear,
		"PrevSlipYears":    prevSlipYears,
		"PausalalYear":     currentYear,
		"PausalalHasData":  pausalalHasData,
		"PausalalTotalFmt": shared.FormatIntWithSpaces(int64(math.Round(pausalalTotal))),
		"PausalalLimitFmt": shared.FormatIntWithSpaces(pausalalLimit),
		"PausalalPercent":  pausalalPercent,
	})
}

func (h *Handler) handleEntrepreneurNewForm(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, h.tmpl.EntrepreneurNew, nil)
}

func (h *Handler) handleEntrepreneurNewSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	pib := strings.TrimSpace(r.FormValue("pib"))

	if name == "" || pib == "" {
		shared.RenderTemplate(w, h.tmpl.EntrepreneurNew, map[string]any{
			"Error": "Оба поља су обавезна.",
			"Name":  name,
			"PIB":   pib,
		})
		return
	}

	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	e, _, err := h.entrepreneurs.FindOrCreate(context.Background(), accountantID, pib, name)
	if err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String(), http.StatusFound)
}

func (h *Handler) handleEntrepreneurUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	pib := strings.TrimSpace(r.FormValue("pib"))
	title := strings.TrimSpace(r.FormValue("title"))

	if name == "" || pib == "" {
		http.Redirect(w, r, "/a/entrepreneurs/"+idStr, http.StatusFound)
		return
	}
	e.Name = name
	e.PIB = pib
	e.Title = title
	e.Address = strings.TrimSpace(r.FormValue("address"))
	e.BankAccount = strings.TrimSpace(r.FormValue("bank_account"))
	e.MB = strings.TrimSpace(r.FormValue("mb"))
	if err := h.entrepreneurs.Update(r.Context(), e); err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+idStr, http.StatusFound)
}

func (h *Handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["pdfs"]
	if len(files) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	const maxUploadFiles = 4
	if len(files) > maxUploadFiles {
		files = files[:maxUploadFiles]
	}
	accountantID, _ := h.accountantFromSession(r)
	result := h.importer.ProcessFiles(r.Context(), accountantID, files)
	shared.RenderTemplate(w, h.tmpl.Results, result)
}
