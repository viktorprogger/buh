package entrepreneur

import (
	"net/http"

	"github.com/google/uuid"

	"buh/internal/auth"
	"buh/internal/bankaccount"
	"buh/internal/client"
	entcore "buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/invitation"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/web/shared"
)

type Handler struct {
	sessions          *auth.SessionManager
	entrepreneurs     *entcore.Repo
	kpoBooks          *kpo.Repo
	clients           *client.Repo
	invoices          *invoice.Repo
	bankAccounts      *bankaccount.Repo
	entrepreneurUsers *entrepreneuruser.Repo
	invitations       *invitation.Repo
	tmpl              shared.Templates
}

func NewHandler(
	sessions *auth.SessionManager,
	entrepreneurs *entcore.Repo,
	kpoBooks *kpo.Repo,
	clients *client.Repo,
	invoices *invoice.Repo,
	bankAccounts *bankaccount.Repo,
	entrepreneurUsers *entrepreneuruser.Repo,
	invitations *invitation.Repo,
	tmpl shared.Templates,
) *Handler {
	return &Handler{
		sessions:          sessions,
		entrepreneurs:     entrepreneurs,
		kpoBooks:          kpoBooks,
		clients:           clients,
		invoices:          invoices,
		bankAccounts:      bankAccounts,
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		tmpl:              tmpl,
	}
}

func (h *Handler) entrepreneurUserFromSession(r *http.Request) (uuid.UUID, bool) {
	sess, ok := h.sessions.Get(r)
	if !ok || sess.UserType != auth.UserTypeEntrepreneur {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(sess.UserID)
	return id, err == nil
}

func (h *Handler) renderError(w http.ResponseWriter, code int) {
	shared.RenderError(w, h.tmpl.ErrPage, code)
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handleDashboard)
	mux.HandleFunc("GET /invite-accountant", h.handleInviteForm)
	mux.HandleFunc("POST /invite-accountant", h.handleSendInvite)
	mux.HandleFunc("GET /clients", h.handleClientList)
	mux.HandleFunc("GET /clients/search", h.handleClientSearch)
	mux.HandleFunc("GET /clients/new", h.handleClientNewForm)
	mux.HandleFunc("POST /clients/new", h.handleClientNewSubmit)
	mux.HandleFunc("GET /clients/{cid}/edit", h.handleClientEditForm)
	mux.HandleFunc("POST /clients/{cid}", h.handleClientUpdate)
	mux.HandleFunc("POST /clients/{cid}/delete", h.handleClientDelete)
	mux.HandleFunc("GET /bank-accounts", h.handleBankAccountList)
	mux.HandleFunc("GET /bank-accounts/new", h.handleBankAccountNewForm)
	mux.HandleFunc("POST /bank-accounts/new", h.handleBankAccountCreate)
	mux.HandleFunc("GET /bank-accounts/{aid}/edit", h.handleBankAccountEditForm)
	mux.HandleFunc("POST /bank-accounts/{aid}", h.handleBankAccountUpdate)
	mux.HandleFunc("POST /bank-accounts/{aid}/delete", h.handleBankAccountDelete)
	mux.HandleFunc("GET /bank-accounts/{aid}/correspondents/new", h.handleCorrespondentNewForm)
	mux.HandleFunc("POST /bank-accounts/{aid}/correspondents/new", h.handleCorrespondentCreate)
	mux.HandleFunc("GET /bank-accounts/{aid}/correspondents/{cid}/edit", h.handleCorrespondentEditForm)
	mux.HandleFunc("POST /bank-accounts/{aid}/correspondents/{cid}", h.handleCorrespondentUpdate)
	mux.HandleFunc("POST /bank-accounts/{aid}/correspondents/{cid}/delete", h.handleCorrespondentDelete)
	mux.HandleFunc("GET /bank-accounts/{aid}/correspondents", h.handleCorrespondentsByAccount)
	mux.HandleFunc("GET /invoices", h.handleInvoiceList)
	mux.HandleFunc("GET /invoices/new", h.handleInvoiceNewForm)
	mux.HandleFunc("POST /invoices", h.handleInvoiceCreate)
	mux.HandleFunc("GET /invoices/{iid}", h.handleInvoice)
	mux.HandleFunc("GET /invoices/{iid}/pdf", h.handleInvoicePDF)
	mux.HandleFunc("GET /kpo/{year}", h.handleKPO)
	mux.HandleFunc("GET /kpo/{year}/pdf", h.handleKPOPDF)
	mux.HandleFunc("POST /kpo/{year}/entries", h.handleKPOAddEntry)
	mux.HandleFunc("POST /kpo/{year}/entries/reorder", h.handleKPOReorderEntries)
	mux.HandleFunc("POST /kpo/{year}/entries/{entryID}/update", h.handleKPOUpdateEntry)
	mux.HandleFunc("POST /kpo/{year}/entries/{entryID}/delete", h.handleKPODeleteEntry)
	mux.HandleFunc("POST /kpo/{year}/finalize", h.handleKPOFinalize)
	mux.HandleFunc("POST /kpo/{year}/unfinalize", h.handleKPOUnfinalize)
	return mux
}
