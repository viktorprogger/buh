package accountant

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/i18n"
	"buh/internal/ips"
	"buh/internal/slip"
	"buh/internal/sliphistory"
	"buh/internal/sliprecord"
	"buh/internal/web/shared"
)

var fieldKeys = map[string]string{
	"amount":       "slip_history.field_amount",
	"currency":     "slip_history.field_currency",
	"purpose":      "slip_history.field_purpose",
	"payee":        "slip_history.field_payee",
	"payeeAccount": "slip_history.field_payeeAccount",
	"reference":    "slip_history.field_reference",
	"payer":        "slip_history.field_payer",
	"paymentCode":  "slip_history.field_paymentCode",
	"year":         "slip_history.field_year",
	"advance":      "slip_history.field_advance",
}

var eventKeys = map[sliphistory.Event]string{
	sliphistory.EventCreated:  "slip_history.event_created",
	sliphistory.EventImported: "slip_history.event_imported",
	sliphistory.EventUpdated:  "slip_history.event_updated",
	sliphistory.EventDeleted:  "slip_history.event_deleted",
}

var actorKeys = map[string]string{
	"accountant":   "slip_history.actor_accountant",
	"entrepreneur": "slip_history.actor_entrepreneur",
}

// HistoryChange is one field change for display in the slip history timeline.
type HistoryChange struct {
	Label string
	Old   string
	New   string
}

// HistoryEntry is one formatted history record for display.
type HistoryEntry struct {
	EventLabel string
	ActorLabel string
	At         time.Time
	Changes    []HistoryChange
}

func buildHistoryEntries(records []sliphistory.Record, l *i18n.Localizer) []HistoryEntry {
	entries := make([]HistoryEntry, 0, len(records))
	for _, rec := range records {
		eventLabel := string(rec.Event)
		if k, ok := eventKeys[rec.Event]; ok {
			eventLabel = l.T(k)
		}
		actorLabel := rec.ActorType
		if k, ok := actorKeys[rec.ActorType]; ok {
			actorLabel = l.T(k)
		}
		entry := HistoryEntry{
			EventLabel: eventLabel,
			ActorLabel: actorLabel,
			At:         rec.ChangedAt,
		}

		for key, val := range rec.Changes {
			label := key
			if k, ok := fieldKeys[key]; ok {
				label = l.T(k)
			}
			// diff entry: {"old": X, "new": Y}
			if m, ok := val.(map[string]interface{}); ok {
				entry.Changes = append(entry.Changes, HistoryChange{
					Label: label,
					Old:   fmt.Sprintf("%v", m["old"]),
					New:   fmt.Sprintf("%v", m["new"]),
				})
				continue
			}
			// snapshot entry (created/deleted): just show the value
			entry.Changes = append(entry.Changes, HistoryChange{
				Label: label,
				New:   fmt.Sprintf("%v", val),
			})
		}
		entries = append(entries, entry)
	}
	return entries
}

func (h *Handler) logSlipHistory(ctx context.Context, slipID uuid.UUID, event sliphistory.Event, actorID uuid.UUID, changes map[string]any) {
	if h.slipHistory == nil {
		return
	}
	if err := h.slipHistory.Log(ctx, slipID, event, "accountant", actorID, changes); err != nil {
		log.Printf("slip history log failed slip=%s event=%s: %v", slipID, event, err)
	}
}

func (h *Handler) findOwnedSlip(w http.ResponseWriter, r *http.Request, id uuid.UUID) (sliprecord.SlipRecord, bool) {
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		h.renderError(w, r, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	s, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, r, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), s.EntrepreneurID)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		h.renderError(w, r, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	if e.AccountantID != accountantID {
		h.renderError(w, r, http.StatusForbidden)
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
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	var historyEntries []HistoryEntry
	if h.slipHistory != nil {
		if records, err := h.slipHistory.ListBySlip(r.Context(), id); err == nil {
			historyEntries = buildHistoryEntries(records, i18n.FromContext(r.Context()))
		}
	}
	shared.RenderTemplate(w, r, h.tmpl.Slip, map[string]any{
		"Slip":        s,
		"Saved":       r.URL.Query().Get("saved") == "1",
		"Downloading": r.URL.Query().Get("download") == "1",
		"History":     historyEntries,
	})
}

func (h *Handler) handleSlipSave(w http.ResponseWriter, r *http.Request) {
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
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	if actorID, ok := h.accountantFromSession(r); ok {
		if diff := sliprecord.Diff(existing, updated); diff != nil {
			h.logSlipHistory(r.Context(), id, sliphistory.EventUpdated, actorID, diff)
		}
	}
	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1", http.StatusFound)
}

func (h *Handler) handleSlipDownload(w http.ResponseWriter, r *http.Request) {
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
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	if actorID, ok := h.accountantFromSession(r); ok {
		if diff := sliprecord.Diff(existing, updated); diff != nil {
			h.logSlipHistory(r.Context(), id, sliphistory.EventUpdated, actorID, diff)
		}
	}
	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1&download=1", http.StatusFound)
}

func (h *Handler) handleSlipDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	entrepreneurID := s.EntrepreneurID
	if actorID, ok := h.accountantFromSession(r); ok {
		h.logSlipHistory(r.Context(), id, sliphistory.EventDeleted, actorID, sliprecord.Snapshot(s))
	}
	if err := h.slips.Delete(r.Context(), id); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+entrepreneurID.String(), http.StatusFound)
}

func (h *Handler) handleSlipPDF(w http.ResponseWriter, r *http.Request) {
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
		O:  "",
		P:  s.Payer,
	}
	pay.I = ips.FormatAmount(s.Currency, strings.ReplaceAll(s.Amount, ",", "."))

	tmp, err := os.CreateTemp("", "buh-slip-*.pdf")
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
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
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
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
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.SlipNew, map[string]any{
		"Entrepreneur": e,
		"Form":         shared.SlipNewForm{SF: "253", Currency: "RSD", P: e.Name, Year: time.Now().Year()},
	})
}

func (h *Handler) handleSlipNewSubmit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
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

	l := i18n.FromContext(r.Context())
	renderErr := func(msg string) {
		shared.RenderTemplate(w, r, h.tmpl.SlipNew, map[string]any{
			"Entrepreneur": e,
			"Form":         form,
			"Error":        msg,
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
		http.Error(w, l.T("error.server_error"), http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		renderErr(fmt.Sprintf("%s: %v", l.T("error.server_error"), err))
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
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	if actorID, ok := h.accountantFromSession(r); ok {
		h.logSlipHistory(r.Context(), saved.ID, sliphistory.EventCreated, actorID, sliprecord.Snapshot(saved))
	}
	http.Redirect(w, r, "/a/slips/"+saved.ID.String(), http.StatusFound)
}
