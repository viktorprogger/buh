package web

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/bankaccount"
	"buh/internal/client"
	"buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/i18n"
	"buh/internal/invitation"
	"buh/internal/uploadqueue"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/middleware"
	"buh/internal/sliphistory"
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

func (h *handler) renderError(w http.ResponseWriter, r *http.Request, code int) {
	shared.RenderError(w, r, h.tmpl.ErrPage, code)
}

// langMiddleware resolves the active locale for every request and injects it into the context.
// Priority: lang cookie > Accept-Language header.
// On login success the cookie is set to the user's DB preference (see handleLogin).
func langMiddleware(bundle *i18n.Bundle) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lang := i18n.Detect(r, "")
			l := bundle.NewLocalizer(lang)
			r = i18n.WithLocalizer(r, l)
			next.ServeHTTP(w, r)
		})
	}
}

// NewHandler returns an HTTP handler for the web UI.
func NewHandler(accts *accountant.Repo, entrepreneurUsers *entrepreneuruser.Repo, sessions *auth.SessionManager, db *sql.DB) http.Handler {
	entrepreneurs := entrepreneur.NewRepo(db)
	slips := sliprecord.NewRepo(db)
	slipHistory := sliphistory.NewRepo(db)
	kpoBooks := kpo.NewRepo(db)
	clients := client.NewRepo(db)
	invoices := invoice.NewRepo(db)
	bankAccounts := bankaccount.NewRepo(db)
	invitations := invitation.NewRepo(db)
	tmpl := shared.ParseTemplates(templateFS)
	bundle := i18n.NewBundle()

	uploadQueue := uploadqueue.NewRepo(db)
	aH := webaccountant.NewHandler(
		sessions,
		entrepreneurs,
		slips,
		slipHistory,
		kpoBooks,
		uploadQueue,
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
	mux.HandleFunc("POST /language", h.handleSetLanguage)
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)
	mux.HandleFunc("/privacy", h.handlePrivacy)
	mux.HandleFunc("/terms", h.handleTerms)
	mux.HandleFunc("/info/pausal-limit", h.handlePausalLimitInfo)
	mux.HandleFunc("/info/vat-limit", h.handleVATLimitInfo)

	// Public entrepreneur routes (no auth middleware).
	mux.HandleFunc("GET /e/register", eH.HandleRegisterForm)
	mux.HandleFunc("POST /e/register", eH.HandleRegisterSubmit)
	mux.HandleFunc("GET /invite/{token}", h.handleInviteToken)
	mux.HandleFunc("POST /invite/{token}/accept", h.handleInviteAccept)

	mux.Handle("/a/", http.StripPrefix("/a", middleware.RequireAccountant(sessions, aH.Routes())))
	mux.Handle("/e/", http.StripPrefix("/e", middleware.RequireEntrepreneur(sessions, eH.Routes())))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			h.renderError(w, r, http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	return langMiddleware(bundle)(mux)
}

// handleSetLanguage handles POST /language. Sets the lang cookie and, if the user
// is logged in, persists the preference to the database.
func (h *handler) handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	lang := r.FormValue("lang")
	if lang != "sr" && lang != "en" && lang != "ru" {
		lang = "sr"
	}
	http.SetCookie(w, &http.Cookie{
		Name:    i18n.LangCookie,
		Value:   lang,
		Path:    "/",
		MaxAge:  365 * 24 * 60 * 60,
		Expires: time.Now().Add(365 * 24 * time.Hour),
	})
	if sess, ok := h.sessions.Get(r); ok {
		if sess.UserType == auth.UserTypeAccountant {
			_ = h.accountants.SetLanguage(r.Context(), sess.UserID, lang)
		} else if sess.UserType == auth.UserTypeEntrepreneur {
			if id, err := uuid.Parse(sess.UserID); err == nil {
				_ = h.entrepreneurUsers.SetLanguage(r.Context(), id, lang)
			}
		}
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}

func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		userType := r.FormValue("user_type")

		l := i18n.FromContext(r.Context())
		renderErr := func() {
			shared.RenderTemplate(w, r, h.tmpl.Login, map[string]any{"Error": l.T("login.error_invalid_credentials"), "UserType": userType})
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
				http.Error(w, l.T("login.error_server"), http.StatusInternalServerError)
				return
			}
			if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeEntrepreneur, UserID: u.ID.String()}); err != nil {
				http.Error(w, l.T("login.error_session"), http.StatusInternalServerError)
				return
			}
			// Persist the user's language preference to cookie.
			if u.Language != "" {
				http.SetCookie(w, &http.Cookie{
					Name:    i18n.LangCookie,
					Value:   u.Language,
					Path:    "/",
					MaxAge:  365 * 24 * 60 * 60,
					Expires: time.Now().Add(365 * 24 * time.Hour),
				})
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
			http.Error(w, l.T("login.error_server"), http.StatusInternalServerError)
			return
		}
		if err := h.sessions.Set(w, auth.Session{UserType: auth.UserTypeAccountant, UserID: a.ID}); err != nil {
			http.Error(w, l.T("login.error_session"), http.StatusInternalServerError)
			return
		}
		// Persist the accountant's language preference to cookie.
		if a.Language != "" {
			http.SetCookie(w, &http.Cookie{
				Name:    i18n.LangCookie,
				Value:   a.Language,
				Path:    "/",
				MaxAge:  365 * 24 * 60 * 60,
				Expires: time.Now().Add(365 * 24 * time.Hour),
			})
		}
		http.Redirect(w, r, "/a/", http.StatusFound)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.Login, nil)
}

