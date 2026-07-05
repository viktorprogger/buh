package accountant

import (
	"net/http"

	"github.com/google/uuid"

	"buh/internal/auth"
	"buh/internal/entrepreneur"
	"buh/internal/entrepreneuruser"
	"buh/internal/invitation"
	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/sliphistory"
	"buh/internal/sliprecord"
	"buh/internal/uploadqueue"
	"buh/internal/web/shared"
)

type Handler struct {
	sessions          *auth.SessionManager
	entrepreneurs     *entrepreneur.Repo
	slips             *sliprecord.Repo
	slipHistory       *sliphistory.Repo
	kpoBooks          *kpo.Repo
	uploadQueue       *uploadqueue.Repo
	entrepreneurUsers *entrepreneuruser.Repo
	invitations       *invitation.Repo
	invoices          *invoice.Repo
	tmpl              shared.Templates
}

func NewHandler(
	sessions *auth.SessionManager,
	entrepreneurs *entrepreneur.Repo,
	slips *sliprecord.Repo,
	slipHistory *sliphistory.Repo,
	kpoBooks *kpo.Repo,
	uploadQueue *uploadqueue.Repo,
	entrepreneurUsers *entrepreneuruser.Repo,
	invitations *invitation.Repo,
	invoices *invoice.Repo,
	tmpl shared.Templates,
) *Handler {
	return &Handler{
		sessions:          sessions,
		entrepreneurs:     entrepreneurs,
		slips:             slips,
		slipHistory:       slipHistory,
		kpoBooks:          kpoBooks,
		uploadQueue:       uploadQueue,
		entrepreneurUsers: entrepreneurUsers,
		invitations:       invitations,
		invoices:          invoices,
		tmpl:              tmpl,
	}
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
	mux.HandleFunc("POST /entrepreneurs/{id}/invite", h.handleAccountantInviteEntrepreneur)
	mux.HandleFunc("GET /slips/{id}", h.handleSlip)
	mux.HandleFunc("POST /slips/{id}/save", h.handleSlipSave)
	mux.HandleFunc("POST /slips/{id}/download", h.handleSlipDownload)
	mux.HandleFunc("POST /slips/{id}/delete", h.handleSlipDelete)
	mux.HandleFunc("GET /slips/{id}/pdf", h.handleSlipPDF)
	mux.HandleFunc("/", h.handleAccountantIndex)
	return mux
}
