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
	"buh/internal/importer"
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
		login:           template.Must(template.New("login.html").ParseFS(templateFS, "templates/login.html")),
		index:           mustPageTmpl("index.html", nil),
		entrepreneur:    mustPageTmpl("entrepreneur.html", entrepreneurFuncs, "templates/slip_table.html"),
		entrepreneurNew: mustPageTmpl("entrepreneur_new.html", nil),
		results:         mustPageTmpl("results.html", nil),
		slip:            mustPageTmpl("slip.html", nil, slipExtra...),
		slipNew:         mustPageTmpl("slip_new.html", nil, slipExtra...),
		placeholder:     mustPageTmpl("placeholder.html", nil),
		errPage:         mustPageTmpl("error.html", nil),
		settings:          mustPageTmpl("settings.html", nil),
		clientForm:        mustPageTmpl("client_form.html", nil),
		bankAccountForm:   mustPageTmpl("bank_account_form.html", nil),
		correspondentForm: mustPageTmpl("correspondent_form.html", nil),
		invoiceNew:        mustPageTmpl("invoice_new.html", nil),
		invoiceDetail:     mustPageTmpl("invoice.html", nil),
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
	accountants   *accountant.Repo
	sessions      *auth.SessionManager
	entrepreneurs *entrepreneur.Repo
	slips         *sliprecord.Repo
	kpoBooks      *kpo.Repo
	clients       *client.Repo
	invoices      *invoice.Repo
	bankAccounts  *bankaccount.Repo
	importer      *importer.Importer
	tmpl          templates
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
func NewHandler(accountants *accountant.Repo, sessions *auth.SessionManager, db *sql.DB) http.Handler {
	entrepreneurs := entrepreneur.NewRepo(db)
	slips := sliprecord.NewRepo(db)
	kpoBooks := kpo.NewRepo(db)
	clients := client.NewRepo(db)
	invoices := invoice.NewRepo(db)
	bankAccounts := bankaccount.NewRepo(db)
	h := &handler{
		accountants:   accountants,
		sessions:      sessions,
		entrepreneurs: entrepreneurs,
		slips:         slips,
		kpoBooks:      kpoBooks,
		clients:       clients,
		invoices:      invoices,
		bankAccounts:  bankAccounts,
		importer:      importer.New(entrepreneurs, slips),
		tmpl:          parseTemplates(),
	}
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/logout", h.handleLogout)
	mux.HandleFunc("/privacy", h.handlePrivacy)
	mux.HandleFunc("/terms", h.handleTerms)

	protected := http.NewServeMux()
	protected.HandleFunc("POST /process", h.handleProcess)
	protected.HandleFunc("GET /entrepreneurs/new", h.handleEntrepreneurNewForm)
	protected.HandleFunc("POST /entrepreneurs/new", h.handleEntrepreneurNewSubmit)
	protected.HandleFunc("GET /entrepreneurs/{id}", h.handleEntrepreneur)
	protected.HandleFunc("POST /entrepreneurs/{id}", h.handleEntrepreneurUpdate)
	protected.HandleFunc("GET /entrepreneurs/{id}/settings", h.handleSettings)
	protected.HandleFunc("POST /entrepreneurs/{id}/settings", h.handleSettingsUpdate)
	protected.HandleFunc("GET /entrepreneurs/{id}/clients/search", h.handleClientSearch)
	protected.HandleFunc("GET /entrepreneurs/{id}/clients/new", h.handleClientNewForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/clients/new", h.handleClientNewSubmit)
	protected.HandleFunc("GET /entrepreneurs/{id}/clients/{cid}/edit", h.handleClientEditForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/clients/{cid}", h.handleClientUpdate)
	protected.HandleFunc("POST /entrepreneurs/{id}/clients/{cid}/delete", h.handleClientDelete)
	protected.HandleFunc("GET /entrepreneurs/{id}/bank-accounts/new", h.handleBankAccountNewForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/new", h.handleBankAccountCreate)
	protected.HandleFunc("GET /entrepreneurs/{id}/bank-accounts/{aid}/edit", h.handleBankAccountEditForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/{aid}", h.handleBankAccountUpdate)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/{aid}/delete", h.handleBankAccountDelete)
	protected.HandleFunc("GET /entrepreneurs/{id}/bank-accounts/{aid}/correspondents/new", h.handleCorrespondentNewForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/{aid}/correspondents/new", h.handleCorrespondentCreate)
	protected.HandleFunc("GET /entrepreneurs/{id}/bank-accounts/{aid}/correspondents/{cid}/edit", h.handleCorrespondentEditForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/{aid}/correspondents/{cid}", h.handleCorrespondentUpdate)
	protected.HandleFunc("POST /entrepreneurs/{id}/bank-accounts/{aid}/correspondents/{cid}/delete", h.handleCorrespondentDelete)
	protected.HandleFunc("GET /entrepreneurs/{id}/bank-accounts/{aid}/correspondents", h.handleCorrespondentsByAccount)
	protected.HandleFunc("GET /entrepreneurs/{id}/invoices/new", h.handleInvoiceNewForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/invoices", h.handleInvoiceCreate)
	protected.HandleFunc("GET /entrepreneurs/{id}/invoices/{iid}", h.handleInvoice)
	protected.HandleFunc("GET /entrepreneurs/{id}/invoices/{iid}/pdf", h.handleInvoicePDF)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries", h.handleKPOAddEntry)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/reorder", h.handleKPOReorderEntries)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/update", h.handleKPOUpdateEntry)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/delete", h.handleKPODeleteEntry)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/finalize", h.handleKPOFinalize)
	protected.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/unfinalize", h.handleKPOUnfinalize)
	protected.HandleFunc("GET /entrepreneurs/{id}/slips/new", h.handleSlipNewForm)
	protected.HandleFunc("POST /entrepreneurs/{id}/slips/new", h.handleSlipNewSubmit)
	protected.HandleFunc("GET /slips/{id}", h.handleSlip)
	protected.HandleFunc("POST /slips/{id}/save", h.handleSlipSave)
	protected.HandleFunc("POST /slips/{id}/download", h.handleSlipDownload)
	protected.HandleFunc("POST /slips/{id}/delete", h.handleSlipDelete)
	protected.HandleFunc("GET /slips/{id}/pdf", h.handleSlipPDF)
	protected.HandleFunc("/", h.handleIndex)

	mux.Handle("/", middleware.RequireAuth(sessions, protected))
	return mux
}

// handleLogin GET → login form, POST → check email+password.
func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		a, err := h.accountants.FindByEmail(context.Background(), email)
		if err == nil {
			err = accountant.CheckPassword(a, password)
		}
		if errors.Is(err, accountant.ErrNotFound) || errors.Is(err, accountant.ErrInvalidCredentials) {
			renderTemplate(w, h.tmpl.login, map[string]any{"Error": "Погрешна е-пошта или лозинка."})
			return
		}
		if err != nil {
			http.Error(w, "Грешка при пријави", http.StatusInternalServerError)
			return
		}

		if err := h.sessions.Set(w, a.ID); err != nil {
			http.Error(w, "Грешка при постављању сесије", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderTemplate(w, h.tmpl.login, nil)
}

// handleLogout clears the session and redirects to /login.
func (h *handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	h.sessions.Clear(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// accountantFromSession returns the accountant UUID stored in the current session.
func (h *handler) accountantFromSession(r *http.Request) (uuid.UUID, bool) {
	s, _ := h.sessions.Get(r)
	id, err := uuid.Parse(s)
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
	if err != nil || e.AccountantID != accountantID {
		h.renderError(w, http.StatusForbidden)
		return sliprecord.SlipRecord{}, false
	}
	return s, true
}

// handleIndex lists all entrepreneurs for the logged-in accountant.
func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.renderError(w, http.StatusNotFound)
		return
	}
	accountantIDStr, _ := h.sessions.Get(r)
	accountantID, err := uuid.Parse(accountantIDStr)
	if err != nil {
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

	currentBook, err := h.kpoBooks.FindOrCreate(r.Context(), id, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО", http.StatusInternalServerError)
		return
	}

	entries, err := h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
		return
	}

	books, err := h.kpoBooks.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}

	advanceInvoices, err := h.invoices.ListAdvanceByEntrepreneurYear(r.Context(), id, selectedYear)
	if err != nil {
		http.Error(w, "Грешка при учитавању авансних фактура", http.StatusInternalServerError)
		return
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
		"Entrepreneur":      e,
		"KPOBooks":          books,
		"CurrentBook":       currentBook,
		"KPOEntries":        entries,
		"KPORows":           kpoRows,
		"SelectedYear":      selectedYear,
		"TotalProduct":      totalProduct,
		"TotalService":      totalService,
		"TotalAll":          totalProduct + totalService,
		"LatestSlipYear":    latestSlipYear,
		"PrevSlipYears":     prevSlipYears,
		"PausalalYear":      currentYear,
		"PausalalHasData":   pausalalHasData,
		"PausalalTotalFmt":  formatIntWithSpaces(int64(math.Round(pausalalTotal))),
		"PausalalLimitFmt":  formatIntWithSpaces(pausalalLimit),
		"PausalalPercent":   pausalalPercent,
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
	book, err := h.kpoBooks.FindOrCreate(r.Context(), id, year)
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

	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s?year=%d#kpo-new", r.PathValue("id"), year), http.StatusFound)
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
		CollectionDate: collectionDate,
		InvoiceNumber:  strings.TrimSpace(r.FormValue("invoice_number")),
		ProductRevenue: productRev,
		ServiceRevenue: serviceRev,
	}); err != nil {
		http.Error(w, "Грешка при измени ставке", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s?year=%d#kpo", r.PathValue("id"), year), http.StatusFound)
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
	if err := h.kpoBooks.DeleteEntry(r.Context(), entryID); err != nil {
		http.Error(w, "Грешка при брисању ставке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
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
	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
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
	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s?year=%d", r.PathValue("id"), year), http.StatusFound)
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
			"PIB":  pib,
		})
		return
	}

	accountantIDStr, _ := h.sessions.Get(r)
	accountantID, err := uuid.Parse(accountantIDStr)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	e, _, err := h.entrepreneurs.FindOrCreate(context.Background(), accountantID, pib, name)
	if err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/entrepreneurs/"+e.ID.String(), http.StatusFound)
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

	http.Redirect(w, r, "/slips/"+idStr+"?saved=1", http.StatusFound)
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

	http.Redirect(w, r, "/slips/"+idStr+"?saved=1&download=1", http.StatusFound)
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
	http.Redirect(w, r, "/entrepreneurs/"+entrepreneurID.String(), http.StatusFound)
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

	http.Redirect(w, r, "/slips/"+saved.ID.String(), http.StatusFound)
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
	clients, err := h.clients.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању клијената", http.StatusInternalServerError)
		return
	}
	accounts, err := h.bankAccounts.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	// Load correspondent banks for each account.
	for i, a := range accounts {
		cbs, err := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), a.ID)
		if err != nil {
			http.Error(w, "Грешка при учитавању кор. банака", http.StatusInternalServerError)
			return
		}
		accounts[i].CorrespondentBanks = cbs
	}
	renderTemplate(w, h.tmpl.settings, map[string]any{
		"Entrepreneur": e,
		"Clients":      clients,
		"BankAccounts": accounts,
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
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings?saved=1", http.StatusFound)
}

// ── Client CRUD ───────────────────────────────────────────────────────────────

func (h *handler) handleClientSearch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	results, err := h.clients.Search(r.Context(), id, q)
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

func (h *handler) handleClientNewForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	renderTemplate(w, h.tmpl.clientForm, map[string]any{
		"Entrepreneur": e,
		"Client":       client.Client{},
		"IsNew":        true,
	})
}

func (h *handler) handleClientNewSubmit(w http.ResponseWriter, r *http.Request) {
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
	if name == "" {
		renderTemplate(w, h.tmpl.clientForm, map[string]any{
			"Entrepreneur": e,
			"Client":       buildClientFromForm(r, id),
			"IsNew":        true,
			"Error":        "Назив клијента је обавезан.",
		})
		return
	}
	c := buildClientFromForm(r, id)
	if _, err := h.clients.Create(r.Context(), c); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func (h *handler) handleClientEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	c, err := h.clients.FindByID(r.Context(), cid)
	if errors.Is(err, client.ErrNotFound) || c.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању клијента", http.StatusInternalServerError)
		return
	}
	renderTemplate(w, h.tmpl.clientForm, map[string]any{
		"Entrepreneur": e,
		"Client":       c,
		"IsNew":        false,
	})
}

func (h *handler) handleClientUpdate(w http.ResponseWriter, r *http.Request) {
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
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if errors.Is(err, client.ErrNotFound) || existing.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању", http.StatusInternalServerError)
		return
	}
	r.ParseForm()
	updated := buildClientFromForm(r, id)
	updated.ID = cid
	if updated.Name == "" {
		renderTemplate(w, h.tmpl.clientForm, map[string]any{
			"Entrepreneur": e,
			"Client":       updated,
			"IsNew":        false,
			"Error":        "Назив клијента је обавезан.",
		})
		return
	}
	if err := h.clients.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func (h *handler) handleClientDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if errors.Is(err, client.ErrNotFound) || existing.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.clients.Delete(r.Context(), cid); err != nil {
		http.Error(w, "Грешка при брисању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func buildClientFromForm(r *http.Request, entrepreneurID uuid.UUID) client.Client {
	return client.Client{
		EntrepreneurID:     entrepreneurID,
		Name:               strings.TrimSpace(r.FormValue("name")),
		PIB:                strings.TrimSpace(r.FormValue("pib")),
		RegistrationNumber: strings.TrimSpace(r.FormValue("registration_number")),
		Email:              strings.TrimSpace(r.FormValue("email")),
		Address:            strings.TrimSpace(r.FormValue("address")),
		IsForeign:          r.FormValue("is_foreign") == "on",
	}
}

// ── Bank account handlers ─────────────────────────────────────────────────────

func (h *handler) handleBankAccountNewForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	renderTemplate(w, h.tmpl.bankAccountForm, map[string]any{
		"Entrepreneur": e,
		"IsNew":        true,
		"Account":      bankaccount.BankAccount{},
	})
}

func (h *handler) handleBankAccountCreate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	a := bankAccountFromForm(r, id)
	if _, err := h.bankAccounts.Create(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func (h *handler) handleBankAccountEditForm(w http.ResponseWriter, r *http.Request) {
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
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	cbs, _ := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), aid)
	a.CorrespondentBanks = cbs
	renderTemplate(w, h.tmpl.bankAccountForm, map[string]any{
		"Entrepreneur": e,
		"IsNew":        false,
		"Account":      a,
	})
}

