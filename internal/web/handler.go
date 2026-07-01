package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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
	"buh/internal/ips"
	"buh/internal/kpo"
	"buh/internal/middleware"
	"buh/internal/slip"
	"buh/internal/sliprecord"
)

const maxUploadFiles = 4

// pausalalLimitForYear returns the paušal turnover limit in RSD for the given year.
func pausalalLimitForYear(year int) int64 {
	if year >= 2027 {
		return 8_000_000
	}
	return 6_000_000
}

// formatIntWithSpaces formats n with space-separated thousands (Serbian convention).
func formatIntWithSpaces(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	offset := len(s) % 3
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (i-offset)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, s[i])
	}
	return string(out)
}

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type templates struct {
	login             *template.Template
	index             *template.Template
	entrepreneur      *template.Template
	entrepreneurNew   *template.Template
	results           *template.Template
	slip              *template.Template
	slipNew           *template.Template
	placeholder       *template.Template
	errPage           *template.Template
	settings          *template.Template
	clientForm        *template.Template
	bankAccountForm   *template.Template
	correspondentForm *template.Template
	invoiceNew        *template.Template
	invoiceDetail     *template.Template
	// Entrepreneur-side templates
	entrepreneurRegister     *template.Template
	entrepreneurDashboard    *template.Template
	inviteToken              *template.Template
	inviteAccept             *template.Template
	entrepreneurInviteAcc    *template.Template
	kpoMerge                 *template.Template
	entrepreneurKPO          *template.Template
	entrepreneurClients      *template.Template
	entrepreneurBankAccounts *template.Template
	entrepreneurInvoices     *template.Template
}

// mustPageTmpl parses base.html + a page template (and optional extras) into a single template set.
// The root template is named after the page file; Execute renders the full page via {{template "base" .}}.
func mustPageTmpl(name string, funcs template.FuncMap, extra ...string) *template.Template {
	t := template.New(name)
	if funcs != nil {
		t = t.Funcs(funcs)
	}
	files := append([]string{"templates/base.html", "templates/" + name}, extra...)
	return template.Must(t.ParseFS(templateFS, files...))
}

func parseTemplates() templates {
	slipExtra := []string{"templates/slip_styles.html"}
	entrepreneurFuncs := template.FuncMap{
		"slipTableArgs": func(entrepreneurID uuid.UUID, slips []sliprecord.SlipRecord) slipTableData {
			return slipTableData{EntrepreneurID: entrepreneurID, Slips: slips}
		},
	}
	return templates{
		login:             template.Must(template.New("login.html").ParseFS(templateFS, "templates/login.html")),
		index:             mustPageTmpl("index.html", nil),
		entrepreneur:      mustPageTmpl("entrepreneur.html", entrepreneurFuncs, "templates/slip_table.html"),
		entrepreneurNew:   mustPageTmpl("entrepreneur_new.html", nil),
		results:           mustPageTmpl("results.html", nil),
		slip:              mustPageTmpl("slip.html", nil, slipExtra...),
		slipNew:           mustPageTmpl("slip_new.html", nil, slipExtra...),
		placeholder:       mustPageTmpl("placeholder.html", nil),
		errPage:           mustPageTmpl("error.html", nil),
		settings:          mustPageTmpl("settings.html", nil),
		clientForm:        mustPageTmpl("client_form.html", nil),
		bankAccountForm:   mustPageTmpl("bank_account_form.html", nil),
		correspondentForm: mustPageTmpl("correspondent_form.html", nil),
		invoiceNew:        mustPageTmpl("invoice_new.html", nil),
		invoiceDetail:     mustPageTmpl("invoice.html", nil),
		// Entrepreneur-side templates
		entrepreneurRegister:     template.Must(template.New("entrepreneur_register.html").ParseFS(templateFS, "templates/entrepreneur_register.html")),
		entrepreneurDashboard:    mustPageTmpl("entrepreneur_dashboard.html", nil),
		inviteToken:              mustPageTmpl("invite_token.html", nil),
		inviteAccept:             mustPageTmpl("invite_accept.html", nil),
		entrepreneurInviteAcc:    mustPageTmpl("entrepreneur_invite_accountant.html", nil),
		kpoMerge:                 mustPageTmpl("kpo_merge.html", template.FuncMap{"add": func(a, b int) int { return a + b }}),
		entrepreneurKPO:          mustPageTmpl("entrepreneur_kpo.html", nil),
		entrepreneurClients:      mustPageTmpl("entrepreneur_clients.html", nil),
		entrepreneurBankAccounts: mustPageTmpl("entrepreneur_bank_accounts.html", nil),
		entrepreneurInvoices:     mustPageTmpl("entrepreneur_invoices.html", nil),
	}
}

// kpoRow is a display-model row for the KPO table: either a regular KPO entry
// or an advance invoice (shown muted, not counted in totals).
type kpoRow struct {
	IsAdvanceInvoice bool
	Date             time.Time
	InvoiceNum       string
	// Regular KPO entry fields:
	EntryID        uuid.UUID
	OrdinalNumber  int
	ProductRevenue float64
	ServiceRevenue float64
	EntryTotal     float64
	// Advance invoice fields:
	InvoiceID  uuid.UUID
	ClientName string
	TotalRSD   float64
}

type handler struct {
	accountants       *accountant.Repo
	sessions          *auth.SessionManager
	entrepreneurs     *entrepreneur.Repo
	slips             *sliprecord.Repo
	kpoBooks          *kpo.Repo
	clients           *client.Repo
	invoices          *invoice.Repo
	bankAccounts      *bankaccount.Repo
	importer          *importer.Importer
	entrepreneurUsers *entrepreneuruser.Repo
	invitations       *invitation.Repo
	tmpl              templates
}

