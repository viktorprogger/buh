package invoice

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// IssuerInfo holds the entrepreneur's profile data needed for the invoice PDF.
type IssuerInfo struct {
	Name        string
	PIB         string
	Address     string
	BankAccount string // legacy plain-text field (used when no structured account set)
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

// GeneratePDF produces an A4 invoice PDF and returns the bytes.
func GeneratePDF(inv Invoice, items []Item, issuer IssuerInfo, bankAcc *BankAccountInfo) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Title ────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 18)
	title := "FAKTURA"
	if inv.InvoiceType == TypeAdvance {
		title = "AVANSNA FAKTURA"
	}
	pdf.CellFormat(0, 10, title, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(0, 6, "Broj: "+inv.InvoiceNumber, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Datum izdavanja: "+inv.IssueDate.Format("02.01.2006."), "", 1, "L", false, 0, "")
	if inv.DueDate.Valid {
		pdf.CellFormat(0, 6, "Rok placanja: "+inv.DueDate.Time.Format("02.01.2006."), "", 1, "L", false, 0, "")
	}
	if inv.PeriodStart.Valid && inv.PeriodEnd.Valid {
		pdf.CellFormat(0, 6,
			"Period: "+inv.PeriodStart.Time.Format("02.01.2006.")+" - "+inv.PeriodEnd.Time.Format("02.01.2006."),
			"", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	// ── Issuer / Client columns ──────────────────────────────────────────────
	colW := 85.0

	// Build issuer lines
	issuerLines := []string{issuer.Name, "PIB: " + issuer.PIB}
	if issuer.Address != "" {
		issuerLines = append(issuerLines, issuer.Address)
	}
	if bankAcc != nil {
		if bankAcc.AccountType == "foreign" {
			if bankAcc.BankName != "" {
				issuerLines = append(issuerLines, "Banka: "+bankAcc.BankName)
			}
			issuerLines = append(issuerLines, "IBAN: "+bankAcc.IBAN)
			issuerLines = append(issuerLines, "SWIFT: "+bankAcc.SWIFT)
			if bankAcc.CorrespondentBankName != "" {
				issuerLines = append(issuerLines, "Kor. banka: "+bankAcc.CorrespondentBankName)
				issuerLines = append(issuerLines, "SWIFT: "+bankAcc.CorrespondentBankSWIFT)
				if bankAcc.CorrespondentBankAddress != "" {
					issuerLines = append(issuerLines, bankAcc.CorrespondentBankAddress)
				}
			}
		} else {
			if bankAcc.BankName != "" {
				issuerLines = append(issuerLines, bankAcc.BankName)
			}
			issuerLines = append(issuerLines, "Racun: "+bankAcc.AccountNumber)
		}
	} else if issuer.BankAccount != "" {
		issuerLines = append(issuerLines, issuer.BankAccount)
	}

	clientLines := []string{inv.ClientName}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(colW, 6, "IZDAVALAC", "", 0, "L", false, 0, "")
	pdf.CellFormat(colW, 6, "KLIJENT", "", 1, "L", false, 0, "")

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
	pdf.CellFormat(wDesc, 6, "Opis", "1", 0, "L", true, 0, "")
	pdf.CellFormat(wQty, 6, "Kol.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(wPrice, 6, "Jed. cena", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wDisc, 6, "Popust%", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 6, "Iznos", "1", 1, "R", true, 0, "")

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
	totalLabel := fmt.Sprintf("Za uplatu: %.2f %s", grandTotal, inv.Currency)
	if inv.Currency != "RSD" {
		totalLabel += fmt.Sprintf("  (≈ %.2f RSD)", inv.TotalRSD)
	}
	pdf.CellFormat(wDesc+wQty+wPrice+wDisc, 7, totalLabel, "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 7, fmt.Sprintf("%.2f", grandTotal), "1", 1, "R", true, 0, "")

	// ── Notes ────────────────────────────────────────────────────────────────
	if inv.Notes != "" {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(0, 6, "Napomena:", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, inv.Notes, "", "L", false)
	}

	// ── Legal footer ─────────────────────────────────────────────────────────
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "I", 8)
	footer := "Obveznik PDV-a nije u sistemu PDV-a - Pausalni preduzetnik."
	if inv.DueDate.Valid {
		footer = "Placanje izvrsiti u roku do " + inv.DueDate.Time.Format("02.01.2006.") + ". " + footer
	}
	pdf.MultiCell(0, 4, footer, "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
