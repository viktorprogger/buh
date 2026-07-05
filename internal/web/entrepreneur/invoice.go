package entrepreneur

import (
	"database/sql"
	"encoding/json"
	htmltemplate "html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"buh/internal/invoice"
	"buh/internal/kpo"
	"buh/internal/web/shared"
)

func (h *Handler) handleInvoiceList(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	invoices, err := h.invoices.ListByEntrepreneurUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Грешка при учитавању фактура", http.StatusInternalServerError)
		return
	}

	sortCol, sortDir, page := shared.ParseListParams(r, "date", "desc")
	sort.Slice(invoices, func(i, j int) bool {
		a, b := invoices[i], invoices[j]
		switch sortCol {
		case "number":
			return shared.LessStr(a.InvoiceNumber, b.InvoiceNumber, sortDir)
		case "client":
			return shared.LessStr(a.ClientName, b.ClientName, sortDir)
		case "type":
			return shared.LessStr(string(a.InvoiceType), string(b.InvoiceType), sortDir)
		case "amount":
			return shared.LessFloat(a.TotalRSD, b.TotalRSD, sortDir)
		default: // date
			if a.IssueDate.Equal(b.IssueDate) {
				return false
			}
			if sortDir == "desc" {
				return a.IssueDate.After(b.IssueDate)
			}
			return a.IssueDate.Before(b.IssueDate)
		}
	})

	list := shared.NewListState(sortCol, sortDir, page, len(invoices), "/e/invoices")
	data := map[string]any{
		"Invoices": shared.PageSlice(invoices, page),
		"List":     list,
	}
	if r.Header.Get("HX-Request") == "true" {
		shared.RenderNamedTemplate(w, r, h.tmpl.EntrepreneurInvoices, "invoices-list", data)
		return
	}
	shared.RenderTemplate(w, r, h.tmpl.EntrepreneurInvoices, data)
}

func (h *Handler) handleInvoiceNewForm(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	accounts, _ := h.bankAccounts.ListByEntrepreneurUser(r.Context(), userID)
	for i, a := range accounts {
		cbs, _ := h.bankAccounts.ListCorrespondentsByAccount(r.Context(), a.ID)
		accounts[i].CorrespondentBanks = cbs
	}
	issuerName, issuerMB, issuerPIB, issuerAddress := "", "", "", ""
	if ent, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		issuerName = ent.Name
		issuerMB = ent.MB
		issuerPIB = ent.PIB
		issuerAddress = ent.Address
	}

	now := time.Now()
	currentYear := now.Year()
	pausalLimit := shared.PausalalLimitForYear(currentYear)
	pausalCurrentTotal, _, _ := h.kpoBooks.SumForEntrepreneurUser(r.Context(), userID, currentYear)
	daysInYear := 365.0
	if shared.IsLeapYear(currentYear) {
		daysInYear = 366
	}
	pausalYearPct := float64(now.YearDay()) / daysInYear * 100

	vatLimit := shared.VATLimitForDate(now)
	vatDailyRevenue, _ := h.kpoBooks.DailyRevenueForEntrepreneurUser(r.Context(), userID, now.AddDate(0, 0, -730))
	if vatDailyRevenue == nil {
		vatDailyRevenue = map[string]float64{}
	}
	vatDailyJSON, _ := json.Marshal(vatDailyRevenue)

	shared.RenderTemplate(w, r, h.tmpl.InvoiceNew, map[string]any{
		"BackURL":             "/e/invoices",
		"ActionURL":           "/e/invoices",
		"SettingsURL":         "/e/",
		"ClientSearchURL":     "/e/clients/search",
		"ClientNewURL":        "/e/clients/new?return_to=/e/invoices/new",
		"Today":               now.Format("2006-01-02"),
		"BankAccounts":        accounts,
		"Currencies":          []string{"RSD", "EUR", "USD", "CHF", "GBP"},
		"FXRates":             invoice.FXRates,
		"ProfileComplete":     true,
		"IssuerName":          issuerName,
		"IssuerMB":            issuerMB,
		"IssuerPIB":           issuerPIB,
		"IssuerAddress":       issuerAddress,
		"PausalCurrentTotal":  pausalCurrentTotal,
		"PausalLimit":         pausalLimit,
		"PausalYear":          currentYear,
		"PausalYearPct":       pausalYearPct,
		"VATLimit":            vatLimit,
		"VATDailyRevenue":     htmltemplate.JS(vatDailyJSON),
	})
}