func (h *handler) renderError(w http.ResponseWriter, code int) {
	type errData struct {
		Code    int
		Title   string
		Message string
	}
	data := map[int]errData{
		http.StatusNotFound:  {404, "Страница није пронађена", "Ресурс који тражите не постоји или је премештен."},
		http.StatusForbidden: {403, "Приступ забрањен", "Немате дозволу да приступите овом ресурсу."},
	}
	d, ok := data[code]
	if !ok {
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.WriteHeader(code)
	renderTemplate(w, h.tmpl.errPage, d)
}

// NewHandler returns an HTTP handler for the web UI.
func NewHandler(accountants *accountant.Repo, entrepreneurUsers *entrepreneuruser.Repo, sessions *auth.SessionManager, db *sql.DB) http.Handler {
	entrepreneurs := entrepreneur.NewRepo(db)
	slips := sliprecord.NewRepo(db)
	kpoBooks := kpo.NewRepo(db)
	clients := client.NewRepo(db)
	invoices := invoice.NewRepo(db)
	bankAccounts := bankaccount.NewRepo(db)
	invitations := invitation.NewRepo(db)
	h := &handler{
		accountants:       accountants,
		sessions:          sessions,
		entrepreneurs:     entrepreneurs,
		slips:             slips,
		kpoBooks:          kpoBooks,
		clients:           clients,
		invoices:          invoices,
		bankAccounts:      bankAccounts,
		importer:          importer.New(entrepreneurs, slips),
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		tmpl:              parseTemplates(),
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)
	mux.HandleFunc("/privacy", h.handlePrivacy)
	mux.HandleFunc("/terms", h.handleTerms)

	// Public routes for entrepreneur registration and invite acceptance.
	mux.HandleFunc("GET /e/register", h.handleEntrepreneurRegisterForm)
	mux.HandleFunc("POST /e/register", h.handleEntrepreneurRegisterSubmit)
	mux.HandleFunc("GET /invite/{token}", h.handleInviteToken)
	mux.HandleFunc("POST /invite/{token}/accept", h.handleInviteAccept)

	// Accountant-protected routes under /a/.
	aMux := http.NewServeMux()
	aMux.HandleFunc("POST /process", h.handleProcess)
	aMux.HandleFunc("GET /entrepreneurs/new", h.handleEntrepreneurNewForm)
	aMux.HandleFunc("POST /entrepreneurs/new", h.handleEntrepreneurNewSubmit)
	aMux.HandleFunc("GET /entrepreneurs/{id}", h.handleEntrepreneur)
	aMux.HandleFunc("POST /entrepreneurs/{id}", h.handleEntrepreneurUpdate)
	aMux.HandleFunc("GET /entrepreneurs/{id}/settings", h.handleSettings)
	aMux.HandleFunc("POST /entrepreneurs/{id}/settings", h.handleSettingsUpdate)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries", h.handleKPOAddEntry)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/reorder", h.handleKPOReorderEntries)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/update", h.handleKPOUpdateEntry)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/delete", h.handleKPODeleteEntry)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/finalize", h.handleKPOFinalize)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/unfinalize", h.handleKPOUnfinalize)
	aMux.HandleFunc("GET /entrepreneurs/{id}/kpo/{year}/merge", h.handleKPOMergeView)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/merge/copy/{eid}", h.handleKPOMergeCopyEntry)
	aMux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/merge/done", h.handleKPOMergeMarkDone)
	aMux.HandleFunc("GET /entrepreneurs/{id}/slips/new", h.handleSlipNewForm)
	aMux.HandleFunc("POST /entrepreneurs/{id}/slips/new", h.handleSlipNewSubmit)
	aMux.HandleFunc("POST /entrepreneurs/{id}/invite", h.handleAccountantInviteEntrepreneur)
	aMux.HandleFunc("GET /slips/{id}", h.handleSlip)
	aMux.HandleFunc("POST /slips/{id}/save", h.handleSlipSave)
	aMux.HandleFunc("POST /slips/{id}/download", h.handleSlipDownload)
	aMux.HandleFunc("POST /slips/{id}/delete", h.handleSlipDelete)
	aMux.HandleFunc("GET /slips/{id}/pdf", h.handleSlipPDF)
	aMux.HandleFunc("/", h.handleAccountantIndex)
	mux.Handle("/a/", http.StripPrefix("/a", middleware.RequireAccountant(sessions, aMux)))

	// Entrepreneur-protected routes under /e/.
	eMux := http.NewServeMux()
	eMux.HandleFunc("/", h.handleEntrepreneurDashboard)
	eMux.HandleFunc("GET /invite-accountant", h.handleEntrepreneurInviteForm)
	eMux.HandleFunc("POST /invite-accountant", h.handleEntrepreneurSendInvite)
	eMux.HandleFunc("GET /clients", h.handleEClientList)
	eMux.HandleFunc("GET /clients/search", h.handleEClientSearch)
	eMux.HandleFunc("GET /clients/new", h.handleEClientNewForm)
	eMux.HandleFunc("POST /clients/new", h.handleEClientNewSubmit)
	eMux.HandleFunc("GET /clients/{cid}/edit", h.handleEClientEditForm)
	eMux.HandleFunc("POST /clients/{cid}", h.handleEClientUpdate)
	eMux.HandleFunc("POST /clients/{cid}/delete", h.handleEClientDelete)
	eMux.HandleFunc("GET /bank-accounts", h.handleEBankAccountList)
	eMux.HandleFunc("GET /bank-accounts/new", h.handleEBankAccountNewForm)
	eMux.HandleFunc("POST /bank-accounts/new", h.handleEBankAccountCreate)
	eMux.HandleFunc("GET /bank-accounts/{aid}/edit", h.handleEBankAccountEditForm)
	eMux.HandleFunc("POST /bank-accounts/{aid}", h.handleEBankAccountUpdate)
	eMux.HandleFunc("POST /bank-accounts/{aid}/delete", h.handleEBankAccountDelete)
	eMux.HandleFunc("GET /bank-accounts/{aid}/correspondents/new", h.handleECorrespondentNewForm)
	eMux.HandleFunc("POST /bank-accounts/{aid}/correspondents/new", h.handleECorrespondentCreate)
	eMux.HandleFunc("GET /bank-accounts/{aid}/correspondents/{cid}/edit", h.handleECorrespondentEditForm)
	eMux.HandleFunc("POST /bank-accounts/{aid}/correspondents/{cid}", h.handleECorrespondentUpdate)
	eMux.HandleFunc("POST /bank-accounts/{aid}/correspondents/{cid}/delete", h.handleECorrespondentDelete)
	eMux.HandleFunc("GET /bank-accounts/{aid}/correspondents", h.handleECorrespondentsByAccount)
	eMux.HandleFunc("GET /invoices", h.handleEInvoiceList)
	eMux.HandleFunc("GET /invoices/new", h.handleEInvoiceNewForm)
	eMux.HandleFunc("POST /invoices", h.handleEInvoiceCreate)
	eMux.HandleFunc("GET /invoices/{iid}", h.handleEInvoice)
	eMux.HandleFunc("GET /invoices/{iid}/pdf", h.handleEInvoicePDF)
	eMux.HandleFunc("GET /kpo/{year}", h.handleEKPO)
	eMux.HandleFunc("POST /kpo/{year}/entries", h.handleEKPOAddEntry)
	eMux.HandleFunc("POST /kpo/{year}/entries/reorder", h.handleEKPOReorderEntries)
	eMux.HandleFunc("POST /kpo/{year}/entries/{entryID}/update", h.handleEKPOUpdateEntry)
	eMux.HandleFunc("POST /kpo/{year}/entries/{entryID}/delete", h.handleEKPODeleteEntry)
	eMux.HandleFunc("POST /kpo/{year}/finalize", h.handleEKPOFinalize)
	eMux.HandleFunc("POST /kpo/{year}/unfinalize", h.handleEKPOUnfinalize)
	mux.Handle("/e/", http.StripPrefix("/e", middleware.RequireEntrepreneur(sessions, eMux)))

	// Legacy redirect: bare / goes to login (or could detect session and redirect).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			h.renderError(w, http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	return mux
}

// handleLogin GET → login form, POST → try accountant then entrepreneur credentials.
func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		userType := r.FormValue("user_type") // "accountant" or "entrepreneur"

		renderErr := func() {
			renderTemplate(w, h.tmpl.login, map[string]any{"Error": "Погрешна е-пошта или лозинка.", "UserType": userType})
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

		// Default: accountant login.
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
	renderTemplate(w, h.tmpl.login, nil)
}

