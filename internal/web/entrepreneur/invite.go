package entrepreneur

import (
	"net/http"

	"github.com/google/uuid"

	"buh/internal/invitation"
	"buh/internal/web/shared"
)

func (h *Handler) handleInviteForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	paired, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	if paired.ID != uuid.Nil {
		http.Redirect(w, r, "/e/", http.StatusFound)
		return
	}
	shared.RenderTemplate(w, h.tmpl.EntrepreneurInviteAcc, nil)
}

func (h *Handler) handleSendInvite(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	inv, err := h.invitations.Create(r.Context(), invitation.Invitation{
		InviterType: invitation.InviterTypeEntrepreneur,
		InviterID:   userID,
	})
	if err != nil {
		http.Error(w, "Грешка при креирању позивнице", http.StatusInternalServerError)
		return
	}
	shared.RenderTemplate(w, h.tmpl.InviteToken, map[string]any{
		"Token":       inv.Token,
		"InviterType": "entrepreneur",
	})
}
