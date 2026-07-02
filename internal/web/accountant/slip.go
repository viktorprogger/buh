package accountant

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/ips"
	"buh/internal/slip"
	"buh/internal/sliprecord"
	"buh/internal/web/shared"
)

func (h *Handler) findOwnedSlip(w http.ResponseWriter, r *http.Request, id uuid.UUID) (sliprecord.SlipRecord, bool) {
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		h.renderError(w, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	s, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), s.EntrepreneurID)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		h.renderError(w, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	if e.AccountantID != accountantID {
		h.renderError(w, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	return s, true
}

func slipFormToRecord(r *http.Request, existing sliprecord.SlipRecord) sliprecord.SlipRecord {
	rawAccount := strings.ReplaceAll(strings.TrimSpace(r.FormValue("R")), "-", "")
	existing.Payer = strings.TrimSpace(r.FormValue("P"))
	existing.Purpose = strings.TrimSpace(r.FormValue("S"))
	existing.Payee = strings.TrimSpace(r.FormValue("N"))
	existing.PayeeAccount = rawAccount
	existing.Reference = strings.TrimSpace(r.FormValue("RO"))
	existing.PaymentCode = strings.TrimSpace(r.FormValue("SF"))
	existing.Amount = shared.FormatAmount(r.FormValue("amount"))
	existing.Currency = r.FormValue("currency")
	if y, err := strconv.Atoi(r.FormValue("year")); err == nil && y > 0 {
		existing.Year = y
	}
	existing.Advance = r.FormValue("advance") == "on"
	return existing
}

func (h *Handler) handleSlip(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	shared.RenderTemplate(w, h.tmpl.Slip, map[string]any{
		"Slip":        s,
		"Saved":       r.URL.Query().Get("saved") == "1",
		"Downloading": r.URL.Query().Get("download") == "1",
	})
}

func (h *Handler) handleSlipSave(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1", http.StatusFound)
}

func (h *Handler) handleSlipDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1&download=1", http.StatusFound)
}

func (h *Handler) handleSlipDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	entrepreneurID := s.EntrepreneurID
	if err := h.slips.Delete(r.Context(), id); err != nil {
		http.Error(w, "Грешка при брисању уплатнице", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+entrepreneurID.String(), http.StatusFound)
}

func (h *Handler) handleSlipPDF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	pay := &ips.Payment{
		K:  ips.CodePR,
		V:  "01",
		C:  "1",
		R:  s.PayeeAccount,
		N:  s.Payee,
		SF: s.PaymentCode,
		S:  s.Purpose,
		RO: s.Reference,
		O:  "",
		P:  s.Payer,
	}
	pay.I = ips.FormatAmount(s.Currency, strings.ReplaceAll(s.Amount, ",", "."))

	tmp, err := os.CreateTemp("", "buh-slip-*.pdf")
	if err != nil {
		http.Error(w, "Грешка при креирању фајла", http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		http.Error(w, fmt.Sprintf("Грешка при генерисању PDF: %v", err), http.StatusInternalServerError)
		return
	}
	pdfBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		http.Error(w, "Грешка при читању PDF", http.StatusInternalServerError)
		return
	}
	filename := fmt.Sprintf("uplatnica-%s.pdf", shared.SanitizeFilename(pay.N))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(pdfBytes)
}

func (h *Handler) handleSlipNewForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	shared.RenderTemplate(w, h.tmpl.SlipNew, map[string]any{
		"Entrepreneur": e,
		"Form":         shared.SlipNewForm{SF: "253", Currency: "RSD", P: e.Name, Year: time.Now().Year()},
	})
}

func (h *Handler) handleSlipNewSubmit(w http.ResponseWriter, r *http.Request) {
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
	formYear := time.Now().Year()
	if y, err := strconv.Atoi(r.FormValue("year")); err == nil && y > 0 {
		formYear = y
	}
	form := shared.SlipNewForm{
		S:        strings.TrimSpace(r.FormValue("S")),
		N:        strings.TrimSpace(r.FormValue("N")),
		R:        strings.TrimSpace(r.FormValue("R")),
		RO:       strings.TrimSpace(r.FormValue("RO")),
		SF:       strings.TrimSpace(r.FormValue("SF")),
		Amount:   strings.TrimSpace(r.FormValue("amount")),
		Currency: r.FormValue("currency"),
		P:        strings.TrimSpace(r.FormValue("P")),
		Year:     formYear,
		Advance:  r.FormValue("advance") == "on",
	}

	renderErr := func(msg string) {
		shared.RenderTemplate(w, h.tmpl.SlipNew, map[string]any{
			"Entrepreneur": e,
			"Form":         form,
			"Error":        msg,
		})
	}

	if form.S == "" || form.N == "" || form.R == "" || form.SF == "" || form.Amount == "" {
		renderErr("Молимо попуните сва обавезна поља.")
		return
	}

	rawAccount := strings.ReplaceAll(form.R, "-", "")
	pay := &ips.Payment{
		K:  ips.CodePR,
		V:  "01",
		C:  "1",
		R:  rawAccount,
		N:  form.N,
		SF: form.SF,
		S:  form.S,
		RO: form.RO,
		P:  form.P,
	}
	pay.I = ips.FormatAmount(form.Currency, strings.ReplaceAll(form.Amount, ",", "."))

	tmp, err := os.CreateTemp("", "buh-slip-*.pdf")
	if err != nil {
		http.Error(w, "Грешка при креирању фајла", http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		renderErr(fmt.Sprintf("Грешка при генерисању PDF: %v", err))
		return
	}

	rec := sliprecord.SlipRecord{
		EntrepreneurID: id,
		PaymentCode:    pay.SF,
		Amount:         shared.FormatAmount(form.Amount),
		Currency:       form.Currency,
		Purpose:        pay.S,
		PayeeAccount:   rawAccount,
		Reference:      pay.RO,
		Payee:          pay.N,
		Payer:          pay.P,
		Year:           form.Year,
		Advance:        form.Advance,
	}
	saved, err := h.slips.Save(context.Background(), rec)
	if err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/slips/"+saved.ID.String(), http.StatusFound)
}