func (h *handler) handleBankAccountUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || existing.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	a := bankAccountFromForm(r, id)
	a.ID = aid
	if err := h.bankAccounts.Update(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func (h *handler) handleBankAccountDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || existing.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.bankAccounts.Delete(r.Context(), aid); err != nil {
		http.Error(w, "Грешка при брисању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/settings", http.StatusFound)
}

func (h *handler) handleCorrespondentNewForm(w http.ResponseWriter, r *http.Request) {
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
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	renderTemplate(w, h.tmpl.correspondentForm, map[string]any{
		"Entrepreneur": e,
		"BankAccount":  a,
		"IsNew":        true,
		"Correspondent": bankaccount.CorrespondentBank{},
	})
}

func (h *handler) handleCorrespondentCreate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	cb := bankaccount.CorrespondentBank{
		BankAccountID: aid,
		BankName:      strings.TrimSpace(r.FormValue("bank_name")),
		SWIFT:         strings.TrimSpace(r.FormValue("swift")),
		BankAddress:   strings.TrimSpace(r.FormValue("bank_address")),
	}
	if _, err := h.bankAccounts.CreateCorrespondent(r.Context(), cb); err != nil {
		http.Error(w, "Грешка при чувању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/bank-accounts/"+aid.String()+"/edit", http.StatusFound)
}

func (h *handler) handleCorrespondentEditForm(w http.ResponseWriter, r *http.Request) {
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
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if errors.Is(err, bankaccount.ErrNotFound) || cb.BankAccountID != aid {
		h.renderError(w, http.StatusNotFound)
		return
	}
	renderTemplate(w, h.tmpl.correspondentForm, map[string]any{
		"Entrepreneur":  e,
		"BankAccount":   a,
		"IsNew":         false,
		"Correspondent": cb,
	})
}

func (h *handler) handleCorrespondentUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if errors.Is(err, bankaccount.ErrNotFound) || existing.BankAccountID != aid {
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
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/bank-accounts/"+aid.String()+"/edit", http.StatusFound)
}

func (h *handler) handleCorrespondentDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	existing, err := h.bankAccounts.FindCorrespondentByID(r.Context(), cid)
	if errors.Is(err, bankaccount.ErrNotFound) || existing.BankAccountID != aid {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err := h.bankAccounts.DeleteCorrespondent(r.Context(), cid); err != nil {
		http.Error(w, "Грешка при брисању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/entrepreneurs/"+idStr+"/bank-accounts/"+aid.String()+"/edit", http.StatusFound)
}

// handleCorrespondentsByAccount returns JSON list of correspondents for a bank account (for invoice form AJAX).
func (h *handler) handleCorrespondentsByAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a, err := h.bankAccounts.FindByID(r.Context(), aid)
	if errors.Is(err, bankaccount.ErrNotFound) || a.EntrepreneurID != id {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	cbs, err := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), aid)
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

func bankAccountFromForm(r *http.Request, entrepreneurID uuid.UUID) bankaccount.BankAccount {
	at := bankaccount.TypeLocal
	if r.FormValue("account_type") == "foreign" {
		at = bankaccount.TypeForeign
	}
	return bankaccount.BankAccount{
		EntrepreneurID: entrepreneurID,
		AccountType:    at,
		BankName:       strings.TrimSpace(r.FormValue("bank_name")),
		AccountNumber:  strings.TrimSpace(r.FormValue("account_number")),
		IBAN:           strings.TrimSpace(r.FormValue("iban")),
		SWIFT:          strings.TrimSpace(r.FormValue("swift")),
	}
}

// ── Invoice handlers ──────────────────────────────────────────────────────────

func (h *handler) handleInvoiceNewForm(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	accounts, err := h.bankAccounts.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању рачуна", http.StatusInternalServerError)
		return
	}
	for i, a := range accounts {
		cbs, err := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), a.ID)
		if err != nil {
			http.Error(w, "Грешка при учитавању кор. банака", http.StatusInternalServerError)
			return
		}
		accounts[i].CorrespondentBanks = cbs
	}
	renderTemplate(w, h.tmpl.invoiceNew, map[string]any{
		"Entrepreneur":    e,
		"ProfileComplete": e.ProfileComplete(),
		"Today":           time.Now().Format("2006-01-02"),
		"Currencies":      []string{"RSD", "EUR", "USD", "CHF", "GBP"},
		"FXRates":         invoice.FXRates,
		"BankAccounts":    accounts,
	})
}

