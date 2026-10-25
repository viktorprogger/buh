package accountant

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"buh/internal/accessrequest"
	acctcore "buh/internal/accountant"
	"buh/internal/auth"
	"buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/invitation"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/sliphistory"
	"buh/internal/slipmerge"
	"buh/internal/sliprecord"
	"buh/internal/uploadqueue"
	"buh/internal/web/shared"
)

type Handler struct {
	sessions           *auth.SessionManager
	accountants        *acctcore.Repo
	entrepreneurs      *entrepreneur.Repo
	slips              *sliprecord.Repo
	slipHistory        *sliphistory.Repo
	slipMerges         *slipmerge.Repo
	kpoBooks           *kpo.Repo
	uploadQueue        *uploadqueue.Repo
	entrepreneurUsers  *entrepreneuruser.Repo
	invitations        *invitation.Repo
	invoices           *invoice.Repo
	tmpl              shared.Templates
	accessRequests     *accessrequest.Service
}

func NewHandler(
	sessions *auth.SessionManager,
	accountants *acctcore.Repo,
	entrepreneurs *entrepreneur.Repo,
	slips *sliprecord.Repo,
	slipHistory *sliphistory.Repo,
	slipMerges *slipmerge.Repo,
	kpoBooks *kpo.Repo,
	uploadQueue *uploadqueue.Repo,
	entrepreneurUsers *entrepreneuruser.Repo,
	invitations *invitation.Repo,
	invoices *invoice.Repo,
	tmpl shared.Templates,
	access ...*accessrequest.Service,
) *Handler {
	var accessService *accessrequest.Service
	if len(access) > 0 {
		accessService = access[0]
	}
	return &Handler{
		sessions:          sessions,
		accountants:       accountants,
		entrepreneurs:     entrepreneurs,
		slips:             slips,
		slipHistory:       slipHistory,
		slipMerges:        slipMerges,
		kpoBooks:          kpoBooks,
		uploadQueue:       uploadQueue,
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		invoices:          invoices,
		tmpl:              tmpl,
		accessRequests:    accessService,
	}
}

const accessRequestAccountantEmail = "viktorprogger@gmail.com"

func shouldShowAccessRequestBanner(email string) bool {
	return email == accessRequestAccountantEmail
}

func (h *Handler) isAccessRequestAccountant(r *http.Request, id uuid.UUID) bool {
	if h.accountants == nil {
		return false
	}
	accountant, err := h.accountants.FindByID(r.Context(), id.String())
	return err == nil && shouldShowAccessRequestBanner(accountant.Email)
}

func (h *Handler) accountantFromSession(r *http.Request) (uuid.UUID, bool) {
	sess, ok := h.sessions.Get(r)
	if !ok || sess.UserType != auth.UserTypeAccountant {
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
	mux.HandleFunc("POST /process", h.handleProcess)
	mux.HandleFunc("GET /import/batches/{id}", h.handleBatchStatus)
	mux.HandleFunc("GET /entrepreneurs/new", h.handleEntrepreneurNewForm)
	mux.HandleFunc("POST /entrepreneurs/new", h.handleEntrepreneurNewSubmit)
	mux.HandleFunc("GET /entrepreneurs/{id}", h.handleEntrepreneur)
	mux.HandleFunc("POST /entrepreneurs/{id}", h.handleEntrepreneurUpdate)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries", h.handleKPOAddEntry)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/reorder", h.handleKPOReorderEntries)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/update", h.handleKPOUpdateEntry)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/entries/{entryID}/delete", h.handleKPODeleteEntry)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/open-year", h.handleKPOOpenYear)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/finalize", h.handleKPOFinalize)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/unfinalize", h.handleKPOUnfinalize)
	mux.HandleFunc("GET /entrepreneurs/{id}/kpo/{year}/pdf", h.handleKPOPDF)
	mux.HandleFunc("GET /entrepreneurs/{id}/kpo/{year}/merge", h.handleKPOMergeView)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/merge/copy/{eid}", h.handleKPOMergeCopyEntry)
	mux.HandleFunc("POST /entrepreneurs/{id}/kpo/{year}/merge/done", h.handleKPOMergeMarkDone)
	mux.HandleFunc("GET /entrepreneurs/{id}/slips/new", h.handleSlipNewForm)
	mux.HandleFunc("POST /entrepreneurs/{id}/slips/new", h.handleSlipNewSubmit)
	mux.HandleFunc("GET /entrepreneurs/{id}/slips/merge/{year}", h.handleSlipMergeView)
	mux.HandleFunc("POST /entrepreneurs/{id}/slips/merge/{year}/pick", h.handleSlipMergePickSide)
	mux.HandleFunc("POST /entrepreneurs/{id}/slips/merge/{year}/done", h.handleSlipMergeDone)
	mux.HandleFunc("POST /entrepreneurs/{id}/unpair", h.handleUnpair)
	mux.HandleFunc("POST /entrepreneurs/{id}/invite", h.handleAccountantInviteEntrepreneur)
	mux.HandleFunc("POST /slips/download", h.handleBulkSlipDownload)
	mux.HandleFunc("GET /slips/{id}", h.handleSlip)
	mux.HandleFunc("POST /slips/{id}/save", h.handleSlipSave)
	mux.HandleFunc("POST /slips/{id}/download", h.handleSlipDownload)
	mux.HandleFunc("POST /slips/{id}/delete", h.handleSlipDelete)
	mux.HandleFunc("GET /slips/{id}/pdf", h.handleSlipPDF)
	mux.HandleFunc("/", h.handleAccountantIndex)
	mux.HandleFunc("POST /notices/dismiss", h.handleDismissNotice)
	mux.HandleFunc("POST /access-request", h.handleAccessRequest)
	return h.noticeMiddleware(mux)
}

func (h *Handler) noticeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accountantID, ok := h.accountantFromSession(r); ok {
			if notice, err := h.accountants.GetPendingNotice(r.Context(), accountantID.String()); err == nil && notice != "" {
				r = r.WithContext(shared.WithNotice(r.Context(), notice))
			}
			if h.accessRequests != nil && h.isAccessRequestAccountant(r, accountantID) {
				r = r.WithContext(shared.WithAccessRequest(r.Context(), shared.AccessRequestView{Show: true, Success: r.URL.Query().Get("access_submitted") == "1"}))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleAccessRequest(w http.ResponseWriter, r *http.Request) {
	_, ok := h.accountantFromSession(r)
	if !ok || h.accessRequests == nil {
		h.renderError(w, r, http.StatusForbidden)
		return
	}
	if err := h.accessRequests.Store.Append(r.PostFormValue("email")); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, accessrequest.ErrInvalidEmail) {
			code = http.StatusBadRequest
		}
		http.Error(w, "invalid or unavailable request", code)
		return
	}
	h.accessRequests.Notify()
	http.Redirect(w, r, "/a/?access_submitted=1", http.StatusSeeOther)
}

func (h *Handler) handleDismissNotice(w http.ResponseWriter, r *http.Request) {
	if accountantID, ok := h.accountantFromSession(r); ok {
		_ = h.accountants.ClearPendingNotice(r.Context(), accountantID.String())
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/a/"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}
