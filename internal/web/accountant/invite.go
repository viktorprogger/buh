package accountant

import (
	"net/http"

	"github.com/google/uuid"

	"buh/internal/i18n"
	"buh/internal/invitation"
	"buh/internal/web/shared"
)

func (h *Handler) handleAccountantInviteEntrepreneur(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	if e.IsPaired() {
		h.renderError(w, r, http.StatusForbidden)
		return
	}
	accountantID, _ := h.accountantFromSession(r)
	inv, err := h.invitations.Create(r.Context(), invitation.Invitation{
		InviterType:           invitation.InviterTypeAccountant,
		InviterID:             accountantID,
		ManagedEntrepreneurID: &id,
	})
	if err != nil {
		http.Error(w, i18n.FromContext(r.Context()).T("error.server_error"), http.StatusInternalServerError)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.InviteToken, map[string]any{
		"Token":        inv.Token,
		"Entrepreneur": e,
		"InviterType":  "accountant",
	})
}
