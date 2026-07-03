package accountant

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/kpo"
	"buh/internal/web/shared"
)

// kpoBookFromPath resolves the entrepreneur ID and year from path values, verifies ownership,
// and returns the KPO book (creating it if it doesn't exist for this accountant).
func (h *Handler) kpoBookFromPath(r *http.Request) (entrepreneur.Entrepreneur, kpo.Book, int, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("bad id")
	}
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("forbidden")
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if err != nil {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, err
	}
	if e.AccountantID != accountantID {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("forbidden")
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("bad year")
	}
	book, err := h.kpoBooks.FindOrCreateForAccountant(r.Context(), id, year)
	return e, book, year, err
}

func (h *Handler) handleKPOAddEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}
	r.ParseForm()
	collectionDate, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум (очекује се дд.мм)", http.StatusBadRequest)
		return
	}
	productRev := shared.Round2(shared.ParseAmount(r.FormValue("product_revenue")))
	serviceRev := shared.Round2(shared.ParseAmount(r.FormValue("service_revenue")))

	if _, err = h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      book.ID,
		CollectionDate: collectionDate,
		InvoiceNumber:  r.FormValue("invoice_number"),
		Description:    strings.TrimSpace(r.FormValue("description")),
		ProductRevenue: productRev,
		ServiceRevenue: serviceRev,
	}); err != nil {
		http.Error(w, "Грешка при уносу ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d#kpo-new", r.PathValue("id"), year), http.StatusFound)
}

func (h *Handler) handleKPOUpdateEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	r.ParseForm()
	collectionDate, err := shared.ParseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум (очекује се дд.мм)", http.StatusBadRequest)
		return
	}
	productRev := shared.Round2(shared.ParseAmount(r.FormValue("product_revenue")))
	serviceRev := shared.Round2(shared.ParseAmount(r.FormValue("service_revenue")))

	if err = h.kpoBooks.UpdateEntry(r.Context(), kpo.Entry{
		ID:             entryID,
		KPOBookID:      book.ID,
		CollectionDate: collectionDate,
		InvoiceNumber:  r.FormValue("invoice_number"),
		Description:    strings.TrimSpace(r.FormValue("description")),
		ProductRevenue: productRev,
		ServiceRevenue: serviceRev,
	}); err != nil {
		http.Error(w, "Грешка при измени ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d#kpo", r.PathValue("id"), year), http.StatusFound)
}

func (h *Handler) handleKPOReorderEntries(w http.ResponseWriter, r *http.Request) {
	_, book, _, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}
	if err = r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	rawIDs := r.Form["ids"]
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, s := range rawIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "Неисправан ID", http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}
	if err = h.kpoBooks.ReorderEntries(r.Context(), book.ID, ids); err != nil {
		http.Error(w, "Грешка при преуређивању", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleKPODeleteEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.DeleteEntry(r.Context(), book.ID, entryID); err != nil {
		http.Error(w, "Грешка при брисању ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

func (h *Handler) handleKPOFinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Finalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при укњижавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

func (h *Handler) handleKPOUnfinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Unfinalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при откључавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

func (h *Handler) handleKPOOpenYear(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	year, err := strconv.Atoi(r.FormValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		http.Error(w, "Неисправна година", http.StatusBadRequest)
		return
	}
	if _, err := h.kpoBooks.FindOrCreateForAccountant(r.Context(), e.ID, year); err != nil {
		http.Error(w, "Грешка при отварању КПО", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d#kpo", id, year), http.StatusFound)
}

func (h *Handler) handleKPOMergeView(w http.ResponseWriter, r *http.Request) {
	e, accBook, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	accEntries, err := h.kpoBooks.ListEntries(r.Context(), accBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	var eBook kpo.Book
	var eEntries []kpo.Entry
	if e.EntrepreneurUserID != nil {
		eBook, err = h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
		if err == nil {
			eEntries, _ = h.kpoBooks.ListEntries(r.Context(), eBook.ID)
		}
	}

	shared.RenderTemplate(w, h.tmpl.KpoMerge, map[string]any{
		"Entrepreneur": e,
		"Year":         year,
		"AccBook":      accBook,
		"AccEntries":   accEntries,
		"EBook":        eBook,
		"EEntries":     eEntries,
		"HasEBook":     e.EntrepreneurUserID != nil && eBook.ID != uuid.Nil,
	})
}

func (h *Handler) handleKPOMergeCopyEntry(w http.ResponseWriter, r *http.Request) {
	e, accBook, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if accBook.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	eid, err := uuid.Parse(r.PathValue("eid"))
	if err != nil {
		h.renderError(w, http.StatusBadRequest)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eBook, err := h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eEntries, err := h.kpoBooks.ListEntries(r.Context(), eBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању", http.StatusInternalServerError)
		return
	}
	var src *kpo.Entry
	for i := range eEntries {
		if eEntries[i].ID == eid {
			src = &eEntries[i]
			break
		}
	}
	if src == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, err := h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      accBook.ID,
		CollectionDate: src.CollectionDate,
		InvoiceNumber:  src.InvoiceNumber,
		Description:    src.Description,
		ProductRevenue: src.ProductRevenue,
		ServiceRevenue: src.ServiceRevenue,
	}); err != nil {
		http.Error(w, "Грешка при копирању ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String()+"/kpo/"+strconv.Itoa(year)+"/merge", http.StatusFound)
}

func (h *Handler) handleKPOPDF(w http.ResponseWriter, r *http.Request) {
	e, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	entries, err := h.kpoBooks.ListEntries(r.Context(), book.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
		return
	}

	info := kpo.EntrepreneurInfo{
		Name:         e.Name,
		PIB:          e.PIB,
		MB:           e.MB,
		Address:      e.Address,
		TaxpayerCode: e.TaxpayerCode,
		ActivityCode: e.ActivityCode,
	}

	pdfBytes, err := kpo.GeneratePDF(book, entries, info)
	if err != nil {
		log.Printf("kpo pdf generation error: %v", err)
		http.Error(w, "PDF generation failed", http.StatusInternalServerError)
		return
	}

	filename := "kpo-" + strconv.Itoa(year) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if _, err := w.Write(pdfBytes); err != nil {
		log.Printf("kpo pdf write error: %v", err)
	}
}

func (h *Handler) handleKPOMergeMarkDone(w http.ResponseWriter, r *http.Request) {
	e, _, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eBook, err := h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.MarkMerged(r.Context(), eBook.ID); err != nil {
		http.Error(w, "Грешка при обележавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String()+"/kpo/"+strconv.Itoa(year)+"/merge", http.StatusFound)
}
