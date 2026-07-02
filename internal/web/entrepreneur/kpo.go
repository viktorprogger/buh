package entrepreneur

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/kpo"
	"buh/internal/web/shared"
)

func (h *Handler) eKPOBookFromPath(r *http.Request) (uuid.UUID, kpo.Book, int, error) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		return uuid.Nil, kpo.Book{}, 0, errors.New("forbidden")
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		return uuid.Nil, kpo.Book{}, 0, errors.New("bad year")
	}
	book, err := h.kpoBooks.FindOrCreateForEntrepreneur(r.Context(), userID, year)
	return userID, book, year, err
}

func (h *Handler) handleKPO(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	selectedYear := time.Now().Year()
	if ys := r.URL.Query().Get("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}
	if ys := r.PathValue("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}

	currentBook, err := h.kpoBooks.FindOrCreateForEntrepreneur(r.Context(), userID, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}
	entries, err := h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
		return
	}
	books, err := h.kpoBooks.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}
	advanceInvoices, err := h.invoices.ListAdvanceByEntrepreneurUserYear(r.Context(), userID, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању авансних фактура", http.StatusInternalServerError)
		return
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

	var totalProduct, totalService float64
	for _, en := range entries {
		totalProduct += en.ProductRevenue
		totalService += en.ServiceRevenue
	}

	shared.RenderTemplate(w, h.tmpl.EntrepreneurKPO, map[string]any{
		"CurrentBook":  currentBook,
		"KPOBooks":     books,
		"KPOEntries":   entries,
		"KPORows":      kpoRows,
		"SelectedYear": selectedYear,
		"TotalProduct": totalProduct,
		"TotalService": totalService,
		"TotalAll":     totalProduct + totalService,
	})
}

func (h *Handler) handleKPOAddEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	r.ParseForm()
	date, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум", http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if _, err := h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, "Грешка при уносу", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOUpdateEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	r.ParseForm()
	date, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум", http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if err := h.kpoBooks.UpdateEntry(r.Context(), kpo.Entry{
		ID:             entryID,
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, "Грешка при чувању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOReorderEntries(w http.ResponseWriter, r *http.Request) {
	_, book, _, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	r.ParseForm()
	rawIDs := r.Form["ids[]"]
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, s := range rawIDs {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	if err := h.kpoBooks.ReorderEntries(r.Context(), book.ID, ids); err != nil {
		http.Error(w, "Грешка при промени редоследа", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleKPODeleteEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.DeleteEntry(r.Context(), book.ID, entryID); err != nil {
		http.Error(w, "Грешка при брисању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOFinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Finalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при финализацији", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOUnfinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Unfinalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при поништавању финализације", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

