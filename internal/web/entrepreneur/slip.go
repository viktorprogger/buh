package entrepreneur

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/i18n"
	"buh/internal/ips"
	"buh/internal/slip"
	"buh/internal/sliphistory"
	"buh/internal/sliprecord"
	"buh/internal/web/shared"
)

func (h *Handler) findOwnedSlip(w http.ResponseWriter, r *http.Request, slipID uuid.UUID) (sliprecord.SlipRecord, bool) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		h.renderError(w, r, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	s, err := h.slips.FindByID(r.Context(), slipID)
	if errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, r, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}

	// Direct ownership.
	if s.EntrepreneurUserID != nil && *s.EntrepreneurUserID == userID {
		return s, true
	}

	// Post-merge: slip belongs to the managed entrepreneur paired with this user.
	if s.ManagedEntrepreneurID != (uuid.UUID{}) {
		managed, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
		if err == nil && managed.ID == s.ManagedEntrepreneurID {
			return s, true
		}
	}

	h.renderError(w, r, http.StatusForbidden)
	return sliprecord.SlipRecord{}, false
}

func (h *Handler) logSlipHistory(ctx context.Context, slipID uuid.UUID, event sliphistory.Event, actorID uuid.UUID, changes map[string]any) {
	if h.slipHistory == nil {
		return
	}
	if err := h.slipHistory.Log(ctx, slipID, event, "entrepreneur", actorID, changes); err != nil {
		log.Printf("slip history log failed slip=%s event=%s: %v", slipID, event, err)
	}
}

func (h *Handler) handleSlipList(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	slips, err := h.slips.ListAccessibleByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	latestYear, prevYears := shared.GroupSlipsByYear(slips)

	var managedID uuid.UUID
	var mergedYears map[int]bool
	if managed, merr := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); merr == nil {
		managedID = managed.ID
		if h.slipMerges != nil {
			years, _ := h.slipMerges.ListMergedYears(r.Context(), managedID)
			mergedYears = make(map[int]bool, len(years))
			for _, y := range years {
				mergedYears[y] = true
			}
		}
	}

	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlips, map[string]any{
		"LatestYear":  latestYear,
		"PrevYears":   prevYears,
		"ManagedID":   managedID,
		"MergedYears": mergedYears,
	})
}

func (h *Handler) handleESlip(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlip, map[string]any{
		"Slip":        s,
		"Saved":       r.URL.Query().Get("saved") == "1",
		"Downloading": r.URL.Query().Get("download") == "1",
	})
}

func eSlipFormToRecord(r *http.Request, existing sliprecord.SlipRecord) sliprecord.SlipRecord {
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

func (h *Handler) handleESlipSave(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	updated := eSlipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	if actorID, ok := h.entrepreneurUserFromSession(r); ok {
		if diff := sliprecord.Diff(existing, updated); diff != nil {
			h.logSlipHistory(r.Context(), id, sliphistory.EventUpdated, actorID, diff)
		}
	}
	http.Redirect(w, r, "/e/slips/"+idStr+"?saved=1", http.StatusFound)
}

func (h *Handler) handleESlipDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	updated := eSlipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	if actorID, ok := h.entrepreneurUserFromSession(r); ok {
		if diff := sliprecord.Diff(existing, updated); diff != nil {
			h.logSlipHistory(r.Context(), id, sliphistory.EventUpdated, actorID, diff)
		}
	}
	http.Redirect(w, r, "/e/slips/"+idStr+"?saved=1&download=1", http.StatusFound)
}

func (h *Handler) handleESlipDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	if actorID, ok := h.entrepreneurUserFromSession(r); ok {
		h.logSlipHistory(r.Context(), id, sliphistory.EventDeleted, actorID, sliprecord.Snapshot(s))
	}
	if err := h.slips.Delete(r.Context(), id); err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/slips", http.StatusFound)
}

func (h *Handler) handleESlipPDF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
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
		P:  s.Payer,
	}
	pay.I = ips.FormatAmount(s.Currency, strings.ReplaceAll(s.Amount, ",", "."))

	tmp, err := os.CreateTemp("", "buh-slip-*.pdf")
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		http.Error(w, fmt.Sprintf("%s: %v", i18n.FromContext(r.Context()).T("error.server_error"), err), http.StatusInternalServerError)
		return
	}
	pdfBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	filename := fmt.Sprintf("uplatnica-%s.pdf", shared.SanitizeFilename(pay.N))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(pdfBytes)
}

func (h *Handler) handleESlipNewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	var payerName string
	if managed, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		payerName = managed.Name
	}
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipNew, map[string]any{
		"Form": shared.SlipNewForm{SF: "253", Currency: "RSD", P: payerName, Year: time.Now().Year()},
	})
}

func (h *Handler) handleESlipNewSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
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
	l := i18n.FromContext(r.Context())
	renderErr := func(msg string) {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipNew, map[string]any{
			"Form":  form,
			"Error": msg,
		})
	}
	if form.S == "" || form.N == "" || form.R == "" || form.SF == "" || form.Amount == "" {
		renderErr(l.T("slip.error_required_fields"))
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
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		renderErr(fmt.Sprintf("%s: %v", l.T("error.server_error"), err))
		return
	}

	eid := userID
	rec := sliprecord.SlipRecord{
		EntrepreneurUserID: &eid,
		PaymentCode:        pay.SF,
		Amount:             shared.FormatAmount(form.Amount),
		Currency:           form.Currency,
		Purpose:            pay.S,
		PayeeAccount:       rawAccount,
		Reference:          pay.RO,
		Payee:              pay.N,
		Payer:              pay.P,
		Year:               form.Year,
		Advance:            form.Advance,
	}
	saved, err := h.slips.Save(context.Background(), rec)
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	h.logSlipHistory(r.Context(), saved.ID, sliphistory.EventCreated, userID, sliprecord.Snapshot(saved))
	http.Redirect(w, r, "/e/slips", http.StatusFound)
}

func (h *Handler) handleESlipUploadForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	var pib string
	if managed, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		pib = managed.PIB
	}
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipUpload, map[string]any{
		"PIBMissing": pib == "",
	})
}

func (h *Handler) handleESlipUploadSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	managed, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	if err != nil || managed.PIB == "" {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipUpload, map[string]any{
			"PIBMissing": true,
		})
		return
	}

	l := i18n.FromContext(r.Context())
	renderErr := func(msg string) {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipUpload, map[string]any{
			"PIBMissing": false,
			"Error":      msg,
		})
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.renderError(w, r, http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["pdf"]
	if len(files) == 0 {
		http.Redirect(w, r, "/e/slips/upload", http.StatusFound)
		return
	}

	if h.importer == nil {
		renderErr(l.T("error.server_error"))
		return
	}

	fh := files[0]
	f, openErr := fh.Open()
	if openErr != nil {
		renderErr(l.T("error.server_error"))
		return
	}
	data, readErr := io.ReadAll(f)
	f.Close()
	if readErr != nil {
		renderErr(l.T("error.server_error"))
		return
	}

	results, processErr := h.importer.ProcessFileDataForEntrepreneurUser(r.Context(), userID, managed.PIB, fh.Filename, data)
	if processErr != nil {
		renderErr(processErr.Error())
		return
	}

	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurSlipUpload, map[string]any{
		"PIBMissing": false,
		"Results":    results,
		"Done":       true,
	})
}