// handleLogout clears the session and redirects to /login.
func (h *handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// accountantFromSession returns the accountant UUID from the current session.
func (h *handler) accountantFromSession(r *http.Request) (uuid.UUID, bool) {
	sess, ok := h.sessions.Get(r)
	if !ok || sess.UserType != auth.UserTypeAccountant {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(sess.UserID)
	return id, err == nil
}

// entrepreneurUserFromSession returns the entrepreneur_user UUID from the current session.
func (h *handler) entrepreneurUserFromSession(r *http.Request) (uuid.UUID, bool) {
	sess, ok := h.sessions.Get(r)
	if !ok || sess.UserType != auth.UserTypeEntrepreneur {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(sess.UserID)
	return id, err == nil
}

// findOwnedEntrepreneur loads an entrepreneur by ID and returns 403 if it doesn't belong
// to the logged-in accountant. Reports the error via w and returns false on any failure.
func (h *handler) findOwnedEntrepreneur(w http.ResponseWriter, r *http.Request, id uuid.UUID) (entrepreneur.Entrepreneur, bool) {
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		h.renderError(w, http.StatusForbidden)
		return entrepreneur.Entrepreneur{}, false
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		h.renderError(w, http.StatusNotFound)
		return entrepreneur.Entrepreneur{}, false
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return entrepreneur.Entrepreneur{}, false
	}
	if e.AccountantID != accountantID {
		h.renderError(w, http.StatusForbidden)
		return entrepreneur.Entrepreneur{}, false
	}
	return e, true
}

// findOwnedSlip loads a slip by ID and returns 403 if the slip's entrepreneur doesn't
// belong to the logged-in accountant. Reports the error via w and returns false on any failure.
func (h *handler) findOwnedSlip(w http.ResponseWriter, r *http.Request, id uuid.UUID) (sliprecord.SlipRecord, bool) {
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		h.renderError(w, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	s, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		h.renderError(w, http.StatusNotFound)
		return sliprecord.SlipRecord{}, false
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), s.EntrepreneurID)
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return sliprecord.SlipRecord{}, false
	}
	if e.AccountantID != accountantID {
		h.renderError(w, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	return s, true
}

// handleAccountantIndex lists all managed entrepreneurs for the logged-in accountant.
func (h *handler) handleAccountantIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.renderError(w, http.StatusNotFound)
		return
	}
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	entrepreneurs, err := h.entrepreneurs.ListByAccountant(context.Background(), accountantID)
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, h.tmpl.index, map[string]any{
		"Entrepreneurs": entrepreneurs,
	})
}

// handleEntrepreneur shows the entrepreneur page with KPO for the selected year and slip list.
// The ?year= query param selects the year; defaults to the current year. The KPO book is
// not persisted until the first entry is added.
func (h *handler) handleEntrepreneur(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}

	selectedYear := time.Now().Year()
	if ys := r.URL.Query().Get("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}

	currentBook, err := h.kpoBooks.FindByYear(r.Context(), id, selectedYear)
	if err != nil && !errors.Is(err, kpo.ErrNotFound) {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	var entries []kpo.Entry
	if currentBook.ID != (uuid.UUID{}) {
		entries, err = h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
		if err != nil {
			http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
			return
		}
	}

	books, err := h.kpoBooks.ListByManagedEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}

	// Advance invoices come from the paired entrepreneur_user (if any).
	var advanceInvoices []invoice.Invoice
	if e.EntrepreneurUserID != nil {
		advanceInvoices, err = h.invoices.ListAdvanceByEntrepreneurUserYear(r.Context(), *e.EntrepreneurUserID, selectedYear)
		if err != nil {
			http.Error(w, "Грешка при учитавању авансних фактура", http.StatusInternalServerError)
			return
		}
	}

	// Build unified KPO display rows (regular entries + advance invoices), sorted by date.
	var kpoRows []kpoRow
	for i, e := range entries {
		kpoRows = append(kpoRows, kpoRow{
			IsAdvanceInvoice: false,
			Date:             e.CollectionDate,
			InvoiceNum:       e.InvoiceNumber,
			EntryID:          e.ID,
			OrdinalNumber:    i + 1,
			ProductRevenue:   e.ProductRevenue,
			ServiceRevenue:   e.ServiceRevenue,
			EntryTotal:       e.Total(),
		})
	}
	for _, inv := range advanceInvoices {
		kpoRows = append(kpoRows, kpoRow{
			IsAdvanceInvoice: true,
			Date:             inv.IssueDate,
			InvoiceNum:       inv.InvoiceNumber,
			InvoiceID:        inv.ID,
			ClientName:       inv.ClientName,
			TotalRSD:         inv.TotalRSD,
		})
	}
	sort.Slice(kpoRows, func(i, j int) bool {
		return kpoRows[i].Date.Before(kpoRows[j].Date)
	})

	slips, err := h.slips.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатница", http.StatusInternalServerError)
		return
	}
	latestSlipYear, prevSlipYears := groupSlipsByYear(slips)

	var totalProduct, totalService float64
	for _, en := range entries {
		totalProduct += en.ProductRevenue
		totalService += en.ServiceRevenue
	}

	currentYear := time.Now().Year()
	pausalalLimit := pausalalLimitForYear(currentYear)
	var pausalalTotal float64
	var pausalalHasData bool
	if selectedYear == currentYear {
		pausalalTotal = totalProduct + totalService
		pausalalHasData = len(entries) > 0
	} else {
		pausalalTotal, pausalalHasData, err = h.kpoBooks.SumForYear(r.Context(), id, currentYear)
		if err != nil {
			http.Error(w, "Грешка при учитавању паушалног прага", http.StatusInternalServerError)
			return
		}
	}
	var pausalalPercent float64
	if pausalalLimit > 0 {
		pausalalPercent = pausalalTotal / float64(pausalalLimit) * 100
	}

	renderTemplate(w, h.tmpl.entrepreneur, map[string]any{
		"Entrepreneur":     e,
		"KPOBooks":         books,
		"CurrentBook":      currentBook,
		"KPOEntries":       entries,
		"KPORows":          kpoRows,
		"SelectedYear":     selectedYear,
		"TotalProduct":     totalProduct,
		"TotalService":     totalService,
		"TotalAll":         totalProduct + totalService,
		"LatestSlipYear":   latestSlipYear,
		"PrevSlipYears":    prevSlipYears,
		"PausalalYear":     currentYear,
		"PausalalHasData":  pausalalHasData,
		"PausalalTotalFmt": formatIntWithSpaces(int64(math.Round(pausalalTotal))),
		"PausalalLimitFmt": formatIntWithSpaces(pausalalLimit),
		"PausalalPercent":  pausalalPercent,
	})
}

// kpoBookFromPath resolves the entrepreneur ID and year from path values, verifies the entrepreneur
// exists (and belongs to the logged-in accountant), and returns the KPO book.
func (h *handler) kpoBookFromPath(r *http.Request) (entrepreneur.Entrepreneur, kpo.Book, int, error) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("bad id")
	}
	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("forbidden")
	}
	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if err != nil {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, err
	}
	if e.AccountantID != accountantID {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("forbidden")
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, errors.New("bad year")
	}
	book, err := h.kpoBooks.FindOrCreateForAccountant(r.Context(), id, year)
	return e, book, year, err
}

// parseDayMonth parses a "dd.mm" string combined with the given year into a time.Time.
func parseDayMonth(s string, year int) (time.Time, error) {
	return time.Parse("02.01.2006", strings.TrimSpace(s)+"."+strconv.Itoa(year))
}

