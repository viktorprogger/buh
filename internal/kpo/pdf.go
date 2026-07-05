package kpo

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"buh/internal/pdffonts"
)

// EntrepreneurInfo holds the entrepreneur's profile data for the KPO PDF header.
type EntrepreneurInfo struct {
	Name         string // firm/business name (Firma-radnje / Obveznik)
	PIB          string
	MB           string
	Address      string // business seat (Sedište)
	TaxpayerCode string // šifra poreskog obveznika
	ActivityCode string // šifra delatnosti
}

// GeneratePDF produces an A4 KPO book PDF following the official 5-column form layout.
func GeneratePDF(book Book, entries []Entry, info EntrepreneurInfo) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("dejavu", "", pdffonts.Regular)
	pdf.AddUTF8FontFromBytes("dejavu", "B", pdffonts.Bold)
	pdf.AddUTF8FontFromBytes("dejavu", "I", pdffonts.Italic)
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Title ────────────────────────────────────────────────────────────────
	pdf.SetFont("dejavu", "B", 13)
	pdf.CellFormat(0, 7, "KNJIGA O OSTVARENOM PROMETU", "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 7, "PAUSALNO OPOREZOVANIH OBVEZNIKA", "", 1, "C", false, 0, "")
	pdf.SetFont("dejavu", "", 10)
	pdf.CellFormat(0, 5, "(KPO)", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// ── Heading fields (official KPO form) ───────────────────────────────────
	pageWFull, _ := pdf.GetPageSize()
	pageW := pageWFull - 30 // left+right margins = 30mm
	half := pageW / 2
	colLabel := 50.0
	colValue := half - colLabel

	// Row 1: PIB | Obveznik/Firma-radnje
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "PIB:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.PIB, "B", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "Obveznik/Firma-radnje:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.Name, "B", 1, "L", false, 0, "")
	pdf.Ln(1)

	// Row 2: MB | Sediste
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "MB:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.MB, "B", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "Sediste:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.Address, "B", 1, "L", false, 0, "")
	pdf.Ln(1)

	// Row 3: Sifra poreskog obveznika | Sifra delatnosti
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "Sifra poreskog obveznika:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.TaxpayerCode, "B", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "B", 9)
	pdf.CellFormat(colLabel, 5, "Sifra delatnosti:", "", 0, "L", false, 0, "")
	pdf.SetFont("dejavu", "", 9)
	pdf.CellFormat(colValue, 5, info.ActivityCode, "B", 1, "L", false, 0, "")
	pdf.Ln(1)

	// Row 4: Godina
	pdf.SetFont("dejavu", "B", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Godina: %d", book.Year), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// ── Table header (5 official columns) ────────────────────────────────────
	// Col 1: serial number (narrow)
	// Col 2: date and description (wide)
	// Col 3: product revenue
	// Col 4: service revenue
	// Col 5: total
	wNum := 10.0
	wDesc := 80.0
	wProd := 30.0
	wSvc := 30.0
	wTotal := pageW - wNum - wDesc - wProd - wSvc

	pdf.SetFont("dejavu", "B", 8)
	pdf.SetFillColor(230, 230, 230)
	// Header row 1: column numbers
	pdf.CellFormat(wNum, 5, "1", "LT", 0, "C", true, 0, "")
	pdf.CellFormat(wDesc, 5, "2", "LT", 0, "C", true, 0, "")
	pdf.CellFormat(wProd, 5, "3", "LT", 0, "C", true, 0, "")
	pdf.CellFormat(wSvc, 5, "4", "LT", 0, "C", true, 0, "")
	pdf.CellFormat(wTotal, 5, "5", "LRT", 1, "C", true, 0, "")
	// Header row 2: column names
	pdf.CellFormat(wNum, 5, "Rbr.", "LB", 0, "C", true, 0, "")
	pdf.CellFormat(wDesc, 5, "Datum i opis prometa", "LB", 0, "C", true, 0, "")
	pdf.CellFormat(wProd, 5, "Prihodi od prodaje roba", "LB", 0, "C", true, 0, "")
	pdf.CellFormat(wSvc, 5, "Prihodi od usluga", "LB", 0, "C", true, 0, "")
	pdf.CellFormat(wTotal, 5, "Ukupno (3+4)", "LRB", 1, "C", true, 0, "")

	// ── Table rows ────────────────────────────────────────────────────────────
	pdf.SetFont("dejavu", "", 8)
	var totalProd, totalSvc float64
	for _, e := range entries {
		totalProd += e.ProductRevenue
		totalSvc += e.ServiceRevenue

		// Build column 2 content: date on first line, invoice + description on second
		descParts := []string{e.CollectionDate.Format("02.01.2006.")}
		if e.InvoiceNumber != "" {
			descParts = append(descParts, e.InvoiceNumber)
		}
		if e.Description != "" {
			descParts = append(descParts, e.Description)
		}
		col2 := strings.Join(descParts, " – ")

		rowH := 6.0
		pdf.CellFormat(wNum, rowH, fmt.Sprintf("%d", e.OrdinalNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(wDesc, rowH, col2, "1", 0, "L", false, 0, "")
		pdf.CellFormat(wProd, rowH, fmt.Sprintf("%.2f", e.ProductRevenue), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wSvc, rowH, fmt.Sprintf("%.2f", e.ServiceRevenue), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wTotal, rowH, fmt.Sprintf("%.2f", e.Total()), "1", 1, "R", false, 0, "")
	}

	// ── Totals row ────────────────────────────────────────────────────────────
	pdf.SetFont("dejavu", "B", 8)
	pdf.SetFillColor(245, 245, 245)
	pdf.CellFormat(wNum+wDesc, 7, "UKUPNO:", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wProd, 7, fmt.Sprintf("%.2f", totalProd), "1", 0, "R", true, 0, "")
	pdf.CellFormat(wSvc, 7, fmt.Sprintf("%.2f", totalSvc), "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 7, fmt.Sprintf("%.2f", totalProd+totalSvc), "1", 1, "R", true, 0, "")

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.Ln(6)
	pdf.SetFont("dejavu", "I", 8)
	pdf.SetTextColor(110, 110, 110)
	if book.IsFinalized() {
		pdf.CellFormat(0, 5, "Knjiga je zakljucena "+book.FinalizedAtStr(), "", 1, "L", false, 0, "")
	}
	pdf.MultiCell(0, 4,
		"Ovaj dokument je generisan kompjuterski iz sistema buh i vazecan je bez pecata i potpisa.",
		"", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
