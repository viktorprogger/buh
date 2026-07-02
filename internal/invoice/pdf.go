package invoice

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// IssuerInfo holds the entrepreneur's profile data needed for the invoice PDF.
type IssuerInfo struct {
	Name        string
	MB          string // matični broj (registration number)
	PIB         string
	Address     string
	BankAccount string // legacy plain-text field (used when no structured account set)
}

// ClientInfo holds the client's data for the invoice PDF.
type ClientInfo struct {
	Name               string
	PIB                string
	RegistrationNumber string
	Address            string
	IsForeign          bool
}

// BankAccountInfo holds structured bank account data for PDF printing.
type BankAccountInfo struct {
	AccountType   string // "local" or "foreign"
	BankName      string
	AccountNumber string
	IBAN          string
	SWIFT         string

	// Set only for foreign accounts when a correspondent bank is chosen.
	CorrespondentBankName    string
	CorrespondentBankSWIFT   string
	CorrespondentBankAddress string
}

type pdfLabels struct {
	Invoice        string
	AdvanceInvoice string
	Number         string
	IssueDate      string
	DueDate        string
	Period         string
	Issuer         string
	Client         string
	Bank           string
	CorrBank       string
	Account        string
	Description    string
	Qty            string
	UnitPrice      string
	Discount       string
	Amount         string
	TotalDue       string
	Note           string
	PaymentBy      string
	MB             string // issuer registration number label
	RegNum         string // client MB label (local clients)
	TaxIDLabel     string // client tax id / reg no. label (foreign clients)
	NoVATLine      string
	NoSignLine     string
}

var labelsSR = pdfLabels{
	Invoice:        "FAKTURA",
	AdvanceInvoice: "AVANSNA FAKTURA",
	Number:         "Broj:",
	IssueDate:      "Datum izdavanja:",
	DueDate:        "Rok placanja:",
	Period:         "Period:",
	Issuer:         "IZDAVALAC",
	Client:         "KLIJENT",
	Bank:           "Banka:",
	CorrBank:       "Kor. banka:",
	Account:        "Racun:",
	Description:    "Opis",
	Qty:            "Kol.",
	UnitPrice:      "Jed. cena",
	Discount:       "Popust%",
	Amount:         "Iznos",
	TotalDue:       "Za uplatu:",
	Note:           "Napomena:",
	PaymentBy:      "Placanje izvrsiti u roku do",
	MB:             "MB:",
	RegNum:         "MB:",
	TaxIDLabel:     "Tax ID / Reg. br.:",
	NoVATLine:      "Nije obveznik PDV-a na osnovu cl. 12 Zakona o PDV RS.",
	NoSignLine:     "Ovaj racun je generisan kompjuterski i vazecan je bez pecata i potpisa.",
}

var labelsEN = pdfLabels{
	Invoice:        "INVOICE",
	AdvanceInvoice: "ADVANCE INVOICE",
	Number:         "Number:",
	IssueDate:      "Date of Issue:",
	DueDate:        "Payment Due:",
	Period:         "Period:",
	Issuer:         "FROM",
	Client:         "TO",
	Bank:           "Bank:",
	CorrBank:       "Corr. Bank:",
	Account:        "Account:",
	Description:    "Description",
	Qty:            "Qty.",
	UnitPrice:      "Unit Price",
	Discount:       "Discount%",
	Amount:         "Amount",
	TotalDue:       "Total Due:",
	Note:           "Note:",
	PaymentBy:      "Payment due by",
	MB:             "MB:",
	RegNum:         "Reg. No.:",
	TaxIDLabel:     "Tax ID / Reg. No.:",
	NoVATLine:      "Not subject to VAT according to Article 12 of the VAT Law of the Republic of Serbia.",
	NoSignLine:     "This invoice is computer-generated and valid without stamp or signature.",
}

