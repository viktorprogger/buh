package web

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"buh/internal/extractor"
	"buh/internal/ips"
	"buh/internal/slip"
)

const sessionCookie = "buh_session"
const maxUploadFiles = 4

type handler struct {
	password string
	mux      *http.ServeMux
}

// NewHandler returns an HTTP handler for the web UI.
func NewHandler(password string) http.Handler {
	h := &handler{password: password}
	mux := http.NewServeMux()
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/process", h.auth(h.handleProcess))
	mux.HandleFunc("/download", h.auth(h.handleDownload))
	mux.HandleFunc("/", h.auth(h.handleIndex))
	return mux
}

func (h *handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value != h.password {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// handleLogin GET → login form, POST → check password.
func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		r.ParseForm()
		if r.FormValue("password") == h.password {
			http.SetCookie(w, &http.Cookie{
				Name:  sessionCookie,
				Value: h.password,
				Path:  "/",
			})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		renderTemplate(w, loginTmpl, map[string]any{"Error": "Погрешна лозинка."})
		return
	}
	renderTemplate(w, loginTmpl, nil)
}

// handleIndex shows the PDF upload form.
func (h *handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, indexTmpl, nil)
}

// slipView is the data passed to the results template for one uplatnica.
type slipView struct {
	// Display fields
	Payer        string
	Purpose      string
	Payee        string
	PaymentCode  string
	Currency     string
	Amount       string // editable, just the number (e.g. "1000,00")
	PayeeAccount string
	Reference    string
	// Hidden IPS fields for reconstruction
	K  string
	V  string
	C  string
	R  string
	N  string
	SF string
	S  string
	RO string
	O  string
	P  string
	M  string
	JS string
	RL string
	RP string
}

// handleProcess receives uploaded PDFs, extracts QR codes, renders results.
func (h *handler) handleProcess(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	r.ParseMultipartForm(32 << 20)
	files := r.MultipartForm.File["pdfs"]
	if len(files) == 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if len(files) > maxUploadFiles {
		files = files[:maxUploadFiles]
	}

	var slips []slipView
	var warnings []string

	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: грешка при отварању", fh.Filename))
			continue
		}

		tmp, err := os.CreateTemp("", "buh-upload-*.pdf")
		if err != nil {
			f.Close()
			warnings = append(warnings, fmt.Sprintf("%s: грешка при чувању", fh.Filename))
			continue
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(f)
		f.Close()
		tmp.Write(buf.Bytes())
		tmp.Close()

		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)

		_, codes, err := extractor.ExtractQRCodes(tmpPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", fh.Filename, err))
			continue
		}
		if len(codes) == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: нису пронађени QR кодови", fh.Filename))
			continue
		}

		for _, code := range codes {
			if !ips.IsIPS(code) {
				continue
			}
			pay, err := ips.Parse(code)
			if err != nil {
				continue
			}
			slips = append(slips, paymentToView(pay))
		}
	}

	renderTemplate(w, resultsTmpl, map[string]any{
		"Slips":    slips,
		"Warnings": warnings,
	})
}

// handleDownload generates and serves a single payment slip PDF.
func (h *handler) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	r.ParseForm()

	pay := &ips.Payment{
		K:  ips.PaymentCode(r.FormValue("K")),
		V:  r.FormValue("V"),
		C:  r.FormValue("C"),
		R:  r.FormValue("R"),
		N:  r.FormValue("N"),
		SF: r.FormValue("SF"),
		S:  r.FormValue("S"),
		RO: r.FormValue("RO"),
		O:  r.FormValue("O"),
		P:  r.FormValue("P"),
		M:  r.FormValue("M"),
		JS: r.FormValue("JS"),
		RL: r.FormValue("RL"),
		RP: r.FormValue("RP"),
	}

	currency := r.FormValue("currency")
	amount := strings.TrimSpace(r.FormValue("amount"))
	pay.I = ips.FormatAmount(currency, strings.ReplaceAll(amount, ",", "."))

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