func (h *handler) handleKPOAddEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}

	r.ParseForm()
	collectionDate, err := parseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум (очекује се дд.мм)", http.StatusBadRequest)
		return
	}
	productRev := round2(parseAmount(r.FormValue("product_revenue")))
	serviceRev := round2(parseAmount(r.FormValue("service_revenue")))

	_, err = h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      book.ID,
		CollectionDate: collectionDate,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: productRev,
		ServiceRevenue: serviceRev,
	})
	if err != nil {
		http.Error(w, "Грешка при уносу ставке", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d#kpo-new", r.PathValue("id"), year), http.StatusFound)
}

func (h *handler) handleKPOUpdateEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}

	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	r.ParseForm()
	collectionDate, err := parseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум (очекује се дд.мм)", http.StatusBadRequest)
		return
	}
	productRev := round2(parseAmount(r.FormValue("product_revenue")))
	serviceRev := round2(parseAmount(r.FormValue("service_revenue")))

	if err = h.kpoBooks.UpdateEntry(r.Context(), kpo.Entry{
		ID:             entryID,
		KPOBookID:      book.ID,
		CollectionDate: collectionDate,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: productRev,
		ServiceRevenue: serviceRev,
	}); err != nil {
		http.Error(w, "Грешка при измени ставке", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d#kpo", r.PathValue("id"), year), http.StatusFound)
}

func (h *handler) handleKPOReorderEntries(w http.ResponseWriter, r *http.Request) {
	_, book, _, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}

	if err = r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	rawIDs := r.Form["ids"]
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, s := range rawIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			http.Error(w, "Неисправан ID", http.StatusBadRequest)
			return
		}
		ids = append(ids, id)
	}

	if err = h.kpoBooks.ReorderEntries(r.Context(), book.ID, ids); err != nil {
		http.Error(w, "Грешка при преуређивању", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) handleKPODeleteEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		h.renderError(w, http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.DeleteEntry(r.Context(), book.ID, entryID); err != nil {
		http.Error(w, "Грешка при брисању ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

func (h *handler) handleKPOFinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Finalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при укњижавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

func (h *handler) handleKPOUnfinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Unfinalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при откључавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/a/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
}

// handleEntrepreneurNewForm renders the manual entrepreneur creation form.
func (h *handler) handleEntrepreneurNewForm(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, h.tmpl.entrepreneurNew, nil)
}

// handleEntrepreneurNewSubmit validates and creates a new entrepreneur, then redirects.
func (h *handler) handleEntrepreneurNewSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	pib := strings.TrimSpace(r.FormValue("pib"))

	if name == "" || pib == "" {
		renderTemplate(w, h.tmpl.entrepreneurNew, map[string]any{
			"Error": "Оба поља су обавезна.",
			"Name":  name,
			"PIB":   pib,
		})
		return
	}

	accountantID, ok := h.accountantFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	e, _, err := h.entrepreneurs.FindOrCreate(context.Background(), accountantID, pib, name)
	if err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String(), http.StatusFound)
}

// handleSlip shows a saved slip's details and a re-download button.
func (h *handler) handleSlip(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}

	renderTemplate(w, h.tmpl.slip, map[string]any{
		"Slip":        s,
		"Saved":       r.URL.Query().Get("saved") == "1",
		"Downloading": r.URL.Query().Get("download") == "1",
	})
}

// slipFormToRecord fills editable SlipRecord fields from the posted form.
// It assumes r.ParseForm() has already been called.
func slipFormToRecord(r *http.Request, existing sliprecord.SlipRecord) sliprecord.SlipRecord {
	rawAccount := strings.ReplaceAll(strings.TrimSpace(r.FormValue("R")), "-", "")
	existing.Payer = strings.TrimSpace(r.FormValue("P"))
	existing.Purpose = strings.TrimSpace(r.FormValue("S"))
	existing.Payee = strings.TrimSpace(r.FormValue("N"))
	existing.PayeeAccount = rawAccount
	existing.Reference = strings.TrimSpace(r.FormValue("RO"))
	existing.PaymentCode = strings.TrimSpace(r.FormValue("SF"))
	existing.Amount = formatAmount(r.FormValue("amount"))
	existing.Currency = r.FormValue("currency")
	if y, err := strconv.Atoi(r.FormValue("year")); err == nil && y > 0 {
		existing.Year = y
	}
	existing.Advance = r.FormValue("advance") == "on"
	return existing
}

// handleSlipSave saves editable field values to DB and redirects to slip detail with ?saved=1.
func (h *handler) handleSlipSave(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}

	r.ParseForm()
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1", http.StatusFound)
}

// handleSlipDownload saves editable field values, then redirects to slip detail with ?saved=1&download=1.
func (h *handler) handleSlipDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	existing, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}

	r.ParseForm()
	updated := slipFormToRecord(r, existing)
	if err := h.slips.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/slips/"+idStr+"?saved=1&download=1", http.StatusFound)
}

// handleSlipDelete deletes a slip record and redirects to the entrepreneur page.
func (h *handler) handleSlipDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	s, ok := h.findOwnedSlip(w, r, id)
	if !ok {
		return
	}
	entrepreneurID := s.EntrepreneurID

	if err := h.slips.Delete(r.Context(), id); err != nil {
		http.Error(w, "Грешка при брисању уплатнице", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+entrepreneurID.String(), http.StatusFound)
}

// handleSlipPDF generates the PDF from the current SlipRecord fields and streams it as an attachment.
func (h *handler) handleSlipPDF(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
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
		http.Error(w, "Грешка при креирању фајла", http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		http.Error(w, fmt.Sprintf("Грешка при генерисању PDF: %v", err), http.StatusInternalServerError)
		return
	}

	pdfBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		http.Error(w, "Грешка при читању PDF", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("uplatnica-%s.pdf", sanitizeFilename(pay.N))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(pdfBytes)
}

// slipYearGroup groups slip records for a single tax year.
type slipYearGroup struct {
	Year    int
	Regular []sliprecord.SlipRecord
	Advance []sliprecord.SlipRecord
}

// slipTableData is the dot passed into the "slip_table" sub-template.
type slipTableData struct {
	EntrepreneurID uuid.UUID
	Slips          []sliprecord.SlipRecord
}

// slipNewForm holds form values re-rendered on validation error.
type slipNewForm struct {
	S        string // purpose
	N        string // payee name
	R        string // payee account
	RO       string // reference
	SF       string // payment code
	Amount   string
	Currency string
	P        string // payer name
	Year     int
	Advance  bool
}

// handleSlipNewForm renders the manual slip creation form.
func (h *handler) handleSlipNewForm(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}

	renderTemplate(w, h.tmpl.slipNew, map[string]any{
		"Entrepreneur": e,
		"Form":         slipNewForm{SF: "253", Currency: "RSD", P: e.Name, Year: time.Now().Year()},
	})
}

// handleSlipNewSubmit builds an IPS payment from form values, generates a PDF,
// saves a SlipRecord, and redirects to the slip detail page.
func (h *handler) handleSlipNewSubmit(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
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
	form := slipNewForm{
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

	renderErr := func(msg string) {
		renderTemplate(w, h.tmpl.slipNew, map[string]any{
			"Entrepreneur": e,
			"Form":         form,
			"Error":        msg,
		})
	}

	if form.S == "" || form.N == "" || form.R == "" || form.SF == "" || form.Amount == "" {
		renderErr("Молимо попуните сва обавезна поља.")
		return
	}

	// Strip dashes from account number if entered in display format XXX-XXXXXXXXXXXXX-XX.
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
		http.Error(w, "Грешка при креирању фајла", http.StatusInternalServerError)
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		renderErr(fmt.Sprintf("Грешка при генерисању PDF: %v", err))
		return
	}

	rec := sliprecord.SlipRecord{
		EntrepreneurID: id,
		PaymentCode:    pay.SF,
		Amount:         formatAmount(form.Amount),
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
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/slips/"+saved.ID.String(), http.StatusFound)
}

// ── Settings ─────────────────────────────────────────────────────────────────

func (h *handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	renderTemplate(w, h.tmpl.settings, map[string]any{
		"Entrepreneur": e,
		"Saved":        r.URL.Query().Get("saved") == "1",
	})
}

func (h *handler) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	r.ParseForm()
	e.Address = strings.TrimSpace(r.FormValue("address"))
	e.BankAccount = strings.TrimSpace(r.FormValue("bank_account"))
	if err := h.entrepreneurs.Update(r.Context(), e); err != nil {
		http.Error(w, "Грешка при чувању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+idStr+"/settings?saved=1", http.StatusFound)
}

func parseNullDate(s string) sql.NullTime {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func safeIndex(ss []string, i int) string {
	if i < len(ss) {
		return ss[i]
	}
	return ""
}

// handleEntrepreneurUpdate saves editable fields (Title, Name, PIB) for an entrepreneur.
func (h *handler) handleEntrepreneurUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}

	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	pib := strings.TrimSpace(r.FormValue("pib"))
	title := strings.TrimSpace(r.FormValue("title"))

	if name == "" || pib == "" {
		http.Redirect(w, r, "/a/entrepreneurs/"+idStr, http.StatusFound)
		return
	}

	e.Name = name
	e.PIB = pib
	e.Title = title

	if err := h.entrepreneurs.Update(r.Context(), e); err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/entrepreneurs/"+idStr, http.StatusFound)
}

// handlePrivacy renders the privacy policy placeholder.
func (h *handler) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, h.tmpl.placeholder, map[string]any{"Title": "Политика приватности"})
}