func (h *Handler) handleInvoiceCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Неисправан захтев", http.StatusBadRequest)
		return
	}

	invType := invoice.TypeStandard
	if r.FormValue("invoice_type") == "advance" {
		invType = invoice.TypeAdvance
	}

	issueDate, err := time.Parse("2006-01-02", r.FormValue("issue_date"))
	if err != nil {
		issueDate = time.Now()
	}

	currency := r.FormValue("currency")
	if _, ok := invoice.FXRates[currency]; !ok {
		http.Error(w, "Непозната валута", http.StatusBadRequest)
		return
	}

	lang := r.FormValue("language")
	if lang != "en" {
		lang = "sr"
	}

	inv := invoice.Invoice{
		EntrepreneurUserID: userID,
		InvoiceType:        invType,
		InvoiceNumber:      strings.TrimSpace(r.FormValue("invoice_number")),
		IssueDate:          issueDate,
		Currency:           currency,
		Notes:              strings.TrimSpace(r.FormValue("notes")),
		Language:           lang,
		NoVAT:              r.FormValue("no_vat") == "on",
		NoSign:             r.FormValue("no_sign") == "on",
	}

	if cid, err := uuid.Parse(r.FormValue("client_id")); err == nil {
		inv.ClientID = &cid
	}
	inv.ClientName = strings.TrimSpace(r.FormValue("client_name"))

	if aid, err := uuid.Parse(r.FormValue("bank_account_id")); err == nil {
		inv.BankAccountID = &aid
	}
	if cbid, err := uuid.Parse(r.FormValue("correspondent_bank_id")); err == nil {
		inv.CorrespondentBankID = &cbid
	}

	if dd, err := time.Parse("2006-01-02", r.FormValue("due_date")); err == nil {
		inv.DueDate = sql.NullTime{Time: dd, Valid: true}
	}
	if ps, err := time.Parse("2006-01-02", r.FormValue("period_start")); err == nil {
		if pe, err := time.Parse("2006-01-02", r.FormValue("period_end")); err == nil {
			inv.PeriodStart = sql.NullTime{Time: ps, Valid: true}
			inv.PeriodEnd = sql.NullTime{Time: pe, Valid: true}
		}
	}

	descs := r.Form["item_description[]"]
	qtys := r.Form["item_quantity[]"]
	prices := r.Form["item_unit_price[]"]
	discounts := r.Form["item_discount[]"]
	isProducts := r.Form["item_is_product[]"]
	var items []invoice.Item
	for i, d := range descs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		qty, _ := strconv.ParseFloat(strings.ReplaceAll(shared.SafeIndex(qtys, i), ",", "."), 64)
		price, _ := strconv.ParseFloat(strings.ReplaceAll(shared.SafeIndex(prices, i), ",", "."), 64)
		disc, _ := strconv.ParseFloat(strings.ReplaceAll(shared.SafeIndex(discounts, i), ",", "."), 64)
		isProd := shared.SafeIndex(isProducts, i) == "on"
		items = append(items, invoice.Item{
			Description: d,
			Quantity:    qty,
			UnitPrice:   price,
			DiscountPct: disc,
			IsProduct:   isProd,
		})
	}

	var total float64
	for _, it := range items {
		total += it.LineTotal()
	}
	inv.TotalRSD = invoice.ToRSD(total, inv.Currency)

	created, err := h.invoices.Create(r.Context(), inv, items)
	if err != nil {
		http.Error(w, "Грешка при чувању фактуре", http.StatusInternalServerError)
		return
	}

	if invType == invoice.TypeStandard {
		year := issueDate.Year()
		if book, err := h.kpoBooks.FindOrCreateForEntrepreneur(r.Context(), userID, year); err == nil && !book.IsFinalized() {
			var prodRev, svcRev float64
			for _, it := range items {
				lineRSD := invoice.ToRSD(it.LineTotal(), inv.Currency)
				if it.IsProduct {
					prodRev += lineRSD
				} else {
					svcRev += lineRSD
				}
			}
			_, _ = h.kpoBooks.AddEntry(r.Context(), kpo.Entry{
				KPOBookID:      book.ID,
				CollectionDate: issueDate,
				InvoiceNumber:  inv.InvoiceNumber,
				ProductRevenue: prodRev,
				ServiceRevenue: svcRev,
			})
		}
	}

	http.Redirect(w, r, "/e/invoices/"+created.ID.String(), http.StatusFound)
}

