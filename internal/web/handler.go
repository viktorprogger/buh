package web

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/bankaccount"
	"buh/internal/client"
	"buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/importer"
	"buh/internal/invitation"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/middleware"
	"buh/internal/sliprecord"
	webaccountant "buh/internal/web/accountant"
	webentrepreneur "buh/internal/web/entrepreneur"
	"buh/internal/web/shared"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// handler handles public routes: login, logout, invite accept, registration redirect, privacy, terms.
type handler struct {
	sessions          *auth.SessionManager
	accountants       *accountant.Repo
	entrepreneurUsers *entrepreneuruser.Repo
	invitations       *invitation.Repo
	entrepreneurs     *entrepreneur.Repo
	tmpl              shared.Templates
}

func (h *handler) renderError(w http.ResponseWriter, code int) {
	shared.RenderError(w, h.tmpl.ErrPage, code)
}

// NewHandler returns an HTTP handler for the web UI.
func NewHandler(accts *accountant.Repo, entrepreneurUsers *entrepreneuruser.Repo, sessions *auth.SessionManager, db *sql.DB) http.Handler {
	entrepreneurs := entrepreneur.NewRepo(db)
	slips := sliprecord.NewRepo(db)
	kpoBooks := kpo.NewRepo(db)
	clients := client.NewRepo(db)
	invoices := invoice.NewRepo(db)
	bankAccounts := bankaccount.NewRepo(db)
	invitations := invitation.NewRepo(db)
	tmpl := shared.ParseTemplates(templateFS)

	aH := webaccountant.NewHandler(
		sessions,
		entrepreneurs,
		slips,
		kpoBooks,
		importer.New(entrepreneurs, slips),
		entrepreneurUsers,
		invitations,
		invoices,
		tmpl,
	)
	eH := webentrepreneur.NewHandler(
		sessions,
		entrepreneurs,
		kpoBooks,
		clients,
		invoices,
		bankAccounts,
		entrepreneurUsers,
		invitations,
		tmpl,
	)

	h := &handler{
		sessions:          sessions,
		accountants:       accts,
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		entrepreneurs:     entrepreneurs,
		tmpl:              tmpl,
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)
	mux.HandleFunc("/privacy", h.handlePrivacy)
	mux.HandleFunc("/terms", h.handleTerms)

	// Public entrepreneur routes (no auth middleware).
	mux.HandleFunc("GET /e/register", eH.HandleRegisterForm)
	mux.HandleFunc("POST /e/register", eH.HandleRegisterSubmit)
	mux.HandleFunc("GET /invite/{token}", h.handleInviteToken)
	mux.HandleFunc("POST /invite/{token}/accept", h.handleInviteAccept)

	mux.Handle("/a/", http.StripPrefix("/a", middleware.RequireAccountant(sessions, aH.Routes())))
	mux.Handle("/e/", http.StripPrefix("/e", middleware.RequireEntrepreneur(sessions, eH.Routes())))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			h.renderError(w, http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	return mux
}

func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		userType := r.FormValue("user_type")

		renderErr := func() {
			shared.RenderTemplate(w, h.tmpl.Login, map[string]any{"Error": "Погрешна е-пошта или лозинка.", "UserType": userType})
		}

		if userType == "entrepreneur" {
			u, err := h.entrepreneurUsers.FindByEmail(r.Context(), email)
			if err == nil {
				err = entrepreneuruser.CheckPassword(u, password)
			}
			if errors.Is(err, entrepreneuruser.ErrNotFound) || errors.Is(err, entrepreneuruser.ErrInvalidCredentials) {
				renderErr()
				return
			}
			if err != nil {
				http.Error(w, "Грешка при пријави", http.StatusInternalServerError)
				return
			}
			if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeEntrepreneur, UserID: u.ID.String()}); err != nil {
				http.Error(w, "Грешка при постављању сесије", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, "/e/", http.StatusFound)
			return
		}

		a, err := h.accountants.FindByEmail(context.Background(), email)
		if err == nil {
			err = accountant.CheckPassword(a, password)
		}
		if errors.Is(err, accountant.ErrNotFound) || errors.Is(err, accountant.ErrInvalidCredentials) {
			renderErr()
			return
		}
		if err != nil {
			http.Error(w, "Грешка при пријави", http.StatusInternalServerError)
			return
		}
		if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeAccountant, UserID: a.ID}); err != nil {
			http.Error(w, "Грешка при постављању сесије", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/a/", http.StatusFound)
		return
	}
	shared.RenderTemplate(w, h.tmpl.Login, nil)
}