// handleTerms renders the terms of use placeholder.
func (h *handler) handleTerms(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, h.tmpl.placeholder, map[string]any{"Title": "Услови коришћења"})
}

// handleProcess receives uploaded PDFs, saves entrepreneurs and slips to DB, renders import summary.
func (h *handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["pdfs"]
	if len(files) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if len(files) > maxUploadFiles {
		files = files[:maxUploadFiles]
	}

	accountantID, _ := h.accountantFromSession(r)

	result := h.importer.ProcessFiles(r.Context(), accountantID, files)
	renderTemplate(w, h.tmpl.results, result)
}

func parseAmount(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return v
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// formatAmount parses a user-entered amount and returns it as "%.2f" string.
// Returns "0.00" for empty or unparseable input.
func formatAmount(s string) string {
	return fmt.Sprintf("%.2f", round2(parseAmount(s)))
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	s = r.Replace(s)
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "slip"
	}
	return s
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

// groupSlipsByYear partitions slips into a latest group and previous groups.
// The latest group is the highest year that has at least one non-advance slip.
// All other years are returned as previous groups, sorted newest-first.
func groupSlipsByYear(slips []sliprecord.SlipRecord) (latest slipYearGroup, previous []slipYearGroup) {
	yearMap := make(map[int]*slipYearGroup)
	for _, s := range slips {
		g, ok := yearMap[s.Year]
		if !ok {
			g = &slipYearGroup{Year: s.Year}
			yearMap[s.Year] = g
		}
		if s.Advance {
			g.Advance = append(g.Advance, s)
		} else {
			g.Regular = append(g.Regular, s)
		}
	}

	// Determine the latest non-advance year; fall back to max year if none.
	latestYear := 0
	for y, g := range yearMap {
		if len(g.Regular) > 0 && y > latestYear {
			latestYear = y
		}
	}
	if latestYear == 0 {
		for y := range yearMap {
			if y > latestYear {
				latestYear = y
			}
		}
	}

	years := make([]int, 0, len(yearMap))
	for y := range yearMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	for _, y := range years {
		if y == latestYear {
			latest = *yearMap[y]
		} else {
			previous = append(previous, *yearMap[y])
		}
	}
	return
}

// ── KPO merge ─────────────────────────────────────────────────────────────────
// The accountant copies entries from the entrepreneur's own KPO book into the
// managed_entrepreneur's KPO book, then marks the entrepreneur's book merged.

func (h *handler) handleKPOMergeView(w http.ResponseWriter, r *http.Request) {
	e, accBook, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	// Load accountant-side entries already in the book.
	accEntries, err := h.kpoBooks.ListEntries(r.Context(), accBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	// Find the paired entrepreneur's book for the same year (if any).
	var eBook kpo.Book
	var eEntries []kpo.Entry
	if e.EntrepreneurUserID != nil {
		eBook, err = h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
		if err == nil {
			eEntries, _ = h.kpoBooks.ListEntries(r.Context(), eBook.ID)
		}
	}

	renderTemplate(w, h.tmpl.kpoMerge, map[string]any{
		"Entrepreneur": e,
		"Year":         year,
		"AccBook":      accBook,
		"AccEntries":   accEntries,
		"EBook":        eBook,
		"EEntries":     eEntries,
		"HasEBook":     e.EntrepreneurUserID != nil && eBook.ID != uuid.Nil,
	})
}

func (h *handler) handleKPOMergeCopyEntry(w http.ResponseWriter, r *http.Request) {
	e, accBook, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if accBook.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}

	eid, err := uuid.Parse(r.PathValue("eid"))
	if err != nil {
		h.renderError(w, http.StatusBadRequest)
		return
	}

	// Verify the source entry belongs to the paired entrepreneur's book.
	if e.EntrepreneurUserID == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eBook, err := h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eEntries, err := h.kpoBooks.ListEntries(r.Context(), eBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању", http.StatusInternalServerError)
		return
	}
	var src *kpo.Entry
	for i := range eEntries {
		if eEntries[i].ID == eid {
			src = &eEntries[i]
			break
		}
	}
	if src == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	if _, err := h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      accBook.ID,
		CollectionDate: src.CollectionDate,
		InvoiceNumber:  src.InvoiceNumber,
		ProductRevenue: src.ProductRevenue,
		ServiceRevenue: src.ServiceRevenue,
	}); err != nil {
		http.Error(w, "Грешка при копирању ставке", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String()+"/kpo/"+strconv.Itoa(year)+"/merge", http.StatusFound)
}

func (h *handler) handleKPOMergeMarkDone(w http.ResponseWriter, r *http.Request) {
	e, _, year, err := h.kpoBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if e.EntrepreneurUserID == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	eBook, err := h.kpoBooks.FindEntrepreneurBook(r.Context(), *e.EntrepreneurUserID, year)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.MarkMerged(r.Context(), eBook.ID); err != nil {
		http.Error(w, "Грешка при обележавању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/a/entrepreneurs/"+e.ID.String()+"/kpo/"+strconv.Itoa(year)+"/merge", http.StatusFound)
}

// ── Entrepreneur registration ─────────────────────────────────────────────────

func (h *handler) handleEntrepreneurRegisterForm(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, h.tmpl.entrepreneurRegister, nil)
}

func (h *handler) handleEntrepreneurRegisterSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	renderErr := func(msg string) {
		renderTemplate(w, h.tmpl.entrepreneurRegister, map[string]any{"Error": msg, "Email": email})
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

// ── Entrepreneur dashboard ────────────────────────────────────────────────────

func (h *handler) handleEntrepreneurDashboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	// Load paired managed entrepreneur if any.
	paired, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	renderTemplate(w, h.tmpl.entrepreneurDashboard, map[string]any{
		"Paired":      paired.ID != uuid.Nil,
		"Managed":     paired,
		"CurrentYear": time.Now().Year(),
	})
}

// ── Invite flow ───────────────────────────────────────────────────────────────

