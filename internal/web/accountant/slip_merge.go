package accountant

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/i18n"
	"buh/internal/sliphistory"
	"buh/internal/sliprecord"
	"buh/internal/web/shared"
)

// slipConflict is a pair of slips for the same (purpose, year, advance) key that have differing fields.
type slipConflict struct {
	AccountantSlip   sliprecord.SlipRecord
	EntrepreneurSlip sliprecord.SlipRecord
}

type slipMergeKey struct {
	Purpose string
	Year    int
	Advance bool
}

func (h *Handler) handleSlipMergeView(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, eid)
	if !ok {
		return
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	accountantID, _ := h.accountantFromSession(r)
	conflicts, autoMerge, err := h.buildMergeData(r, e, year)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	// Auto-merge when no conflicts.
	if len(conflicts) == 0 {
		if mergeErr := h.autoMerge(r, e, year, autoMerge, accountantID); mergeErr != nil {
			http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"?merged=1", http.StatusFound)
		return
	}

	shared.RenderTemplate(w, r, h.tmpl.SlipMerge, map[string]any{
		"Entrepreneur": e,
		"Year":         year,
		"Conflicts":    conflicts,
		"AutoMerge":    autoMerge,
	})
}

func (h *Handler) handleSlipMergePickSide(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, eid)
	if !ok {
		return
	}
	if _, err := strconv.Atoi(r.PathValue("year")); err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	r.ParseForm()
	// keepID is the slip ID to keep; the other slip in the conflict will be deleted.
	keepID, err := uuid.Parse(r.FormValue("keep"))
	if err != nil {
		h.renderError(w, r, http.StatusBadRequest)
		return
	}

	accountantID, _ := h.accountantFromSession(r)
	keepSlip, err := h.slips.FindByID(r.Context(), keepID)
	if errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}

	// If the winner is the entrepreneur's slip, move it to managed_entrepreneur_id.
	if keepSlip.EntrepreneurUserID != nil && *keepSlip.EntrepreneurUserID == *e.EntrepreneurUserID {
		if err := h.slips.MoveToManagedEntrepreneur(r.Context(), keepID, e.ID); err != nil {
			http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
			return
		}
		h.logSlipHistory(r.Context(), keepID, sliphistory.EventMerged, accountantID, map[string]any{
			"from": "entrepreneur_user_id:" + e.EntrepreneurUserID.String(),
			"to":   "managed_entrepreneur_id:" + e.ID.String(),
		})
	}

	// Delete the losing slip (the one not chosen).
	loserID, err := uuid.Parse(r.FormValue("discard"))
	if err != nil {
		http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"/slips/merge/"+r.PathValue("year"), http.StatusFound)
		return
	}
	loserSlip, err := h.slips.FindByID(r.Context(), loserID)
	if err != nil && !errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	if err == nil {
		// Verify the loser slip belongs to this merge — either the managed entrepreneur or the paired entrepreneur user.
		ownedByManagedEntrepreneur := loserSlip.ManagedEntrepreneurID == e.ID
		ownedByEntrepreneurUser := loserSlip.EntrepreneurUserID != nil && e.EntrepreneurUserID != nil && *loserSlip.EntrepreneurUserID == *e.EntrepreneurUserID
		if !ownedByManagedEntrepreneur && !ownedByEntrepreneurUser {
			h.renderError(w, r, http.StatusForbidden)
			return
		}
		if delErr := h.slips.Delete(r.Context(), loserID); delErr != nil && !errors.Is(delErr, sliprecord.ErrNotFound) {
			log.Printf("slip merge: delete loser %s: %v", loserID, delErr)
		}
	}

	http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"/slips/merge/"+r.PathValue("year"), http.StatusFound)
}

func (h *Handler) handleSlipMergeDone(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, eid)
	if !ok {
		return
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	accountantID, _ := h.accountantFromSession(r)

	// Re-check there are no remaining conflicts before marking done.
	conflicts, autoMerge, err := h.buildMergeData(r, e, year)
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	if len(conflicts) > 0 {
		// Still has unresolved conflicts; redirect back to merge page.
		http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"/slips/merge/"+r.PathValue("year"), http.StatusFound)
		return
	}

	if err := h.autoMerge(r, e, year, autoMerge, accountantID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"?merged=1", http.StatusFound)
}