// GeneratePDF produces an A4 invoice PDF and returns the bytes.
func GeneratePDF(inv Invoice, items []Item, issuer IssuerInfo, client ClientInfo, bankAcc *BankAccountInfo) ([]byte, error) {
	lbl := labelsSR
	if inv.Language == "en" {
		lbl = labelsEN
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Title ────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 18)
	title := lbl.Invoice
	if inv.InvoiceType == TypeAdvance {
		title = lbl.AdvanceInvoice
	}
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, lbl.Number+" "+inv.InvoiceNumber, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, lbl.IssueDate+" "+inv.IssueDate.Format("02.01.2006."), "", 1, "L", false, 0, "")
	if inv.DueDate.Valid {
		pdf.CellFormat(0, 6, lbl.DueDate+" "+inv.DueDate.Time.Format("02.01.2006."), "", 1, "L", false, 0, "")
	}
	if inv.PeriodStart.Valid && inv.PeriodEnd.Valid {
		pdf.CellFormat(0, 6,
			lbl.Period+" "+inv.PeriodStart.Time.Format("02.01.2006.")+" - "+inv.PeriodEnd.Time.Format("02.01.2006."),
			"", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	// ── Issuer / Client columns ──────────────────────────────────────────────
	colW := 85.0

	issuerLines := []string{issuer.Name}
	if issuer.MB != "" {
		issuerLines = append(issuerLines, lbl.MB+" "+issuer.MB)
	}
	issuerLines = append(issuerLines, "PIB: "+issuer.PIB)
	if issuer.Address != "" {
		issuerLines = append(issuerLines, issuer.Address)
	}
	if bankAcc != nil {
		if bankAcc.AccountType == "foreign" {
			if bankAcc.BankName != "" {
				issuerLines = append(issuerLines, lbl.Bank+" "+bankAcc.BankName)
			}
			issuerLines = append(issuerLines, "IBAN: "+bankAcc.IBAN)
			issuerLines = append(issuerLines, "SWIFT: "+bankAcc.SWIFT)
			if bankAcc.CorrespondentBankName != "" {
				issuerLines = append(issuerLines, lbl.CorrBank+" "+bankAcc.CorrespondentBankName)
				issuerLines = append(issuerLines, "SWIFT: "+bankAcc.CorrespondentBankSWIFT)
				if bankAcc.CorrespondentBankAddress != "" {
					issuerLines = append(issuerLines, bankAcc.CorrespondentBankAddress)
				}
			}
		} else {
			if bankAcc.BankName != "" {
				issuerLines = append(issuerLines, bankAcc.BankName)
			}
			issuerLines = append(issuerLines, lbl.Account+" "+bankAcc.AccountNumber)
		}
	} else if issuer.BankAccount != "" {
		issuerLines = append(issuerLines, issuer.BankAccount)
	}

	clientLines := []string{client.Name}
	if client.IsForeign {
		if client.RegistrationNumber != "" {
			clientLines = append(clientLines, lbl.TaxIDLabel+" "+client.RegistrationNumber)
		}
	} else {
		if client.PIB != "" {
			clientLines = append(clientLines, "PIB: "+client.PIB)
		}
		if client.RegistrationNumber != "" {
			clientLines = append(clientLines, lbl.RegNum+" "+client.RegistrationNumber)
		}
	}
	if client.Address != "" {
		clientLines = append(clientLines, client.Address)
	}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(colW, 6, lbl.Issuer, "", 0, "L", false, 0, "")
	pdf.CellFormat(colW, 6, lbl.Client, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	maxLines := len(issuerLines)
	if len(clientLines) > maxLines {
		maxLines = len(clientLines)
	}
	for i := 0; i < maxLines; i++ {
		iLine := ""
		if i < len(issuerLines) {
			iLine = issuerLines[i]
		}
		cLine := ""
		if i < len(clientLines) {
			cLine = clientLines[i]
		}
		pdf.CellFormat(colW, 5, iLine, "", 0, "L", false, 0, "")
		pdf.CellFormat(colW, 5, cLine, "", 1, "L", false, 0, "")
	}
	pdf.Ln(6)

	// ── Items table ──────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	wDesc, wQty, wPrice, wDisc, wTotal := 80.0, 18.0, 28.0, 18.0, 30.0
	pdf.CellFormat(wDesc, 6, lbl.Description, "1", 0, "L", true, 0, "")
	pdf.CellFormat(wQty, 6, lbl.Qty, "1", 0, "C", true, 0, "")
	pdf.CellFormat(wPrice, 6, lbl.UnitPrice, "1", 0, "R", true, 0, "")
	pdf.CellFormat(wDisc, 6, lbl.Discount, "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 6, lbl.Amount, "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	var grandTotal float64
	for _, it := range items {
		lineTotal := it.LineTotal()
		grandTotal += lineTotal
		pdf.CellFormat(wDesc, 6, it.Description, "1", 0, "L", false, 0, "")
		pdf.CellFormat(wQty, 6, fmt.Sprintf("%.2f", it.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(wPrice, 6, fmt.Sprintf("%.2f", it.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wDisc, 6, fmt.Sprintf("%.0f%%", it.DiscountPct), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wTotal, 6, fmt.Sprintf("%.2f", lineTotal), "1", 1, "R", false, 0, "")
	}

	// Total row
	pdf.SetFont("Helvetica", "B", 10)
	totalLabel := fmt.Sprintf("%s %.2f %s", lbl.TotalDue, grandTotal, inv.Currency)
	if inv.Currency != "RSD" {
		totalLabel += fmt.Sprintf("  (≈ %.2f RSD)", inv.TotalRSD)
	}
	pdf.CellFormat(wDesc+wQty+wPrice+wDisc, 7, totalLabel, "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 7, fmt.Sprintf("%.2f", grandTotal), "1", 1, "R", true, 0, "")

	// ── Notes ────────────────────────────────────────────────────────────────
	if inv.Notes != "" {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(0, 6, lbl.Note, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, inv.Notes, "", "L", false)
	}

	// ── Footer lines (payment due, VAT disclaimer, no-sign) ──────────────────
	if inv.DueDate.Valid || inv.NoVAT || inv.NoSign {
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(110, 110, 110)
		if inv.DueDate.Valid {
			pdf.MultiCell(0, 4, lbl.PaymentBy+" "+inv.DueDate.Time.Format("02.01.2006.")+".", "", "L", false)
		}
		if inv.NoVAT {
			pdf.MultiCell(0, 4, lbl.NoVATLine, "", "L", false)
		}
		if inv.NoSign {
			pdf.MultiCell(0, 4, lbl.NoSignLine, "", "L", false)
		}
		pdf.SetTextColor(0, 0, 0)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
