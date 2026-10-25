package shared

import (
	"context"
	"html/template"
	"log"
	"net/http"

	"buh/internal/i18n"
	"buh/internal/updates"
)

type ctxNoticeKey struct{}
type ctxUpdatesKey struct{}

// WithNotice injects a pending notice i18n key into the context.
func WithNotice(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, ctxNoticeKey{}, key)
}

// NoticeFromContext returns the pending notice key stored in the context, or "".
func NoticeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxNoticeKey{}).(string)
	return v
}

// WithUpdates adds the authenticated user's updates view to the request context.
func WithUpdates(ctx context.Context, view updates.View) context.Context {
	return context.WithValue(ctx, ctxUpdatesKey{}, &view)
}

// UpdatesFromContext returns the updates view stored in the request context,
// or nil for public requests.
func UpdatesFromContext(ctx context.Context) *updates.View {
	v, _ := ctx.Value(ctxUpdatesKey{}).(*updates.View)
	return v
}

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
		"T":       l.T,
		"TWith":   l.TWith,
		"lang":    l.Lang,
		"updates": func() *updates.View { return UpdatesFromContext(r.Context()) },
	})
	return t
}

func RenderTemplate(w http.ResponseWriter, r *http.Request, tmpl *template.Template, data any) {
	t := cloneWithT(tmpl, r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if v := AccessRequestFromContext(r.Context()); v.Show {
		if m, ok := data.(map[string]any); ok {
			m["AccessRequest"] = v
		}
	}
	if notice := NoticeFromContext(r.Context()); notice != "" {
		l := i18n.FromContext(r.Context())
		if m, ok := data.(map[string]any); ok {
			m["Notice"] = l.T(notice)
		}
	}
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
	l := i18n.FromContext(r.Context())
	data := map[int]map[string]any{
		http.StatusNotFound: {
			"Code":    404,
			"Title":   l.T("error.not_found_title"),
			"Message": l.T("error.not_found_message"),
		},
		http.StatusForbidden: {
			"Code":    403,
			"Title":   l.T("error.forbidden_title"),
			"Message": l.T("error.forbidden_message"),
		},
	}
	d, ok := data[code]
	if !ok {
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.WriteHeader(code)
	RenderTemplate(w, r, errPage, d)
}