// buildMergeData loads accountant and entrepreneur slips for the year and identifies conflicts.
func (h *Handler) buildMergeData(r *http.Request, e entrepreneur.Entrepreneur, year int) (conflicts []slipConflict, autoMerge []sliprecord.SlipRecord, err error) {
	acctSlips, err := h.slips.ListByManagedEntrepreneur(r.Context(), e.ID)
	if err != nil {
		return nil, nil, err
	}
	entSlips, err := h.slips.ListByEntrepreneurUser(r.Context(), *e.EntrepreneurUserID)
	if err != nil {
		return nil, nil, err
	}

	// Filter to the target year.
	acctByKey := make(map[slipMergeKey]sliprecord.SlipRecord)
	for _, s := range acctSlips {
		if s.Year == year {
			acctByKey[slipMergeKey{s.Purpose, s.Year, s.Advance}] = s
		}
	}
	entByKey := make(map[slipMergeKey]sliprecord.SlipRecord)
	for _, s := range entSlips {
		if s.Year == year {
			entByKey[slipMergeKey{s.Purpose, s.Year, s.Advance}] = s
		}
	}

	// Entrepreneur-only slips → will be auto-moved.
	for key, es := range entByKey {
		if _, exists := acctByKey[key]; !exists {
			autoMerge = append(autoMerge, es)
			continue
		}
		as := acctByKey[key]
		// Identical → auto-merge (keep accountant copy, delete entrepreneur copy).
		if slipsIdentical(as, es) {
			autoMerge = append(autoMerge, es)
		} else {
			conflicts = append(conflicts, slipConflict{AccountantSlip: as, EntrepreneurSlip: es})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].AccountantSlip.Purpose < conflicts[j].AccountantSlip.Purpose })
	sort.Slice(autoMerge, func(i, j int) bool { return autoMerge[i].Purpose < autoMerge[j].Purpose })
	return conflicts, autoMerge, nil
}

func slipsIdentical(a, b sliprecord.SlipRecord) bool {
	return a.PaymentCode == b.PaymentCode &&
		a.Amount == b.Amount &&
		a.Currency == b.Currency &&
		a.PayeeAccount == b.PayeeAccount &&
		a.Reference == b.Reference &&
		a.Payee == b.Payee &&
		a.Payer == b.Payer
}

// autoMerge processes the no-conflict slips:
// - entrepreneur-only → move to managed_entrepreneur_id
// - identical duplicates → delete entrepreneur copy
func (h *Handler) autoMerge(r *http.Request, e entrepreneur.Entrepreneur, year int, autoMergeSlips []sliprecord.SlipRecord, accountantID uuid.UUID) error {
	acctSlips, err := h.slips.ListByManagedEntrepreneur(r.Context(), e.ID)
	if err != nil {
		return err
	}
	acctByKey := make(map[slipMergeKey]bool)
	for _, s := range acctSlips {
		if s.Year == year {
			acctByKey[slipMergeKey{s.Purpose, s.Year, s.Advance}] = true
		}
	}

	for _, es := range autoMergeSlips {
		if es.EntrepreneurUserID == nil {
			continue
		}
		key := slipMergeKey{es.Purpose, es.Year, es.Advance}
		if acctByKey[key] {
			// Duplicate → delete entrepreneur copy.
			if delErr := h.slips.Delete(r.Context(), es.ID); delErr != nil && !errors.Is(delErr, sliprecord.ErrNotFound) {
				log.Printf("autoMerge: delete duplicate %s: %v", es.ID, delErr)
			}
		} else {
			// Entrepreneur-only → move.
			if moveErr := h.slips.MoveToManagedEntrepreneur(r.Context(), es.ID, e.ID); moveErr != nil {
				log.Printf("autoMerge: move %s: %v", es.ID, moveErr)
				continue
			}
			h.logSlipHistory(r.Context(), es.ID, sliphistory.EventMerged, accountantID, map[string]any{
				"from": "entrepreneur_user_id:" + e.EntrepreneurUserID.String(),
				"to":   "managed_entrepreneur_id:" + e.ID.String(),
			})
		}
	}

	if h.slipMerges != nil {
		return h.slipMerges.MarkMerged(r.Context(), e.ID, year, accountantID)
	}
	return nil
}

func (h *Handler) handleUnpair(w http.ResponseWriter, r *http.Request) {
	eid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, eid)
	if !ok {
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	// Copy all accountant-owned slips to the entrepreneur user.
	if err := h.slips.CopyToEntrepreneurUser(r.Context(), eid, *e.EntrepreneurUserID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	// Copy all KPO books and entries to the entrepreneur user.
	if err := h.kpoBooks.CopyToEntrepreneurUser(r.Context(), eid, *e.EntrepreneurUserID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	// Delete merge records — the merged state is no longer meaningful.
	if h.slipMerges != nil {
		if err := h.slipMerges.DeleteForPairing(r.Context(), eid); err != nil {
			log.Printf("unpair: delete slip merges for %s: %v", eid, err)
		}
	}

	// Unlink the entrepreneur user.
	if err := h.entrepreneurs.Unpair(r.Context(), eid); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	// Notify the entrepreneur user.
	_ = h.entrepreneurUsers.SetPendingNotice(r.Context(), *e.EntrepreneurUserID, "notice.unpaired_by_accountant")

	http.Redirect(w, r, "/a/entrepreneurs/"+eid.String()+"?unpaired=1", http.StatusFound)
}