// handleAccountantInviteEntrepreneur creates an invitation for the entrepreneur to accept.
func (h *handler) handleAccountantInviteEntrepreneur(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	if e.IsPaired() {
		h.renderError(w, http.StatusForbidden) // already paired
		return
	}
	accountantID, _ := h.accountantFromSession(r)
	inv, err := h.invitations.Create(r.Context(), invitation.Invitation{
		InviterType:           invitation.InviterTypeAccountant,
		InviterID:             accountantID,
		ManagedEntrepreneurID: &id,
	})
	if err != nil {
		http.Error(w, "Грешка при креирању позивнице", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, h.tmpl.inviteToken, map[string]any{
		"Token":        inv.Token,
		"Entrepreneur": e,
		"InviterType":  "accountant",
	})
}

// handleEntrepreneurInviteForm shows the entrepreneur's invite-accountant form.
func (h *handler) handleEntrepreneurInviteForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	paired, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID)
	if paired.ID != uuid.Nil {
		http.Redirect(w, r, "/e/", http.StatusFound) // already paired
		return
	}
	renderTemplate(w, h.tmpl.entrepreneurInviteAcc, nil)
}

// handleEntrepreneurSendInvite creates an invitation from entrepreneur to accountant.
func (h *handler) handleEntrepreneurSendInvite(w http.ResponseWriter, r *http.Request) {
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
	renderTemplate(w, h.tmpl.inviteToken, map[string]any{
		"Token":       inv.Token,
		"InviterType": "entrepreneur",
	})
}

// handleInviteToken shows details about an invite; the visitor decides whether to accept.
func (h *handler) handleInviteToken(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := inv.Validate(); err != nil {
		renderTemplate(w, h.tmpl.inviteAccept, map[string]any{"Error": "Позивница је истекла или је већ искоришћена."})
		return
	}

	data := map[string]any{
		"Invitation": inv,
		"Token":      token,
	}
	// Load managed entrepreneur if set (accountant-initiated).
	if inv.ManagedEntrepreneurID != nil {
		me, err := h.entrepreneurs.FindByID(r.Context(), *inv.ManagedEntrepreneurID)
		if err == nil {
			data["ManagedEntrepreneur"] = me
		}
	}
	renderTemplate(w, h.tmpl.inviteAccept, data)
}

// handleInviteAccept processes the acceptance of an invite.
// Accountant-initiated: entrepreneur must be logged in → links managed_entrepreneur to user.
// Entrepreneur-initiated: accountant must be logged in → accountant selects which managed entrepreneur.
func (h *handler) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	inv, err := h.invitations.FindByToken(r.Context(), token)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := inv.Validate(); err != nil {
		renderTemplate(w, h.tmpl.inviteAccept, map[string]any{"Error": "Позивница је истекла или је већ искоришћена."})
		return
	}

	sess, ok := h.sessions.Get(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if inv.InviterType == invitation.InviterTypeAccountant {
		// Entrepreneur accepts: must be logged in as entrepreneur.
		if sess.UserType != auth.UserTypeEntrepreneur {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		entrepreneurUserID, _ := uuid.Parse(sess.UserID)
		managedID := *inv.ManagedEntrepreneurID

		// Check not already paired.
		if existing, _ := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), entrepreneurUserID); existing.ID != uuid.Nil {
			renderTemplate(w, h.tmpl.inviteAccept, map[string]any{"Error": "Већ сте повезани са рачуновођом."})
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
		// Create a stub managed entrepreneur for this accountant.
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

// ── Entrepreneur: list pages ──────────────────────────────────────────────────

func (h *handler) handleEClientList(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	clients, err := h.clients.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању клијената", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, h.tmpl.entrepreneurClients, map[string]any{"Clients": clients})
}

func (h *handler) handleEBankAccountList(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	accounts, err := h.bankAccounts.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, h.tmpl.entrepreneurBankAccounts, map[string]any{"Accounts": accounts})
}

func (h *handler) handleEInvoiceList(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	invoices, err := h.invoices.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању фактура", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, h.tmpl.entrepreneurInvoices, map[string]any{"Invoices": invoices})
}

// ── Entrepreneur: client CRUD ─────────────────────────────────────────────────

func (h *handler) handleEClientSearch(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := h.clients.Search(r.Context(), userID, q)
	if err != nil {
		http.Error(w, "Грешка при претрази", http.StatusInternalServerError)
		return
	}
	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]item, len(results))
	for i, c := range results {
		out[i] = item{ID: c.ID.String(), Name: c.Name}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *handler) handleEClientNewForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.entrepreneurUserFromSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	renderTemplate(w, h.tmpl.clientForm, map[string]any{
		"Client":    client.Client{},
		"IsNew":     true,
		"ActionURL": "/e/clients/new",
		"BackURL":   "/e/",
	})
}

func (h *handler) handleEClientNewSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		renderTemplate(w, h.tmpl.clientForm, map[string]any{
			"Client":    eClientFromForm(r, userID),
			"IsNew":     true,
			"Error":     "Назив клијента је обавезан.",
			"ActionURL": "/e/clients/new",
			"BackURL":   "/e/",
		})
		return
	}
	if _, err := h.clients.Create(r.Context(), eClientFromForm(r, userID)); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func (h *handler) handleEClientEditForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	c, err := h.clients.FindByID(r.Context(), cid)
	if err != nil || c.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	renderTemplate(w, h.tmpl.clientForm, map[string]any{
		"Client":    c,
		"IsNew":     false,
		"ActionURL": "/e/clients/" + cid.String(),
		"BackURL":   "/e/",
	})
}

