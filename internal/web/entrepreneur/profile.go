package entrepreneur

import (
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"buh/internal/i18n"
	"buh/internal/validate"
	"buh/internal/web/shared"
)

func (h *Handler) handleProfileForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	managed, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurProfile, map[string]any{
		"Paired":          managed.ID != uuid.Nil,
		"Managed":         managed,
		"UnpairedSuccess": r.URL.Query().Get("unpaired") == "1",
	})
}

func (h *Handler) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	managed, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	if err != nil {
		h.renderError(w, r, http.StatusForbidden)
		return
	}

	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	pib := strings.TrimSpace(r.FormValue("pib"))

	mb := strings.TrimSpace(r.FormValue("mb"))

	l := i18n.FromContext(r.Context())
	var errMsg string
	switch {
	case name == "" || pib == "":
		errMsg = l.T("profile.error_required_fields")
	case !validate.PIB(pib):
		errMsg = l.T("profile.error_invalid_pib")
	case mb != "" && !validate.MB(mb):
		errMsg = l.T("profile.error_invalid_mb")
	}
	if errMsg != "" {
		managed.Name = name
		managed.PIB = pib
		managed.MB = mb
		managed.Address = strings.TrimSpace(r.FormValue("address"))
		managed.BankAccount = strings.TrimSpace(r.FormValue("bank_account"))
		managed.TaxpayerCode = strings.TrimSpace(r.FormValue("taxpayer_code"))
		managed.ActivityCode = strings.TrimSpace(r.FormValue("activity_code"))
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurProfile, map[string]any{
			"Paired":   true,
			"Managed":  managed,
			"EditMode": true,
			"Error":    errMsg,
		})
		return
	}

	managed.Name = name
	managed.PIB = pib
	managed.MB = mb
	managed.Address = strings.TrimSpace(r.FormValue("address"))
	managed.BankAccount = strings.TrimSpace(r.FormValue("bank_account"))
	managed.TaxpayerCode = strings.TrimSpace(r.FormValue("taxpayer_code"))
	managed.ActivityCode = strings.TrimSpace(r.FormValue("activity_code"))

	if err := h.entrepreneurs.Update(r.Context(), managed); err != nil {
		h.renderError(w, r, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/profile", http.StatusFound)
}

func (h *Handler) handleUnpair(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	e, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	if err := h.slips.CopyToEntrepreneurUser(r.Context(), e.ID, userID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	if err := h.kpoBooks.CopyToEntrepreneurUser(r.Context(), e.ID, userID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	if h.slipMerges != nil {
		if err := h.slipMerges.DeleteForPairing(r.Context(), e.ID); err != nil {
			log.Printf("unpair: delete slip merges for %s: %v", e.ID, err)
		}
	}

	if err := h.entrepreneurs.Unpair(r.Context(), e.ID); err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}

	// Notify the accountant.
	_ = h.accountants.SetPendingNotice(r.Context(), e.AccountantID.String(), "notice.unpaired_by_entrepreneur")

	http.Redirect(w, r, "/e/profile?unpaired=1", http.StatusFound)
}
