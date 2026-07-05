package entrepreneur

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"buh/internal/client"
	"buh/internal/web/shared"
)

func (h *Handler) handleClientList(w http.ResponseWriter, r *http.Request) {
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

	sortCol, sortDir, page := shared.ParseListParams(r, "name", "asc")
	sort.Slice(clients, func(i, j int) bool {
		a, b := clients[i], clients[j]
		switch sortCol {
		case "pib":
			return shared.LessStr(a.PIB, b.PIB, sortDir)
		case "email":
			return shared.LessStr(a.Email, b.Email, sortDir)
		case "type":
			return shared.LessBool(a.IsForeign, b.IsForeign, sortDir)
		default:
			return shared.LessStr(a.Name, b.Name, sortDir)
		}
	})

	list := shared.NewListState(sortCol, sortDir, page, len(clients), "/e/clients")
	data := map[string]any{
		"Clients": shared.PageSlice(clients, page),
		"List":    list,
	}
	if r.Header.Get("HX-Request") == "true" {
		shared.RenderNamedTemplate(w, r, h.tmpl.EntrepreneurClients, "clients-list", data)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurClients, data)
}

func (h *Handler) handleClientSearch(w http.ResponseWriter, r *http.Request) {
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
		ID                 string `json:"id"`
		Name               string `json:"name"`
		PIB                string `json:"pib"`
		RegistrationNumber string `json:"registration_number"`
		Address            string `json:"address"`
		IsForeign          bool   `json:"is_foreign"`
	}
	out := make([]item, len(results))
	for i, c := range results {
		out[i] = item{
			ID:                 c.ID.String(),
			Name:               c.Name,
			PIB:                c.PIB,
			RegistrationNumber: c.RegistrationNumber,
			Address:            c.Address,
			IsForeign:          c.IsForeign,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleClientNewForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.entrepreneurUserFromSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"), "/e/clients")
	actionURL := "/e/clients/new"
	if r.URL.Query().Get("return_to") != "" {
		actionURL += "?return_to=" + r.URL.Query().Get("return_to")
	}
	shared.RenderTemplate(w, r, h.tmpl.ClientForm, map[string]any{
		"Client":    client.Client{},
		"IsNew":     true,
		"ActionURL": actionURL,
		"BackURL":   returnTo,
	})
}

func (h *Handler) handleClientNewSubmit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	r.ParseForm()
	returnTo := safeReturnTo(r.URL.Query().Get("return_to"), "/e/clients")
	actionURL := "/e/clients/new"
	if r.URL.Query().Get("return_to") != "" {
		actionURL += "?return_to=" + r.URL.Query().Get("return_to")
	}
	c := clientFromForm(r, userID)
	if errMsg := validateClient(c); errMsg != "" {
		shared.RenderTemplate(w, r, h.tmpl.ClientForm, map[string]any{
			"Client":    c,
			"IsNew":     true,
			"Error":     errMsg,
			"ActionURL": actionURL,
			"BackURL":   returnTo,
		})
		return
	}
	if _, err := h.clients.Create(r.Context(), c); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, returnTo, http.StatusFound)
}

func (h *Handler) handleClientEditForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	c, err := h.clients.FindByID(r.Context(), cid)
	if err != nil || c.EntrepreneurUserID != userID {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.ClientForm, map[string]any{
		"Client":    c,
		"IsNew":     false,
		"ActionURL": "/e/clients/" + cid.String(),
		"BackURL":   "/e/clients",
	})
}

func (h *Handler) handleClientUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		http.Error(w, "Грешка при учитавању клијента", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	r.ParseForm()
	updated := clientFromForm(r, userID)
	updated.ID = cid
	if errMsg := validateClient(updated); errMsg != "" {
		shared.RenderTemplate(w, r, h.tmpl.ClientForm, map[string]any{
			"Client":    updated,
			"IsNew":     false,
			"Error":     errMsg,
			"ActionURL": "/e/clients/" + cid.String(),
			"BackURL":   "/e/clients",
		})
		return
	}
	if err := h.clients.Update(r.Context(), updated); err != nil {
		http.Error(w, "Грешка при чувању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/clients", http.StatusFound)
}

func (h *Handler) handleClientDelete(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	cid, err := uuid.Parse(r.PathValue("cid"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	existing, err := h.clients.FindByID(r.Context(), cid)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		http.Error(w, "Грешка при учитавању клијента", http.StatusInternalServerError)
		return
	}
	if err != nil || existing.EntrepreneurUserID != userID {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	if err := h.clients.Delete(r.Context(), cid); err != nil {
		http.Error(w, "Грешка при брисању клијента", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/clients", http.StatusFound)
}

func safeReturnTo(v, fallback string) string {
	if strings.HasPrefix(v, "/e/") {
		return v
	}
	return fallback
}

func clientFromForm(r *http.Request, userID uuid.UUID) client.Client {
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

func validateClient(c client.Client) string {
	if c.Name == "" {
		return "Назив клијента је обавезан."
	}
	if c.IsForeign {
		if c.RegistrationNumber == "" {
			return "Порески / регистрациони број је обавезан за стране клијенте."
		}
	} else {
		if c.PIB == "" {
			return "ПИБ је обавезан за домаће клијенте."
		}
		if c.RegistrationNumber == "" {
			return "Матични број је обавезан за домаће клијенте."
		}
	}
	return ""
}
