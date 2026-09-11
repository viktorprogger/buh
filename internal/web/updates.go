package web

import (
	"net/http"

	"buh/internal/auth"
	"buh/internal/i18n"
	"buh/internal/web/shared"
)

func (h *handler) updatesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := h.sessions.Get(r)
		if !ok || (sess.UserType != auth.UserTypeAccountant && sess.UserType != auth.UserTypeEntrepreneur) {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		view, err := h.updates.View(r.Context(), sess.UserType, sess.UserID, i18n.FromContext(r.Context()).Lang())
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Opening the feed is the viewed event. Keep the pre-mark view in this
		// response so the user sees the new-item emphasis once; subsequent pages
		// render the same updates as subdued.
		if r.Method == http.MethodGet && r.URL.Path == "/updates" {
			ids := make([]string, len(view.Items))
			for i := range view.Items {
				ids[i] = view.Items[i].ID
			}
			if err := h.updates.MarkRead(r.Context(), sess.UserType, sess.UserID, ids); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		r = r.WithContext(shared.WithUpdates(r.Context(), view))
		next.ServeHTTP(w, r)
	})
}

func (h *handler) handleUpdates(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sessions.Get(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	view := shared.UpdatesFromContext(r.Context())
	if view == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	data := map[string]any{"Updates": *view}
	if sess.UserType == auth.UserTypeEntrepreneur {
		shared.RenderTemplate(w, r, h.tmpl.EntrepreneurUpdates, data)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.Updates, data)
}