func (h *handler) handleInvoiceCreate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if _, ok := h.findOwnedEntrepreneur(w, r, id); !ok {
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
		http.Error(w, "Неисправан датум издавања", http.StatusBadRequest)
		return
	}

	var clientID *uuid.UUID
	if cidStr := strings.TrimSpace(r.FormValue("client_id")); cidStr != "" {
		if cid, err := uuid.Parse(cidStr); err == nil {
			clientID = &cid
		}
	}
	clientName := strings.TrimSpace(r.FormValue("client_name"))

	currency := r.FormValue("currency")
	if _, ok := invoice.FXRates[currency]; !ok {
		currency = "RSD"
	}

	var bankAccountID *uuid.UUID
	if baidStr := strings.TrimSpace(r.FormValue("bank_account_id")); baidStr != "" {
		if baid, err := uuid.Parse(baidStr); err == nil {
			bankAccountID = &baid
		}
	}
	var correspondentBankID *uuid.UUID
	if cbidStr := strings.TrimSpace(r.FormValue("correspondent_bank_id")); cbidStr != "" {
		if cbid, err := uuid.Parse(cbidStr); err == nil {
			correspondentBankID = &cbid
		}
	}

	inv := invoice.Invoice{
		EntrepreneurID:      id,
		ClientID:            clientID,
		ClientName:          clientName,
		InvoiceType:         invType,
		InvoiceNumber:       strings.TrimSpace(r.FormValue("invoice_number")),
		IssueDate:           issueDate,
		PeriodStart:         parseNullDate(r.FormValue("period_start")),
		PeriodEnd:           parseNullDate(r.FormValue("period_end")),
		DueDate:             parseNullDate(r.FormValue("due_date")),
		Currency:            currency,
		Notes:               strings.TrimSpace(r.FormValue("notes")),
		BankAccountID:       bankAccountID,
		CorrespondentBankID: correspondentBankID,
	}

	// Parse line items.
	descriptions := r.Form["item_description[]"]
	quantities := r.Form["item_quantity[]"]
	unitPrices := r.Form["item_unit_price[]"]
	discountPcts := r.Form["item_discount[]"]
	isProducts := r.Form["item_is_product[]"]

	var items []invoice.Item
	var totalProduct, totalService float64
	for i := range descriptions {
		desc := strings.TrimSpace(descriptions[i])
		if desc == "" {
			continue
		}
		qty := parseAmount(safeIndex(quantities, i))
		if qty <= 0 {
			qty = 1
		}
		price := parseAmount(safeIndex(unitPrices, i))
		disc := parseAmount(safeIndex(discountPcts, i))
		isProd := safeIndex(isProducts, i) == "on"
		it := invoice.Item{
			Description: desc,
			Quantity:    qty,
			UnitPrice:   price,
			DiscountPct: disc,
			IsProduct:   isProd,
			Position:    i + 1,
		}
		lineTotal := it.LineTotal()
		if isProd {
			totalProduct += lineTotal
		} else {
			totalService += lineTotal
		}
		items = append(items, it)
	}

	grandTotal := totalProduct + totalService
	inv.TotalRSD = round2(invoice.ToRSD(grandTotal, currency))

	saved, err := h.invoices.Create(r.Context(), inv, items)
	if err != nil {
		http.Error(w, "Грешка при чувању фактуре", http.StatusInternalServerError)
		return
	}

	// Auto-create KPO entry for standard invoices.
	if invType == invoice.TypeStandard {
		book, err := h.kpoBooks.FindOrCreate(r.Context(), id, issueDate.Year())
		if err != nil {
			log.Printf("kpo FindOrCreate: %v", err)
		} else if !book.IsFinalized() {
			prodRSD := round2(invoice.ToRSD(totalProduct, currency))
			svcRSD := round2(invoice.ToRSD(totalService, currency))
			_, err = h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
				KPOBookID:      book.ID,
				CollectionDate: issueDate,
				InvoiceNumber:  saved.InvoiceNumber,
				ProductRevenue: prodRSD,
				ServiceRevenue: svcRSD,
			})
			if err != nil {
				log.Printf("kpo AddEntry: %v", err)
			}
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/entrepreneurs/%s/invoices/%s", idStr, saved.ID), http.StatusFound)
}