func (h *handler) handleEClientUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		http.Error(w, "Грешка при учитавању клијента", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	r.ParseForm()
	updated := eClientFromForm(r, userID)
	updated.ID = cid
	if updated.Name == "" {
		renderTemplate(w, h.tmpl.clientForm, map[string]any{
			"Client":    updated,
			"IsNew":     false,
			"Error":     "Назив клијента је обавезан.",
			"ActionURL": "/e/clients/" + cid.String(),
			"BackURL":   "/e/",
		})
		return
	}
	if err := h.clients.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func (h *handler) handleEClientDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		http.Error(w, "Грешка при учитавању клијента", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.clients.Delete(r.Context(), cid); err != nil {
		http.Error(w, "Грешка при брисању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func eClientFromForm(r *http.Request, userID uuid.UUID) client.Client {
	return client.Client{
		EntrepreneurUserID: userID,
		Name:               strings.TrimSpace(r.FormValue("name")),
		PIB:                strings.TrimSpace(r.FormValue("pib")),
		RegistrationNumber: strings.TrimSpace(r.FormValue("registration_number")),
		Email:              strings.TrimSpace(r.FormValue("email")),
		Address:            strings.TrimSpace(r.FormValue("address")),
		IsForeign:          r.FormValue("is_foreign") == "on",
	}
}

// ── Entrepreneur: bank account CRUD ──────────────────────────────────────────

func (h *handler) handleEBankAccountNewForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.entrepreneurUserFromSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	renderTemplate(w, h.tmpl.bankAccountForm, map[string]any{
		"IsNew":     true,
		"Account":   bankaccount.BankAccount{},
		"ActionURL": "/e/bank-accounts/new",
		"BackURL":   "/e/",
	})
}

func (h *handler) handleEBankAccountCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	a := eBankAccountFromForm(r, userID)
	if _, err := h.bankAccounts.Create(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func (h *handler) handleEBankAccountEditForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if err != nil && !errors.Is(err, bankaccount.ErrNotFound) {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	if err != nil || a.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cbs, _ := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), aid)
	a.CorrespondentBanks = cbs
	renderTemplate(w, h.tmpl.bankAccountForm, map[string]any{
		"IsNew":                false,
		"Account":              a,
		"ActionURL":            "/e/bank-accounts/" + aid.String(),
		"BackURL":              "/e/",
		"CorrespondentBaseURL": "/e/bank-accounts/" + aid.String() + "/correspondents",
	})
}

func (h *handler) handleEBankAccountUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindByID(r.Context(), aid)
	if err != nil && !errors.Is(err, bankaccount.ErrNotFound) {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	a := eBankAccountFromForm(r, userID)
	a.ID = aid
	if err := h.bankAccounts.Update(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+aid.String()+"/edit", http.StatusFound)
}

func (h *handler) handleEBankAccountDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindByID(r.Context(), aid)
	if err != nil && !errors.Is(err, bankaccount.ErrNotFound) {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.bankAccounts.Delete(r.Context(), aid); err != nil {
		http.Error(w, "Грешка при брисању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/", http.StatusFound)
}

func eBankAccountFromForm(r *http.Request, userID uuid.UUID) bankaccount.BankAccount {
	at := bankaccount.TypeLocal
	if r.FormValue("account_type") == "foreign" {
		at = bankaccount.TypeForeign
	}
	return bankaccount.BankAccount{
		EntrepreneurUserID: userID,
		AccountType:        at,
		BankName:           strings.TrimSpace(r.FormValue("bank_name")),
		AccountNumber:      strings.TrimSpace(r.FormValue("account_number")),
		IBAN:               strings.TrimSpace(r.FormValue("iban")),
		SWIFT:              strings.TrimSpace(r.FormValue("swift")),
	}
}

// ── Entrepreneur: correspondent bank CRUD ────────────────────────────────────

func (h *handler) eOwnedBankAccount(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (bankaccount.BankAccount, bool) {
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return bankaccount.BankAccount{}, false
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if err != nil || a.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return bankaccount.BankAccount{}, false
	}
	return a, true
}

func (h *handler) handleECorrespondentNewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	renderTemplate(w, h.tmpl.correspondentForm, map[string]any{
		"BankAccount":   a,
		"IsNew":         true,
		"Correspondent": bankaccount.CorrespondentBank{},
		"ActionURL":     "/e/bank-accounts/" + a.ID.String() + "/correspondents/new",
		"BackURL":       "/e/bank-accounts/" + a.ID.String() + "/edit",
	})
}

func (h *handler) handleECorrespondentCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	cb := bankaccount.CorrespondentBank{
		BankAccountID: a.ID,
		BankName:      strings.TrimSpace(r.FormValue("bank_name")),
		SWIFT:         strings.TrimSpace(r.FormValue("swift")),
		BankAddress:   strings.TrimSpace(r.FormValue("bank_address")),
	}
	if _, err := h.bankAccounts.CreateCorrespondent(r.Context(), cb); err != nil {
		http.Error(w, "Грешка при чувању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+a.ID.String()+"/edit", http.StatusFound)
}

func (h *handler) handleECorrespondentEditForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if err != nil || cb.BankAccountID != a.ID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	renderTemplate(w, h.tmpl.correspondentForm, map[string]any{
		"BankAccount":   a,
		"IsNew":         false,
		"Correspondent": cb,
		"ActionURL":     "/e/bank-accounts/" + a.ID.String() + "/correspondents/" + cid.String(),
		"BackURL":       "/e/bank-accounts/" + a.ID.String() + "/edit",
	})
}

func (h *handler) handleECorrespondentUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if err != nil || existing.BankAccountID != a.ID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	cb := bankaccount.CorrespondentBank{
		ID:          cid,
		BankName:    strings.TrimSpace(r.FormValue("bank_name")),
		SWIFT:       strings.TrimSpace(r.FormValue("swift")),
		BankAddress: strings.TrimSpace(r.FormValue("bank_address")),
	}
	if err := h.bankAccounts.UpdateCorrespondent(r.Context(), cb); err != nil {
		http.Error(w, "Грешка при чувању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+a.ID.String()+"/edit", http.StatusFound)
}

func (h *handler) handleECorrespondentDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if err != nil || existing.BankAccountID != a.ID {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.bankAccounts.DeleteCorrespondent(r.Context(), cid); err != nil {
		http.Error(w, "Грешка при брисању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+a.ID.String()+"/edit", http.StatusFound)
}

func (h *handler) handleECorrespondentsByAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	cbs, err := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), a.ID)
	if err != nil {
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	type cbItem struct {
		ID          string `json:"id"`
		BankName    string `json:"bank_name"`
		SWIFT       string `json:"swift"`
		BankAddress string `json:"bank_address"`
	}
	out := make([]cbItem, len(cbs))
	for i, cb := range cbs {
		out[i] = cbItem{cb.ID.String(), cb.BankName, cb.SWIFT, cb.BankAddress}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// ── Entrepreneur: invoices ────────────────────────────────────────────────────

func (h *handler) handleEInvoiceNewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	accounts, _ := h.bankAccounts.ListByEntrepreneurUser(r.Context(), userID)
	// Load correspondents into each account
	for i, a := range accounts {
		cbs, _ := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), a.ID)
		accounts[i].CorrespondentBanks = cbs
	}
	renderTemplate(w, h.tmpl.invoiceNew, map[string]any{
		"BackURL":         "/e/",
		"ActionURL":       "/e/invoices",
		"SettingsURL":     "/e/",
		"ClientSearchURL": "/e/clients/search",
		"Today":           time.Now().Format("2006-01-02"),
		"BankAccounts":    accounts,
		"Currencies":      []string{"RSD", "EUR", "USD", "CHF", "GBP"},
		"FXRates":         invoice.FXRates,
		"ProfileComplete": true,
	})
}

func (h *handler) handleEInvoiceCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}

	invType := invoice.TypeStandard
	if r.FormValue("invoice_type") == "advance" {
		invType = invoice.TypeAdvance
	}

	issueDate, err := time.Parse("2006-01-02", r.FormValue("issue_date"))
	if err != nil {
		issueDate = time.Now()
	}

	currency := r.FormValue("currency")
	if _, ok := invoice.FXRates[currency]; !ok {
		http.Error(w, "Непозната валута", http.StatusBadRequest)
		return
	}

	inv := invoice.Invoice{
		EntrepreneurUserID: userID,
		InvoiceType:        invType,
		InvoiceNumber:      strings.TrimSpace(r.FormValue("invoice_number")),
		IssueDate:          issueDate,
		Currency:           currency,
		Notes:              strings.TrimSpace(r.FormValue("notes")),
	}

	if cid, err := uuid.Parse(r.FormValue("client_id")); err == nil {
		inv.ClientID = &cid
	}
	inv.ClientName = strings.TrimSpace(r.FormValue("client_name"))

	if aid, err := uuid.Parse(r.FormValue("bank_account_id")); err == nil {
		inv.BankAccountID = &aid
	}
	if cbid, err := uuid.Parse(r.FormValue("correspondent_bank_id")); err == nil {
		inv.CorrespondentBankID = &cbid
	}

	if dd, err := time.Parse("2006-01-02", r.FormValue("due_date")); err == nil {
		inv.DueDate = sql.NullTime{Time: dd, Valid: true}
	}
	if ps, err := time.Parse("2006-01-02", r.FormValue("period_start")); err == nil {
		if pe, err := time.Parse("2006-01-02", r.FormValue("period_end")); err == nil {
			inv.PeriodStart = sql.NullTime{Time: ps, Valid: true}
			inv.PeriodEnd = sql.NullTime{Time: pe, Valid: true}
		}
	}

	descs := r.Form["item_description[]"]
	qtys := r.Form["item_quantity[]"]
	prices := r.Form["item_unit_price[]"]
	discounts := r.Form["item_discount[]"]
	isProducts := r.Form["item_is_product[]"]
	var items []invoice.Item
	for i, d := range descs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		qty, _ := strconv.ParseFloat(strings.ReplaceAll(safeIndex(qtys, i), ",", "."), 64)
		price, _ := strconv.ParseFloat(strings.ReplaceAll(safeIndex(prices, i), ",", "."), 64)
		disc, _ := strconv.ParseFloat(strings.ReplaceAll(safeIndex(discounts, i), ",", "."), 64)
		isProd := safeIndex(isProducts, i) == "on"
		items = append(items, invoice.Item{
			Description: d,
			Quantity:    qty,
			UnitPrice:   price,
			DiscountPct: disc,
			IsProduct:   isProd,
		})
	}

	var total float64
	for _, it := range items {
		total += it.LineTotal()
	}
	inv.TotalRSD = invoice.ToRSD(total, inv.Currency)

	created, err := h.invoices.Create(r.Context(), inv, items)
	if err != nil {
		http.Error(w, "Грешка при чувању фактуре", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/invoices/"+created.ID.String(), http.StatusFound)
}

func (h *handler) handleEInvoice(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if err != nil || inv.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}

	var grandTotal float64
	for _, it := range items {
		grandTotal += it.LineTotal()
	}

	data := map[string]any{
		"Invoice":    inv,
		"Items":      items,
		"GrandTotal": grandTotal,
		"BackURL":    "/e/",
		"PDFUrl":     "/e/invoices/" + iid.String() + "/pdf",
	}

	if inv.BankAccountID != nil {
		if ba, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID); err == nil {
			data["BankAccount"] = ba
		}
	}
	if inv.CorrespondentBankID != nil {
		if cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID); err == nil {
			data["CorrespondentBank"] = cb
		}
	}

	renderTemplate(w, h.tmpl.invoiceDetail, data)
}

