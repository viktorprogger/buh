package shared

import (
	"html/template"
	"io/fs"
	"math"
	"time"

	"github.com/google/uuid"

	"buh/internal/sliprecord"
)

type Templates struct {
	Login             *template.Template
	Index             *template.Template
	Entrepreneur      *template.Template
	EntrepreneurNew   *template.Template
	Slip              *template.Template
	SlipNew           *template.Template
	Placeholder       *template.Template
	ErrPage           *template.Template
	PausalLimitInfo   *template.Template
	VATLimitInfo      *template.Template
	ClientForm        *template.Template
	BankAccountForm   *template.Template
	CorrespondentForm *template.Template
	InvoiceNew        *template.Template
	InvoiceDetail     *template.Template
	UploadBatch *template.Template
	// Entrepreneur-side templates
	EntrepreneurRegister     *template.Template
	EntrepreneurDashboard    *template.Template
	InviteToken              *template.Template
	InviteAccept             *template.Template
	EntrepreneurInviteAcc    *template.Template
	KpoMerge                 *template.Template
	EntrepreneurKPO          *template.Template
	EntrepreneurClients      *template.Template
	EntrepreneurBankAccounts *template.Template
	EntrepreneurInvoices     *template.Template
	EntrepreneurProfile      *template.Template
}

// i18nPlaceholder is a stub T function used at parse time.
// At render time it is replaced with the real localizer via CloneWithT.
func i18nPlaceholder(key string) string { return key }

// i18nPlaceholderWith is a stub TWith function used at parse time.
func i18nPlaceholderWith(key string, _ ...interface{}) string { return key }

// baseFuncs are always included in every template's FuncMap.
var baseFuncs = template.FuncMap{
	"T":     i18nPlaceholder,
	"TWith": i18nPlaceholderWith,
	"lang":  func() string { return "sr" },
}

func mergedFuncs(extra template.FuncMap) template.FuncMap {
	m := make(template.FuncMap, len(baseFuncs)+len(extra))
	for k, v := range baseFuncs {
		m[k] = v
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func mustPageTmpl(tfs fs.FS, name string, funcs template.FuncMap, extra ...string) *template.Template {
	t := template.New(name).Funcs(mergedFuncs(funcs))
	files := append([]string{"templates/base.html", "templates/" + name}, extra...)
	return template.Must(t.ParseFS(tfs, files...))
}

var entrepreneurBaseFuncs = template.FuncMap{
	"currentYear": func() int { return time.Now().Year() },
}

func mustEntrepreneurPageTmpl(tfs fs.FS, name string, extra ...string) *template.Template {
	t := template.New(name).Funcs(mergedFuncs(entrepreneurBaseFuncs))
	files := append([]string{"templates/entrepreneur_base.html", "templates/" + name}, extra...)
	return template.Must(t.ParseFS(tfs, files...))
}

func ParseTemplates(tfs fs.FS) Templates {
	slipExtra := []string{"templates/slip_styles.html"}
	entrepreneurFuncs := template.FuncMap{
		"slipTableArgs": func(entrepreneurID uuid.UUID, slips []sliprecord.SlipRecord) SlipTableData {
			return SlipTableData{EntrepreneurID: entrepreneurID, Slips: slips}
		},
		"fmtMoney": func(f float64) string {
			return FormatIntWithSpaces(int64(math.Round(f)))
		},
	}
	return Templates{
		Login:           template.Must(template.New("login.html").Funcs(baseFuncs).ParseFS(tfs, "templates/login.html")),
		Index:           mustPageTmpl(tfs, "index.html", nil),
		Entrepreneur:    mustPageTmpl(tfs, "entrepreneur.html", entrepreneurFuncs, "templates/slip_table.html", "templates/pausal_alert.html", "templates/vat_alert.html"),
		EntrepreneurNew: mustPageTmpl(tfs, "entrepreneur_new.html", nil),
		UploadBatch:     mustPageTmpl(tfs, "upload_batch.html", nil),
		Slip:            mustPageTmpl(tfs, "slip.html", nil, slipExtra...),
		SlipNew:         mustPageTmpl(tfs, "slip_new.html", nil, slipExtra...),
		Placeholder:     mustPageTmpl(tfs, "placeholder.html", nil),
		ErrPage:         mustPageTmpl(tfs, "error.html", nil),
		PausalLimitInfo: mustPageTmpl(tfs, "pausal_limit_info.html", nil),
		VATLimitInfo:    mustPageTmpl(tfs, "vat_limit_info.html", nil),
		ClientForm:      mustEntrepreneurPageTmpl(tfs, "client_form.html"),
		BankAccountForm: mustEntrepreneurPageTmpl(tfs, "bank_account_form.html"),
		CorrespondentForm:        mustEntrepreneurPageTmpl(tfs, "correspondent_form.html"),
		InvoiceNew:               mustEntrepreneurPageTmpl(tfs, "invoice_new.html"),
		InvoiceDetail:            mustEntrepreneurPageTmpl(tfs, "invoice.html"),
		EntrepreneurRegister:     template.Must(template.New("entrepreneur_register.html").Funcs(baseFuncs).ParseFS(tfs, "templates/entrepreneur_register.html")),
		EntrepreneurDashboard:    mustEntrepreneurPageTmpl(tfs, "entrepreneur_dashboard.html"),
		InviteToken:              mustPageTmpl(tfs, "invite_token.html", nil),
		InviteAccept:             mustPageTmpl(tfs, "invite_accept.html", nil),
		EntrepreneurInviteAcc:    mustEntrepreneurPageTmpl(tfs, "entrepreneur_invite_accountant.html"),
		KpoMerge:                 mustPageTmpl(tfs, "kpo_merge.html", template.FuncMap{"add": func(a, b int) int { return a + b }}),
		EntrepreneurKPO:          mustEntrepreneurPageTmpl(tfs, "entrepreneur_kpo.html", "templates/pausal_alert.html", "templates/vat_alert.html"),
		EntrepreneurClients:      mustEntrepreneurPageTmpl(tfs, "entrepreneur_clients.html"),
		EntrepreneurBankAccounts: mustEntrepreneurPageTmpl(tfs, "entrepreneur_bank_accounts.html"),
		EntrepreneurInvoices:     mustEntrepreneurPageTmpl(tfs, "entrepreneur_invoices.html"),
		EntrepreneurProfile:      mustEntrepreneurPageTmpl(tfs, "entrepreneur_profile.html"),
	}
}
