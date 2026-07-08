package entrepreneur

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/i18n"
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
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	entries, err := h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	books, err := h.kpoBooks.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	advanceInvoices, err := h.invoices.ListAdvanceByEntrepreneurUserYear(r.Context(), userID, selectedYear)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	var kpoRows []shared.KPORow
	for i, en := range entries {
		kpoRows = append(kpoRows, shared.KPORow{
			IsAdvanceInvoice: false,
			Date:             en.CollectionDate,
			InvoiceNum:       en.InvoiceNumber,
			Description:      en.Description,
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
	shared.MarkFirstOutOfOrder(kpoRows)
	sort.Slice(kpoRows, func(i, j int) bool {
		return kpoRows[i].Date.Before(kpoRows[j].Date)
	})

	var totalProduct, totalService float64
	for _, en := range entries {
		totalProduct += en.ProductRevenue
		totalService += en.ServiceRevenue
	}

	currentYear := time.Now().Year()
	pausalLimit := shared.PausalalLimitForYear(currentYear)
	var pausalTotal float64
	if selectedYear == currentYear {
		pausalTotal = totalProduct + totalService
	} else {
		pausalTotal, _, err = h.kpoBooks.SumForEntrepreneurUser(r.Context(), userID, currentYear)
		if err != nil {
			http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
			return
		}
	}
	pausalAlert := shared.ComputePausalAlert(pausalTotal, pausalLimit, time.Now())

	vatNow := time.Now()
	vatDays := 364
	if shared.IsLeapYear(vatNow.Year()) {
		vatDays = 365
	}
	vatFrom := vatNow.AddDate(0, 0, -vatDays)
	vatLimit := shared.VATLimitForDate(vatNow)
	vatTotal, err := h.kpoBooks.RollingSumForEntrepreneurUser(r.Context(), userID, vatFrom, vatNow)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	vatAlert := shared.ComputeVATAlert(vatTotal, vatLimit)

	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurKPO, map[string]any{
		"CurrentBook":  currentBook,
		"KPOBooks":     books,
		"KPOEntries":   entries,
		"KPORows":      kpoRows,
		"SelectedYear": selectedYear,
		"TotalProduct": totalProduct,
		"TotalService": totalService,
		"TotalAll":     totalProduct + totalService,
		"PausalAlert":  pausalAlert,
		"VATAlert":     vatAlert,
	})
}

func (h *Handler) handleKPOAddEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_finalized"), http.StatusForbidden)
		return
	}
	r.ParseForm()
	date, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_bad_date"), http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if _, err := h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		Description:    strings.TrimSpace(r.FormValue("description")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOUpdateEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_finalized"), http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	r.ParseForm()
	date, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_bad_date"), http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if err := h.kpoBooks.UpdateEntry(r.Context(), kpo.Entry{
		ID:             entryID,
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		Description:    strings.TrimSpace(r.FormValue("description")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOReorderEntries(w http.ResponseWriter, r *http.Request) {
	_, book, _, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_finalized"), http.StatusForbidden)
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
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleKPODeleteEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, i18n.FromContext(r.Context()).T("kpo.error_finalized"), http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.DeleteEntry(r.Context(), book.ID, entryID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOFinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Finalize(r.Context(), book.ID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOUnfinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Unfinalize(r.Context(), book.ID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *Handler) handleKPOPDF(w http.ResponseWriter, r *http.Request) {
	userID, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	entries, err := h.kpoBooks.ListEntries(r.Context(), book.ID)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	info := kpo.EntrepreneurInfo{}
	if ent, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		info = kpo.EntrepreneurInfo{
			Name:         ent.Name,
			PIB:          ent.PIB,
			MB:           ent.MB,
			Address:      ent.Address,
			TaxpayerCode: ent.TaxpayerCode,
			ActivityCode: ent.ActivityCode,
		}
	}

	pdfBytes, err := kpo.GeneratePDF(book, entries, info)
	if err != nil {
		log.Printf("kpo pdf generation error: %v", err)
		http.Error(w, i18n.FromContext(r.Context()).T("error.pdf_generation_failed"), http.StatusInternalServerError)
		return
	}

	filename := "kpo-" + strconv.Itoa(year) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if _, err := w.Write(pdfBytes); err != nil {
		log.Printf("kpo pdf write error: %v", err)
	}
}

