package entrepreneur

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"buh/internal/auth"
	"buh/internal/i18n"
	"buh/internal/web/shared"
)

func (h *Handler) HandleRegisterForm(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurRegister, nil)
}

func (h *Handler) HandleRegisterSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.FormValue("email")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	l := i18n.FromContext(r.Context())
	renderErr := func(msg string) {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurRegister, map[string]any{"Error": msg, "Email": email})
	}
	if email == "" || password == "" {
		renderErr(l.T("entrepreneur_register.error_required"))
		return
	}
	if password != confirm {
		renderErr(l.T("entrepreneur_register.error_password_mismatch"))
		return
	}
	u, err := h.entrepreneurUsers.Create(r.Context(), email, password)
	if err != nil {
		renderErr(l.T("entrepreneur_register.error_email_taken"))
		return
	}
	if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeEntrepreneur, UserID: u.ID.String()}); err != nil {
		http.Error(w, l.T("login.error_session"), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	paired, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurDashboard, map[string]any{
		"Paired":      paired.ID != uuid.Nil,
		"Managed":     paired,
		"CurrentYear": time.Now().Year(),
	})
}
