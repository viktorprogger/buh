package entrepreneur

import (
	"net/http"

	"github.com/google/uuid"

	"buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/bankaccount"
	"buh/internal/client"
	entcore "buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/importer"
	"buh/internal/invitation"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/sliphistory"
	"buh/internal/slipmerge"
	"buh/internal/sliprecord"
	"buh/internal/web/shared"
)

type Handler struct {
	sessions          *auth.SessionManager
	accountants       *accountant.Repo
	entrepreneurs     *entcore.Repo
	kpoBooks          *kpo.Repo
	clients           *client.Repo
	invoices          *invoice.Repo
	bankAccounts      *bankaccount.Repo
	entrepreneurUsers *entrepreneuruser.Repo
	invitations       *invitation.Repo
	slips             *sliprecord.Repo
	slipHistory       *sliphistory.Repo
	slipMerges        *slipmerge.Repo
	importer          *importer.Importer
	tmpl              shared.Templates
}

func NewHandler(
	sessions *auth.SessionManager,
	accountants *accountant.Repo,
	entrepreneurs *entcore.Repo,
	kpoBooks *kpo.Repo,
	clients *client.Repo,
	invoices *invoice.Repo,
	bankAccounts *bankaccount.Repo,
	entrepreneurUsers *entrepreneuruser.Repo,
	invitations *invitation.Repo,
	slips *sliprecord.Repo,
	slipHistory *sliphistory.Repo,
	slipMerges *slipmerge.Repo,
	imp *importer.Importer,
	tmpl shared.Templates,
) *Handler {
	return &Handler{
		sessions:          sessions,
		accountants:       accountants,
		entrepreneurs:     entrepreneurs,
		kpoBooks:          kpoBooks,
		clients:           clients,
		invoices:          invoices,
		bankAccounts:      bankAccounts,
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		slips:             slips,
		slipHistory:       slipHistory,
		slipMerges:        slipMerges,
		importer:          imp,
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

func (h *Handler) renderError(w http.ResponseWriter, r *http.Request, code int) {
	shared.RenderError(w, r, h.tmpl.ErrPage, code)
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
	mux.HandleFunc("GET /slips", h.handleSlipList)
	mux.HandleFunc("GET /slips/new", h.handleESlipNewForm)
	mux.HandleFunc("POST /slips/new", h.handleESlipNewSubmit)
	mux.HandleFunc("GET /slips/upload", h.handleESlipUploadForm)
	mux.HandleFunc("POST /slips/upload", h.handleESlipUploadSubmit)
	mux.HandleFunc("GET /slips/{id}", h.handleESlip)
	mux.HandleFunc("POST /slips/{id}/save", h.handleESlipSave)
	mux.HandleFunc("POST /slips/{id}/download", h.handleESlipDownload)
	mux.HandleFunc("POST /slips/{id}/delete", h.handleESlipDelete)
	mux.HandleFunc("GET /slips/{id}/pdf", h.handleESlipPDF)
	mux.HandleFunc("GET /profile", h.handleProfileForm)
	mux.HandleFunc("POST /profile", h.handleProfileUpdate)
	mux.HandleFunc("POST /unpair", h.handleUnpair)
	mux.HandleFunc("POST /notices/dismiss", h.handleDismissNotice)
	return h.noticeMiddleware(mux)
}

func (h *Handler) noticeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userID, ok := h.entrepreneurUserFromSession(r); ok {
			if notice, err := h.entrepreneurUsers.GetPendingNotice(r.Context(), userID); err == nil && notice != "" {
				r = r.WithContext(shared.WithNotice(r.Context(), notice))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleDismissNotice(w http.ResponseWriter, r *http.Request) {
	if userID, ok := h.entrepreneurUserFromSession(r); ok {
		_ = h.entrepreneurUsers.ClearPendingNotice(r.Context(), userID)
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/e/"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}
