package web

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/entrepreneur"
	"buh/internal/importer"
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
	login           *template.Template
	index           *template.Template
	entrepreneur    *template.Template
	entrepreneurNew *template.Template
	results         *template.Template
	slip            *template.Template
	slipNew         *template.Template
	placeholder     *template.Template
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
	return templates{
		login:           template.Must(template.New("login.html").ParseFS(templateFS, "templates/login.html")),
		index:           mustPageTmpl("index.html", nil),
		entrepreneur:    mustPageTmpl("entrepreneur.html", nil),
		entrepreneurNew: mustPageTmpl("entrepreneur_new.html", nil),
		results:         mustPageTmpl("results.html", nil),
		slip:            mustPageTmpl("slip.html", nil, slipExtra...),
		slipNew:         mustPageTmpl("slip_new.html", nil, slipExtra...),
		placeholder:     mustPageTmpl("placeholder.html", nil),
	}
}

type handler struct {
	accountants   *accountant.Repo
	sessions      *auth.SessionManager
	entrepreneurs *entrepreneur.Repo
	slips         *sliprecord.Repo
	kpoBooks      *kpo.Repo
	importer      *importer.Importer
	tmpl          templates
}

// NewHandler returns an HTTP handler for the web UI.
func NewHandler(accountants *accountant.Repo, sessions *auth.SessionManager, db *sql.DB) http.Handler {
	entrepreneurs := entrepreneur.NewRepo(db)
	slips := sliprecord.NewRepo(db)
	kpoBooks := kpo.NewRepo(db)
	h := &handler{
		accountants:   accountants,
		sessions:      sessions,
		entrepreneurs: entrepreneurs,
		slips:         slips,
		kpoBooks:      kpoBooks,
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

// handleIndex lists all entrepreneurs for the logged-in accountant.
func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
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
		http.NotFound(w, r)
		return
	}

	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
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
	if errors.Is(err, kpo.ErrNotFound) {
		currentBook = kpo.Book{Year: selectedYear, EntrepreneurID: id}
	}

	var entries []kpo.Entry
	if currentBook.ID != (uuid.UUID{}) {
		entries, err = h.kpoBooks.ListEntries(r.Context(), currentBook.ID)
		if err != nil {
			http.Error(w, "Грешка при учитавању КПО ставки", http.StatusInternalServerError)
			return
		}
	}

	books, err := h.kpoBooks.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању КПО књига", http.StatusInternalServerError)
		return
	}

	slips, err := h.slips.ListByEntrepreneur(r.Context(), id)
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатница", http.StatusInternalServerError)
		return
	}

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
		"SelectedYear":      selectedYear,
		"TotalProduct":      totalProduct,
		"TotalService":      totalService,
		"TotalAll":          totalProduct + totalService,
		"Slips":             slips,
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
	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if err != nil {
		return entrepreneur.Entrepreneur{}, kpo.Book{}, 0, err
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
		http.NotFound(w, r)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је укњижен", http.StatusForbidden)
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
		http.NotFound(w, r)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је укњижен", http.StatusForbidden)
		return
	}

	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		http.NotFound(w, r)
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
		http.NotFound(w, r)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је укњижен", http.StatusForbidden)
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
		http.NotFound(w, r)
		return
	}
	if book.IsFinalized() {
		http.Error(w, "КПО је укњижен", http.StatusForbidden)
		return
	}
	entryID, err := uuid.Parse(r.PathValue("entryID"))
	if err != nil {
		http.NotFound(w, r)
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
		http.NotFound(w, r)
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
		http.NotFound(w, r)
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
		http.NotFound(w, r)
		return
	}

	s, err := h.slips.FindByID(context.Background(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
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
	return existing
}

// handleSlipSave saves editable field values to DB and redirects to slip detail with ?saved=1.
func (h *handler) handleSlipSave(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	existing, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
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
		http.NotFound(w, r)
		return
	}

	existing, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
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
		http.NotFound(w, r)
		return
	}

	s, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
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
		http.NotFound(w, r)
		return
	}

	s, err := h.slips.FindByID(r.Context(), id)
	if errors.Is(err, sliprecord.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању уплатнице", http.StatusInternalServerError)
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
}

// handleSlipNewForm renders the manual slip creation form.
func (h *handler) handleSlipNewForm(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	e, err := h.entrepreneurs.FindByID(context.Background(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, h.tmpl.slipNew, map[string]any{
		"Entrepreneur": e,
		"Form":         slipNewForm{SF: "253", Currency: "RSD", P: e.Name},
	})
}

// handleSlipNewSubmit builds an IPS payment from form values, generates a PDF,
// saves a SlipRecord, and redirects to the slip detail page.
func (h *handler) handleSlipNewSubmit(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	e, err := h.entrepreneurs.FindByID(context.Background(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
		return
	}

	r.ParseForm()
	form := slipNewForm{
		S:        strings.TrimSpace(r.FormValue("S")),
		N:        strings.TrimSpace(r.FormValue("N")),
		R:        strings.TrimSpace(r.FormValue("R")),
		RO:       strings.TrimSpace(r.FormValue("RO")),
		SF:       strings.TrimSpace(r.FormValue("SF")),
		Amount:   strings.TrimSpace(r.FormValue("amount")),
		Currency: r.FormValue("currency"),
		P:        strings.TrimSpace(r.FormValue("P")),
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
	}
	saved, err := h.slips.Save(context.Background(), rec)
	if err != nil {
		http.Error(w, "Грешка при чувању уплатнице", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/slips/"+saved.ID.String(), http.StatusFound)
}

// handleEntrepreneurUpdate saves editable fields (Title, Name, PIB) for an entrepreneur.
func (h *handler) handleEntrepreneurUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	e, err := h.entrepreneurs.FindByID(r.Context(), id)
	if errors.Is(err, entrepreneur.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Грешка при учитавању предузетника", http.StatusInternalServerError)
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
