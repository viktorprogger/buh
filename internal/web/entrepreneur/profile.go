package entrepreneur

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"buh/internal/i18n"
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
		"Paired":  managed.ID != uuid.Nil,
		"Managed": managed,
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

	if name == "" || pib == "" {
		managed.Name = name
		managed.PIB = pib
		managed.MB = strings.TrimSpace(r.FormValue("mb"))
		managed.Address = strings.TrimSpace(r.FormValue("address"))
		managed.BankAccount = strings.TrimSpace(r.FormValue("bank_account"))
		managed.TaxpayerCode = strings.TrimSpace(r.FormValue("taxpayer_code"))
		managed.ActivityCode = strings.TrimSpace(r.FormValue("activity_code"))
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurProfile, map[string]any{
			"Paired":   true,
			"Managed":  managed,
			"EditMode": true,
			"Error":    i18n.FromContext(r.Context()).T("profile.error_required_fields"),
		})
		return
	}

	managed.Name = name
	managed.PIB = pib
	managed.MB = strings.TrimSpace(r.FormValue("mb"))
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