func (h *handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *handler) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, h.tmpl.Placeholder, map[string]any{"Title": "Политика приватности"})
}

func (h *handler) handleTerms(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, h.tmpl.Placeholder, map[string]any{"Title": "Услови коришћења"})
}

func (h *handler) handleInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := inv.Validate(); err != nil {
		shared.RenderTemplate(w, h.tmpl.InviteAccept, map[string]any{"Error": "Позивница је истекла или је већ искоришћена."})
		return
	}
	data := map[string]any{
		"Invitation": inv,
		"Token":      token,
	}
	if inv.ManagedEntrepreneurID != nil {
		me, err := h.entrepreneurs.FindByID(r.Context(), *inv.ManagedEntrepreneurID)
		if err == nil {
			data["ManagedEntrepreneur"] = me
		}
	}
	shared.RenderTemplate(w, h.tmpl.InviteAccept, data)
}

func (h *handler) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := inv.Validate(); err != nil {
		shared.RenderTemplate(w, h.tmpl.InviteAccept, map[string]any{"Error": "Позивница је истекла или је већ искоришћена."})
		return
	}

	sess, ok := h.sessions.Get(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if inv.InviterType == invitation.InviterTypeAccountant {
		if sess.UserType != auth.UserTypeEntrepreneur {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		entrepreneurUserID, _ := uuid.Parse(sess.UserID)
		managedID := *inv.ManagedEntrepreneurID

		if existing, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), entrepreneurUserID); existing.ID != uuid.Nil {
			shared.RenderTemplate(w, h.tmpl.InviteAccept, map[string]any{"Error": "Већ сте повезани са рачуновођом."})
			return
		}
		if err := h.entrepreneurs.Pair(r.Context(), managedID, entrepreneurUserID); err != nil {
			http.Error(w, "Грешка при повезивању", http.StatusInternalServerError)
			return
		}
		h.invitations.Accept(r.Context(), inv.ID)
		http.Redirect(w, r, "/e/", http.StatusFound)
		return
	}

	// Entrepreneur-initiated: accountant accepts.
	if sess.UserType != auth.UserTypeAccountant {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	managedIDStr := strings.TrimSpace(r.FormValue("managed_entrepreneur_id"))
	accountantID, _ := uuid.Parse(sess.UserID)
	entrepreneurUserID := inv.InviterID

	var managedID uuid.UUID
	if managedIDStr == "" || managedIDStr == "new" {
		u, err := h.entrepreneurUsers.FindByID(r.Context(), entrepreneurUserID)
		name := "Предузетник"
		if err == nil && u.Email != "" {
			name = u.Email
		}
		e, _, err := h.entrepreneurs.FindOrCreate(r.Context(), accountantID, "0000000000", name)
		if err != nil {
			http.Error(w, "Грешка при креирању предузетника", http.StatusInternalServerError)
			return
		}
		managedID = e.ID
	} else {
		if managedID, err = uuid.Parse(managedIDStr); err != nil {
			h.renderError(w, http.StatusBadRequest)
			return
		}
	}
	if err := h.entrepreneurs.Pair(r.Context(), managedID, entrepreneurUserID); err != nil {
		http.Error(w, "Грешка при повезивању", http.StatusInternalServerError)
		return
	}
	h.invitations.Accept(r.Context(), inv.ID)
	http.Redirect(w, r, "/a/entrepreneurs/"+managedID.String(), http.StatusFound)
}