func paymentToView(pay *ips.Payment) slipView {
	currency, amount := splitAmount(pay.I)
	return slipView{
		Payer:        joinNonEmpty(pay.P, formatAccount(pay.O)),
		Purpose:      pay.S,
		Payee:        pay.N,
		PaymentCode:  pay.SF,
		Currency:     currency,
		Amount:       amount,
		PayeeAccount: formatAccount(pay.R),
		Reference:    pay.RO,
		K:            string(pay.K),
		V:            pay.V,
		C:            pay.C,
		R:            pay.R,
		N:            pay.N,
		SF:           pay.SF,
		S:            pay.S,
		RO:           pay.RO,
		O:            pay.O,
		P:            pay.P,
		M:            pay.M,
		JS:           pay.JS,
		RL:           pay.RL,
		RP:           pay.RP,
	}
}

func splitAmount(i string) (currency, amount string) {
	a, err := ips.ParseAmount(i)
	if err != nil {
		return "RSD", i
	}
	return a.Currency, strings.ReplaceAll(a.Value, ".", ",")
}

func formatAccount(s string) string {
	if len(s) != 18 {
		return s
	}
	return s[:3] + "-" + s[3:16] + "-" + s[16:]
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ── Templates ────────────────────────────────────────────────────────────────

var loginTmpl = template.Must(template.New("login").Parse(`<!DOCTYPE html>
<html lang="sr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Пријава — Паушалне уплатнице</title>
<style>` + commonCSS + `
.login-wrap { display:flex; align-items:center; justify-content:center; min-height:100vh; }
.login-box { background:#fff; border-radius:8px; box-shadow:0 2px 16px #0002; padding:40px 48px; width:320px; }
.login-box h1 { font-size:1.3rem; margin-bottom:8px; color:#1a1a2e; }
.login-box p { color:#555; font-size:.9rem; margin-bottom:24px; }
.error { color:#c0392b; font-size:.85rem; margin-bottom:16px; }
label { display:block; font-size:.85rem; color:#333; margin-bottom:6px; }
input[type=password] { width:100%; padding:10px 12px; border:1px solid #ccc; border-radius:5px; font-size:1rem; box-sizing:border-box; }
button[type=submit] { width:100%; margin-top:16px; padding:11px; background:#2563eb; color:#fff; border:none; border-radius:5px; font-size:1rem; cursor:pointer; }
button[type=submit]:hover { background:#1d4ed8; }
</style>
</head>
<body>
<div class="login-wrap">
  <div class="login-box">
    <h1>Паушалне уплатнице</h1>
    <p>Пријавите се да бисте наставили</p>
    {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
    <form method="POST" action="/login">
      <label for="password">Лозинка</label>
      <input type="password" id="password" name="password" autofocus>
      <button type="submit">Пријави се</button>
    </form>
  </div>
</div>
</body>
</html>`))

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="sr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Паушалне уплатнице</title>
<style>` + commonCSS + `
.upload-wrap { display:flex; flex-direction:column; align-items:center; padding:60px 24px; }
.upload-box { background:#fff; border-radius:10px; box-shadow:0 2px 16px #0002; padding:48px 56px; max-width:560px; width:100%; }
.upload-box h1 { font-size:1.5rem; color:#1a1a2e; margin-bottom:8px; }
.upload-box .subtitle { color:#555; margin-bottom:32px; line-height:1.5; }
.drop-zone { border:2px dashed #93c5fd; border-radius:8px; padding:32px; text-align:center; cursor:pointer; background:#eff6ff; transition:background .2s; }
.drop-zone:hover { background:#dbeafe; }
.drop-zone p { color:#2563eb; margin:0; font-size:.95rem; }
.drop-zone small { color:#6b7280; }
#file-list { margin-top:12px; font-size:.85rem; color:#374151; min-height:20px; }
.btn-process { display:block; width:100%; margin-top:24px; padding:12px; background:#2563eb; color:#fff; border:none; border-radius:6px; font-size:1rem; cursor:pointer; }
.btn-process:hover { background:#1d4ed8; }
.btn-process:disabled { background:#93c5fd; cursor:default; }
</style>
</head>
<body>
<div class="upload-wrap">
  <div class="upload-box">
    <h1>Паушалне уплатнице</h1>
    <p class="subtitle">Учитајте решења из е-Пореза за ваше паушале</p>
    <form method="POST" action="/process" enctype="multipart/form-data" id="uploadForm">
      <div class="drop-zone" onclick="document.getElementById('pdfs').click()">
        <p>Изаберите PDF фајлове</p>
        <small>до 4 фајла</small>
        <input type="file" id="pdfs" name="pdfs" accept=".pdf" multiple style="display:none" onchange="updateFileList(this)">
      </div>
      <div id="file-list"></div>
      <button type="submit" class="btn-process" id="submitBtn" disabled>Обради</button>
    </form>
  </div>
</div>
<script>
function updateFileList(input) {
  const list = document.getElementById('file-list');
  const btn = document.getElementById('submitBtn');
  const files = Array.from(input.files).slice(0, 4);
  if (files.length > 4) {
    const dt = new DataTransfer();
    files.forEach(f => dt.items.add(f));
    input.files = dt.files;
  }
  if (files.length) {
    list.innerHTML = files.map(f => '<div>📄 ' + f.name + '</div>').join('');
    btn.disabled = false;
  } else {
    list.innerHTML = '';
    btn.disabled = true;
  }
}
</script>
</body>
</html>`))

var resultsTmpl = template.Must(template.New("results").Funcs(template.FuncMap{
	"nl2br": func(s string) template.HTML {
		return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(s), "\n", "<br>"))
	},
}).Parse(`<!DOCTYPE html>
<html lang="sr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Уплатнице — Паушалне уплатнице</title>
<style>` + commonCSS + `
.page { padding:32px 24px; max-width:1000px; margin:0 auto; }
.page h1 { font-size:1.4rem; color:#1a1a2e; margin-bottom:24px; }
.back-link { display:inline-block; margin-bottom:24px; color:#2563eb; text-decoration:none; font-size:.9rem; }
.back-link:hover { text-decoration:underline; }
.warning { background:#fef3c7; border:1px solid #fcd34d; border-radius:6px; padding:12px 16px; margin-bottom:16px; color:#92400e; font-size:.85rem; }
.slip-row { display:flex; align-items:flex-start; gap:24px; margin-bottom:32px; background:#fff; border-radius:10px; box-shadow:0 2px 12px #0001; padding:20px; }
.slip-wrap { flex:1; min-width:0; }
.slip-actions { display:flex; flex-direction:column; align-items:center; justify-content:center; gap:8px; min-width:110px; }
/* PP50 slip layout */
.slip { width:100%; border:0.5mm solid #000; display:grid; grid-template-columns:55% 1fr; font-family:Arial,sans-serif; font-size:7pt; }
.slip-left { border-right:0.5mm solid #000; display:grid; grid-template-rows:auto 1fr 1fr 1fr; }
.slip-right { display:grid; grid-template-rows:auto auto auto 1fr; padding:6px; gap:4px; }
.slip-header { text-align:center; font-weight:bold; font-size:8pt; padding:4px; border-bottom:0.5mm solid #000; text-transform:uppercase; letter-spacing:.3mm; }
.slip-field { border:0.2mm solid #bbb; padding:3px 4px; margin:2px; }
.slip-label { font-size:5.5pt; color:#555; text-transform:uppercase; letter-spacing:.2mm; margin-bottom:1px; }
.slip-value { font-size:8pt; font-weight:bold; white-space:pre-line; min-height:10px; }
.slip-row-fields { display:flex; gap:4px; }
.slip-row-fields .slip-field { flex:1; }
.slip-row-fields .slip-field.narrow { flex:0 0 52px; }
.slip-qr { display:flex; align-items:flex-end; justify-content:flex-end; padding:4px; }
.qr-placeholder { border:1px dashed #aaa; border-radius:4px; padding:8px 10px; font-size:6pt; color:#888; text-align:center; width:56px; line-height:1.3; }
.amount-input, .payer-input { width:100%; border:none; background:transparent; font-size:9pt; font-weight:bold; font-family:inherit; outline:none; border-bottom:1.5px solid #2563eb; padding:0 0 1px; color:#1a1a2e; }
.amount-input:focus, .payer-input:focus { border-bottom-color:#1d4ed8; }
.payer-input { font-size:8pt; }
.btn-download { background:#2563eb; color:#fff; border:none; border-radius:6px; padding:10px 20px; font-size:.9rem; cursor:pointer; white-space:nowrap; }
.btn-download:hover { background:#1d4ed8; }
.empty { color:#6b7280; text-align:center; padding:48px 0; }
</style>
</head>
<body>
<div class="page">
  <a class="back-link" href="/">← Учитај нове фајлове</a>
  <h1>Пронађене уплатнице</h1>

  {{range .Warnings}}
  <div class="warning">⚠ {{.}}</div>
  {{end}}

  {{if not .Slips}}
  <div class="empty">Нису пронађене уплатнице у учитаним фајловима.</div>
  {{end}}

  {{range .Slips}}
  <div class="slip-row">
    <div class="slip-wrap">
      <form method="POST" action="/download">
        {{/* Hidden IPS fields */}}
        <input type="hidden" name="K"  value="{{.K}}">
        <input type="hidden" name="V"  value="{{.V}}">
        <input type="hidden" name="C"  value="{{.C}}">
        <input type="hidden" name="R"  value="{{.R}}">
        <input type="hidden" name="N"  value="{{.N}}">
        <input type="hidden" name="SF" value="{{.SF}}">
        <input type="hidden" name="S"  value="{{.S}}">
        <input type="hidden" name="RO" value="{{.RO}}">
        <input type="hidden" name="O"  value="{{.O}}">
        {{/* P (payer name) is an editable field — rendered in the slip body below */}}
        <input type="hidden" name="M"  value="{{.M}}">
        <input type="hidden" name="JS" value="{{.JS}}">
        <input type="hidden" name="RL" value="{{.RL}}">
        <input type="hidden" name="RP" value="{{.RP}}">
        <input type="hidden" name="currency" value="{{.Currency}}">

        <div class="slip">
          <div class="slip-left">
            <div class="slip-header">Налог за уплату / Nalog za uplatu</div>
            <div class="slip-field">
              <div class="slip-label">Уплатилац / Uplatilac</div>
              <input type="text" name="P" value="{{.P}}" class="payer-input" placeholder="Ime i prezime / naziv firme">
            </div>
            <div class="slip-field">
              <div class="slip-label">Сврха уплате / Svrha uplate</div>
              <div class="slip-value">{{.Purpose}}</div>
            </div>
            <div class="slip-field">
              <div class="slip-label">Прималац / Primalac</div>
              <div class="slip-value">{{.Payee}}</div>
            </div>
          </div>
          <div class="slip-right">
            <div class="slip-row-fields">
              <div class="slip-field narrow">
                <div class="slip-label">Шифра плаћања</div>
                <div class="slip-value">{{.PaymentCode}}</div>
              </div>
              <div class="slip-field narrow">
                <div class="slip-label">Валута</div>
                <div class="slip-value">{{.Currency}}</div>
              </div>
              <div class="slip-field">
                <div class="slip-label">Износ / Iznos</div>
                <input type="text" name="amount" value="{{.Amount}}" class="amount-input" placeholder="нпр. 1000,00">
              </div>
            </div>
            <div class="slip-field">
              <div class="slip-label">Рачун примаоца / Racun primaoca</div>
              <div class="slip-value">{{.PayeeAccount}}</div>
            </div>
            <div class="slip-field">
              <div class="slip-label">Позив на број / Poziv na broj</div>
              <div class="slip-value">{{.Reference}}</div>
            </div>
            <div class="slip-qr">
              <div class="qr-placeholder">Овде ће бити QR код</div>
            </div>
          </div>
        </div>

        <div style="display:flex; justify-content:flex-end; margin-top:12px;">
          <button type="submit" class="btn-download">Преузми PDF</button>
        </div>
      </form>
    </div>
  </div>
  {{end}}
</div>
</body>
</html>`))

const commonCSS = `
* { box-sizing:border-box; margin:0; padding:0; }
body { font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif; background:#f1f5f9; color:#1a1a2e; }
`
