package shared

import (
	"html/template"
	"log"
	"net/http"

	"buh/internal/i18n"
)

// cloneWithT returns a clone of tmpl with the real T/TWith functions injected
// from the request's localizer. Cloning is needed because FuncMap entries are
// per-template instance; injecting into the original would be a data race.
func cloneWithT(tmpl *template.Template, r *http.Request) *template.Template {
	l := i18n.FromContext(r.Context())
	t, err := tmpl.Clone()
	if err != nil {
		log.Printf("template clone error: %v", err)
		return tmpl
	}
	t.Funcs(template.FuncMap{
		"T":     l.T,
		"TWith": l.TWith,
		"lang":  l.Lang,
	})
	return t
}

func RenderTemplate(w http.ResponseWriter, r *http.Request, tmpl *template.Template, data any) {
	t := cloneWithT(tmpl, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func RenderNamedTemplate(w http.ResponseWriter, r *http.Request, tmpl *template.Template, name string, data any) {
	t := cloneWithT(tmpl, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func RenderError(w http.ResponseWriter, r *http.Request, errPage *template.Template, code int) {
	type errData struct {
		Code    int
		Title   string
		Message string
	}
	l := i18n.FromContext(r.Context())
	data := map[int]errData{
		http.StatusNotFound:  {404, l.T("error.not_found_title"), l.T("error.not_found_message")},
		http.StatusForbidden: {403, l.T("error.forbidden_title"), l.T("error.forbidden_message")},
	}
	d, ok := data[code]
	if !ok {
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.WriteHeader(code)
	RenderTemplate(w, r, errPage, d)
}
