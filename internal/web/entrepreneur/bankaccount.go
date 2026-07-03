package entrepreneur

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"buh/internal/bankaccount"
	"buh/internal/web/shared"
)

func (h *Handler) eOwnedBankAccount(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (bankaccount.BankAccount, bool) {
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

func (h *Handler) handleBankAccountList(w http.ResponseWriter, r *http.Request) {
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

	sortCol, sortDir, page := shared.ParseListParams(r, "bank", "asc")
	sort.Slice(accounts, func(i, j int) bool {
		a, b := accounts[i], accounts[j]
		switch sortCol {
		case "type":
			return shared.LessStr(string(a.AccountType), string(b.AccountType), sortDir)
		case "number":
			numA := a.AccountNumber
			if string(a.AccountType) == "foreign" {
				numA = a.IBAN
			}
			numB := b.AccountNumber
			if string(b.AccountType) == "foreign" {
				numB = b.IBAN
			}
			return shared.LessStr(numA, numB, sortDir)
		default:
			return shared.LessStr(a.BankName, b.BankName, sortDir)
		}
	})

	list := shared.NewListState(sortCol, sortDir, page, len(accounts), "/e/bank-accounts")
	data := map[string]any{
		"Accounts": shared.PageSlice(accounts, page),
		"List":     list,
	}
	if r.Header.Get("HX-Request") == "true" {
		shared.RenderNamedTemplate(w, h.tmpl.EntrepreneurBankAccounts, "accounts-list", data)
		return
	}
	shared.RenderTemplate(w, h.tmpl.EntrepreneurBankAccounts, data)
}

func (h *Handler) handleBankAccountNewForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.entrepreneurUserFromSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	shared.RenderTemplate(w, h.tmpl.BankAccountForm, map[string]any{
		"IsNew":     true,
		"Account":   bankaccount.BankAccount{},
		"ActionURL": "/e/bank-accounts/new",
		"BackURL":   "/e/bank-accounts",
	})
}

func (h *Handler) handleBankAccountCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}
	a := bankAccountFromForm(r, userID)
	if _, err := h.bankAccounts.Create(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts", http.StatusFound)
}

func (h *Handler) handleBankAccountEditForm(w http.ResponseWriter, r *http.Request) {
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
	shared.RenderTemplate(w, h.tmpl.BankAccountForm, map[string]any{
		"IsNew":                false,
		"Account":              a,
		"ActionURL":            "/e/bank-accounts/" + aid.String(),
		"BackURL":              "/e/bank-accounts",
		"CorrespondentBaseURL": "/e/bank-accounts/" + aid.String() + "/correspondents",
	})
}

func (h *Handler) handleBankAccountUpdate(w http.ResponseWriter, r *http.Request) {
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
	a := bankAccountFromForm(r, userID)
	a.ID = aid
	if err := h.bankAccounts.Update(r.Context(), a); err != nil {
		http.Error(w, "Грешка при чувању рачуна", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+aid.String()+"/edit", http.StatusFound)
}

func (h *Handler) handleBankAccountDelete(w http.ResponseWriter, r *http.Request) {
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
	http.Redirect(w, r, "/e/bank-accounts", http.StatusFound)
}

func bankAccountFromForm(r *http.Request, userID uuid.UUID) bankaccount.BankAccount {
	at := bankaccount.TypeLocal
	if r.FormValue("account_type") == "foreign" {
		at = bankaccount.TypeForeign
	}
	return bankaccount.BankAccount{
		EntrepreneurUserID: userID,
		AccountType:        at,
		BankName:           r.FormValue("bank_name"),
		AccountNumber:      r.FormValue("account_number"),
		IBAN:               r.FormValue("iban"),
		SWIFT:              r.FormValue("swift"),
	}
}

func (h *Handler) handleCorrespondentNewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	a, ok := h.eOwnedBankAccount(w, r, userID)
	if !ok {
		return
	}
	shared.RenderTemplate(w, h.tmpl.CorrespondentForm, map[string]any{
		"BankAccount":   a,
		"IsNew":         true,
		"Correspondent": bankaccount.CorrespondentBank{},
		"ActionURL":     "/e/bank-accounts/" + a.ID.String() + "/correspondents/new",
		"BackURL":       "/e/bank-accounts/" + a.ID.String() + "/edit",
	})
}

func (h *Handler) handleCorrespondentCreate(w http.ResponseWriter, r *http.Request) {
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
		BankName:      r.FormValue("bank_name"),
		SWIFT:         r.FormValue("swift"),
		BankAddress:   r.FormValue("bank_address"),
	}
	if _, err := h.bankAccounts.CreateCorrespondent(r.Context(), cb); err != nil {
		http.Error(w, "Грешка при чувању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+a.ID.String()+"/edit", http.StatusFound)
}

func (h *Handler) handleCorrespondentEditForm(w http.ResponseWriter, r *http.Request) {
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
	shared.RenderTemplate(w, h.tmpl.CorrespondentForm, map[string]any{
		"BankAccount":   a,
		"IsNew":         false,
		"Correspondent": cb,
		"ActionURL":     "/e/bank-accounts/" + a.ID.String() + "/correspondents/" + cid.String(),
		"BackURL":       "/e/bank-accounts/" + a.ID.String() + "/edit",
	})
}

func (h *Handler) handleCorrespondentUpdate(w http.ResponseWriter, r *http.Request) {
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
		BankName:    r.FormValue("bank_name"),
		SWIFT:       r.FormValue("swift"),
		BankAddress: r.FormValue("bank_address"),
	}
	if err := h.bankAccounts.UpdateCorrespondent(r.Context(), cb); err != nil {
		http.Error(w, "Грешка при чувању кор. банке", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/e/bank-accounts/"+a.ID.String()+"/edit", http.StatusFound)
}

func (h *Handler) handleCorrespondentDelete(w http.ResponseWriter, r *http.Request) {
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

func (h *Handler) handleCorrespondentsByAccount(w http.ResponseWriter, r *http.Request) {
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