func (h *Handler) handleInvoice(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if err != nil || inv.EntrepreneurUserID != userID {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	var grandTotal float64
	for _, it := range items {
		grandTotal += it.LineTotal()
	}

	data := map[string]any{
		"Invoice":    inv,
		"Items":      items,
		"GrandTotal": grandTotal,
		"BackURL":    "/e/invoices",
		"PDFUrl":     "/e/invoices/" + iid.String() + "/pdf",
	}
	if inv.BankAccountID != nil {
		if ba, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID); err == nil {
			data["BankAccount"] = ba
		}
	}
	if inv.CorrespondentBankID != nil {
		if cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID); err == nil {
			data["CorrespondentBank"] = cb
		}
	}
	if ent, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		data["Issuer"] = ent
	}
	if inv.ClientID != nil {
		if c, err := h.clients.FindByID(r.Context(), *inv.ClientID); err == nil {
			data["Client"] = c
		}
	}
	shared.RenderTemplate(w, r, h.tmpl.InvoiceDetail, data)
}

func (h *Handler) handleInvoicePDF(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.entrepreneurUserFromSession(r)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		h.renderError(w, r, http.StatusNotFound)
		return
	}
	inv, items, err := h.invoices.FindByID(r.Context(), iid)
	if err != nil || inv.EntrepreneurUserID != userID {
		h.renderError(w, r, http.StatusNotFound)
		return
	}

	issuer := invoice.IssuerInfo{}
	if ent, err := h.entrepreneurs.FindByEntrepreneurUserID(r.Context(), userID); err == nil {
		issuer = invoice.IssuerInfo{
			Name:        ent.Name,
			MB:          ent.MB,
			PIB:         ent.PIB,
			Address:     ent.Address,
			BankAccount: ent.BankAccount,
		}
	}

	clientInfo := invoice.ClientInfo{Name: inv.ClientName}
	if inv.ClientID != nil {
		if c, err := h.clients.FindByID(r.Context(), *inv.ClientID); err == nil {
			clientInfo = invoice.ClientInfo{
				Name:               c.Name,
				PIB:                c.PIB,
				RegistrationNumber: c.RegistrationNumber,
				Address:            c.Address,
				IsForeign:          c.IsForeign,
			}
		}
	}

	var bankAccInfo *invoice.BankAccountInfo
	if inv.BankAccountID != nil {
		if ba, err := h.bankAccounts.FindByID(r.Context(), *inv.BankAccountID); err == nil {
			info := &invoice.BankAccountInfo{
				AccountType:   string(ba.AccountType),
				BankName:      ba.BankName,
				AccountNumber: ba.AccountNumber,
				IBAN:          ba.IBAN,
				SWIFT:         ba.SWIFT,
			}
			if inv.CorrespondentBankID != nil {
				if cb, err := h.bankAccounts.FindCorrespondentByID(r.Context(), *inv.CorrespondentBankID); err == nil {
					info.CorrespondentBankName = cb.BankName
					info.CorrespondentBankSWIFT = cb.SWIFT
					info.CorrespondentBankAddress = cb.BankAddress
				}
			}
			bankAccInfo = info
		}
	}

	pdfBytes, err := invoice.GeneratePDF(inv, items, issuer, clientInfo, bankAccInfo)
	if err != nil {
		log.Printf("invoice pdf generation error: %v", err)
		http.Error(w, "PDF generation failed", http.StatusInternalServerError)
		return
	}

	filename := "faktura.pdf"
	if inv.InvoiceNumber != "" {
		safe := strings.Map(func(r rune) rune {
			if strings.ContainsRune(`/\:*?"<>|`, r) {
				return '-'
			}
			return r
		}, inv.InvoiceNumber)
		filename = "faktura-" + safe + ".pdf"
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if _, err := w.Write(pdfBytes); err != nil {
		log.Printf("invoice pdf write error: %v", err)
	}
}