func (h *handler) handleEInvoicePDF(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if err != nil || inv.EntrepreneurUserID != userID {
		h.renderError(w, http.StatusNotFound)
		return
	}

	var grandTotal float64
	for _, it := range items {
		grandTotal += it.LineTotal()
	}

	data := map[string]any{
		"Invoice":    inv,
		"Items":      items,
		"GrandTotal": grandTotal,
		"BackURL":    "/e/invoices/" + iid.String(),
		"PDFUrl":     "#",
	}
	if inv.BankAccountID != nil {
		if ba, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID); err == nil {
			data["BankAccount"] = ba
		}
	}
	if inv.CorrespondentBankID != nil {
		if cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID); err == nil {
			data["CorrespondentBank"] = cb
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="faktura.html"`)
	if err := h.tmpl.invoiceDetail.Execute(w, data); err != nil {
		log.Printf("invoice pdf template error: %v", err)
	}
}

// ── Entrepreneur: KPO ────────────────────────────────────────────────────────

// eKPOBookFromPath resolves the year from the path and returns the book for the logged-in entrepreneur.
func (h *handler) eKPOBookFromPath(r *http.Request) (uuid.UUID, kpo.Book, int, error) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		return uuid.Nil, kpo.Book{}, 0, errors.New("forbidden")
	}
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil || year < 2000 || year > 2100 {
		return uuid.Nil, kpo.Book{}, 0, errors.New("bad year")
	}
	book, err := h.kpoBooks.FindOrCreateForEntrepreneur(r.Context(), userID, year)
	return userID, book, year, err
}

func (h *handler) handleEKPO(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	selectedYear := time.Now().Year()
	if ys := r.URL.Query().Get("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}
	// year path value takes precedence when coming from eMux route
	if ys := r.PathValue("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 2000 && y <= 2100 {
			selectedYear = y
		}
	}

	currentBook, err := h.kpoBooks.FindOrCreateForEntrepreneur(r.Context(), userID, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	entries, err := h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
		return
	}

	books, err := h.kpoBooks.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}

	advanceInvoices, err := h.invoices.ListAdvanceByEntrepreneurUserYear(r.Context(), userID, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању авансних фактура", http.StatusInternalServerError)
		return
	}

	var kpoRows []kpoRow
	for i, e := range entries {
		kpoRows = append(kpoRows, kpoRow{
			IsAdvanceInvoice: false,
			Date:             e.CollectionDate,
			InvoiceNum:       e.InvoiceNumber,
			EntryID:          e.ID,
			OrdinalNumber:    i + 1,
			ProductRevenue:   e.ProductRevenue,
			ServiceRevenue:   e.ServiceRevenue,
			EntryTotal:       e.Total(),
		})
	}
	for _, inv := range advanceInvoices {
		kpoRows = append(kpoRows, kpoRow{
			IsAdvanceInvoice: true,
			Date:             inv.IssueDate,
			InvoiceNum:       inv.InvoiceNumber,
			InvoiceID:        inv.ID,
			ClientName:       inv.ClientName,
			TotalRSD:         inv.TotalRSD,
		})
	}
	sort.Slice(kpoRows, func(i, j int) bool {
		return kpoRows[i].Date.Before(kpoRows[j].Date)
	})

	var totalProduct, totalService float64
	for _, en := range entries {
		totalProduct += en.ProductRevenue
		totalService += en.ServiceRevenue
	}

	renderTemplate(w, h.tmpl.entrepreneurKPO, map[string]any{
		"CurrentBook":  currentBook,
		"KPOBooks":     books,
		"KPOEntries":   entries,
		"KPORows":      kpoRows,
		"SelectedYear": selectedYear,
		"TotalProduct": totalProduct,
		"TotalService": totalService,
		"TotalAll":     totalProduct + totalService,
	})
}

func (h *handler) handleEKPOAddEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	r.ParseForm()
	date, err := parseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум", http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if _, err := h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, "Грешка при уносу", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *handler) handleEKPOUpdateEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	r.ParseForm()
	date, err := parseDayMonth(r.FormValue("collection_date"), year)
	if err != nil {
		http.Error(w, "Неисправан датум", http.StatusBadRequest)
		return
	}
	prodRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("product_revenue"), ",", "."), 64)
	svcRev, _ := strconv.ParseFloat(strings.ReplaceAll(r.FormValue("service_revenue"), ",", "."), 64)
	if err := h.kpoBooks.UpdateEntry(r.Context(), kpo.Entry{
		ID:             entryID,
		KPOBookID:      book.ID,
		CollectionDate: date,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: prodRev,
		ServiceRevenue: svcRev,
	}); err != nil {
		http.Error(w, "Грешка при чувању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *handler) handleEKPOReorderEntries(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	r.ParseForm()
	rawIDs := r.Form["ids[]"]
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, s := range rawIDs {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	if err := h.kpoBooks.ReorderEntries(r.Context(), book.ID, ids); err != nil {
		http.Error(w, "Грешка при промени редоследа", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	_ = year
}

func (h *handler) handleEKPODeleteEntry(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је финализована", http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.DeleteEntry(r.Context(), book.ID, entryID); err != nil {
		http.Error(w, "Грешка при брисању", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *handler) handleEKPOFinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Finalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при финализацији", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}

func (h *handler) handleEKPOUnfinalize(w http.ResponseWriter, r *http.Request) {
	_, book, year, err := h.eKPOBookFromPath(r)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.kpoBooks.Unfinalize(r.Context(), book.ID); err != nil {
		http.Error(w, "Грешка при поништавању финализације", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/kpo/"+strconv.Itoa(year)+"#kpo", http.StatusFound)
}
