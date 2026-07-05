package entrepreneur

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"buh/internal/auth"
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

	renderErr := func(msg string) {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurRegister, map[string]any{"Error": msg, "Email": email})
	}
	if email == "" || password == "" {
		renderErr("Е-пошта и лозинка су обавезни.")
		return
	}
	if password != confirm {
		renderErr("Лозинке се не подударају.")
		return
	}
	u, err := h.entrepreneurUsers.Create(r.Context(), email, password)
	if err != nil {
		renderErr("Та е-пошта је већ у употреби.")
		return
	}
	if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeEntrepreneur, UserID: u.ID.String()}); err != nil {
		http.Error(w, "Грешка при постављању сесије", http.StatusInternalServerError)
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