func (h *handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (h *handler) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromContext(r.Context())
	shared.RenderTemplate(w, r, h.tmpl.Placeholder, map[string]any{"Title": l.T("footer.privacy")})
}

func (h *handler) handleTerms(w http.ResponseWriter, r *http.Request) {
	l := i18n.FromContext(r.Context())
	shared.RenderTemplate(w, r, h.tmpl.Placeholder, map[string]any{"Title": l.T("footer.terms")})
}

func (h *handler) handlePausalLimitInfo(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, r, h.tmpl.PausalLimitInfo, nil)
}

func (h *handler) handleVATLimitInfo(w http.ResponseWriter, r *http.Request) {
	shared.RenderTemplate(w, r, h.tmpl.VATLimitInfo, nil)
}

func (h *handler) handleInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err := inv.Validate(); err != nil {
		l := i18n.FromContext(r.Context())
		shared.RenderTemplate(w, r, h.tmpl.InviteAccept, map[string]any{"Error": l.T("invite_accept.expired")})
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
	shared.RenderTemplate(w, r, h.tmpl.InviteAccept, data)
}

func (h *handler) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	l := i18n.FromContext(r.Context())
	if err := inv.Validate(); err != nil {
		shared.RenderTemplate(w, r, h.tmpl.InviteAccept, map[string]any{"Error": l.T("invite_accept.expired")})
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
			shared.RenderTemplate(w, r, h.tmpl.InviteAccept, map[string]any{"Error": l.T("invite_accept.already_paired")})
			return
		}
		if err := h.entrepreneurs.Pair(r.Context(), managedID, entrepreneurUserID); err != nil {
			http.Error(w, l.T("invite_accept.error_pairing"), http.StatusInternalServerError)
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
		name := l.T("invite_accept.default_entrepreneur_name")
		if err == nil && u.Email != "" {
			name = u.Email
		}
		e, _, err := h.entrepreneurs.FindOrCreate(r.Context(), accountantID, "0000000000", name)
		if err != nil {
			http.Error(w, l.T("invite_accept.error_create_entrepreneur"), http.StatusInternalServerError)
			return
		}
		managedID = e.ID
	} else {
		if managedID, err = uuid.Parse(managedIDStr); err != nil {
			h.renderError(w, r, http.StatusBadRequest)
			return
		}
	}
	if err := h.entrepreneurs.Pair(r.Context(), managedID, entrepreneurUserID); err != nil {
		http.Error(w, l.T("invite_accept.error_pairing"), http.StatusInternalServerError)
		return
	}
	h.invitations.Accept(r.Context(), inv.ID)
	http.Redirect(w, r, "/a/entrepreneurs/"+managedID.String(), http.StatusFound)
}