func (h *handler) handleInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if errors.Is(err, invoice.ErrNotFound) || inv.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању фактуре", http.StatusInternalServerError)
		return
	}

	var grandTotal float64
	for _, it := range items {
		grandTotal += it.LineTotal()
	}

	var bankAcc *bankaccount.BankAccount
	var corrBank *bankaccount.CorrespondentBank
	if inv.BankAccountID != nil {
		a, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID)
		if err == nil {
			bankAcc = &a
		}
	}
	if inv.CorrespondentBankID != nil {
		cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID)
		if err == nil {
			corrBank = &cb
		}
	}

	renderTemplate(w, h.tmpl.invoiceDetail, map[string]any{
		"Entrepreneur":    e,
		"Invoice":         inv,
		"Items":           items,
		"GrandTotal":      grandTotal,
		"BankAccount":     bankAcc,
		"CorrespondentBank": corrBank,
	})
}

func (h *handler) handleInvoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	e, ok := h.findOwnedEntrepreneur(w, r, id)
	if !ok {
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if errors.Is(err, invoice.ErrNotFound) || inv.EntrepreneurID != id {
		h.renderError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању фактуре", http.StatusInternalServerError)
		return
	}

	issuer := invoice.IssuerInfo{
		Name:        e.Name,
		PIB:         e.PIB,
		Address:     e.Address,
		BankAccount: e.BankAccount,
	}
	var bankAccInfo *invoice.BankAccountInfo
	if inv.BankAccountID != nil {
		a, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID)
		if err == nil {
			info := invoice.BankAccountInfo{
				AccountType:   string(a.AccountType),
				BankName:      a.BankName,
				AccountNumber: a.AccountNumber,
				IBAN:          a.IBAN,
				SWIFT:         a.SWIFT,
			}
			if inv.CorrespondentBankID != nil {
				cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID)
				if err == nil {
					info.CorrespondentBankName = cb.BankName
					info.CorrespondentBankSWIFT = cb.SWIFT
					info.CorrespondentBankAddress = cb.BankAddress
				}
			}
			bankAccInfo = &info
		}
	}
	pdfBytes, err := invoice.GeneratePDF(inv, items, issuer, bankAccInfo)
	if err != nil {
		http.Error(w, fmt.Sprintf("Грешка при генерисању PDF: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("faktura-%s.pdf", sanitizeFilename(inv.InvoiceNumber))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(pdfBytes)
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
		http.Redirect(w, r, "/entrepreneurs/"+idStr, http.StatusFound)
		return
	}

	e.Name = name
	e.PIB = pib
	e.Title = title

	if err := h.entrepreneurs.Update(r.Context(), e); err != nil {
		http.Error(w, "Грешка при чувању предузетника", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/entrepreneurs/"+idStr, http.StatusFound)
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

	accountantIDStr, _ := h.sessions.Get(r)
	accountantID, _ := uuid.Parse(accountantIDStr)

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
