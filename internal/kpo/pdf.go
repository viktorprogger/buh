package kpo

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// EntrepreneurInfo holds the entrepreneur's profile data for the KPO PDF header.
type EntrepreneurInfo struct {
	Name    string
	PIB     string
	MB      string
	Address string
}

// GeneratePDF produces an A4 KPO book PDF and returns the bytes.
func GeneratePDF(book Book, entries []Entry, info EntrepreneurInfo) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Title ────────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "KNJIGA PRIHODA (KPO)", "", 1, "C", false, 0, "")

	// ── Entrepreneur info ─────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, info.Name, "", 1, "C", false, 0, "")

	infoLine := "PIB: " + info.PIB
	if info.MB != "" {
		infoLine += "   MB: " + info.MB
	}
	pdf.CellFormat(0, 5, infoLine, "", 1, "C", false, 0, "")
	if info.Address != "" {
		pdf.CellFormat(0, 5, info.Address, "", 1, "C", false, 0, "")
	}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(0, 7, fmt.Sprintf("Godina: %d", book.Year), "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// ── Table header ─────────────────────────────────────────────────────────
	wNum, wDate, wInv, wProd, wSvc, wTotal := 10.0, 28.0, 62.0, 30.0, 30.0, 30.0

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(wNum, 7, "Rbr.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(wDate, 7, "Datum naplate", "1", 0, "C", true, 0, "")
	pdf.CellFormat(wInv, 7, "Broj racuna", "1", 0, "L", true, 0, "")
	pdf.CellFormat(wProd, 7, "Prodaja robe", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wSvc, 7, "Usluge", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 7, "Ukupno (RSD)", "1", 1, "R", true, 0, "")

	// ── Table rows ────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "", 9)
	var totalProd, totalSvc float64
	for _, e := range entries {
		totalProd += e.ProductRevenue
		totalSvc += e.ServiceRevenue

		pdf.CellFormat(wNum, 6, fmt.Sprintf("%d", e.OrdinalNumber), "1", 0, "C", false, 0, "")
		pdf.CellFormat(wDate, 6, e.CollectionDate.Format("02.01.2006."), "1", 0, "C", false, 0, "")
		pdf.CellFormat(wInv, 6, e.InvoiceNumber, "1", 0, "L", false, 0, "")
		pdf.CellFormat(wProd, 6, fmt.Sprintf("%.2f", e.ProductRevenue), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wSvc, 6, fmt.Sprintf("%.2f", e.ServiceRevenue), "1", 0, "R", false, 0, "")
		pdf.CellFormat(wTotal, 6, fmt.Sprintf("%.2f", e.Total()), "1", 1, "R", false, 0, "")
	}

	// ── Totals row ────────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(245, 245, 245)
	pdf.CellFormat(wNum+wDate+wInv, 7, "UKUPNO:", "1", 0, "R", true, 0, "")
	pdf.CellFormat(wProd, 7, fmt.Sprintf("%.2f", totalProd), "1", 0, "R", true, 0, "")
	pdf.CellFormat(wSvc, 7, fmt.Sprintf("%.2f", totalSvc), "1", 0, "R", true, 0, "")
	pdf.CellFormat(wTotal, 7, fmt.Sprintf("%.2f", totalProd+totalSvc), "1", 1, "R", true, 0, "")

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "I", 8)
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
