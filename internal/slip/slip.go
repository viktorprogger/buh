package slip

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"buh/internal/ips"
	"github.com/jung-kurt/gofpdf"
	qrcode "github.com/skip2/go-qrcode"
)

//go:embed DejaVuSans.ttf
var fontRegular []byte

//go:embed DejaVuSans-Bold.ttf
var fontBold []byte

const (
	pageW = 210.0
	pageH = 100.0

	divX = 107.0 // left/right split per PP50 standard

	headerH  = 8.0
	bottomH  = 18.0                       // stamp / date strip
	contentH = pageH - headerH - bottomH  // 74mm
	rowH     = contentH / 3               // ~24.67mm per row

	codeW  = 22.0 // Šifra plaćanja
	currW  = 18.0 // Valuta
	stampW = 65.0 // Pečat i potpis width in bottom strip

	qrSz = 38.0
	qrX  = pageW - 2.0 - qrSz                      // 170mm
	qrY  = headerH + rowH + (2*rowH-qrSz)/2        // centered over rows 2-3
)

// GeneratePDF creates a PP50-format payment slip PDF at outputPath.
// The QR code is generated fresh from pay.String(), not taken from any source image.
func GeneratePDF(pay *ips.Payment, outputPath string) error {
	qrPNG, err := qrcode.Encode(pay.String(), qrcode.Medium, 400)
	if err != nil {
		return fmt.Errorf("generate QR: %w", err)
	}

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: pageW, Ht: pageH},
	})
	pdf.AddUTF8FontFromBytes("DejaVu", "", fontRegular)
	pdf.AddUTF8FontFromBytes("DejaVu", "B", fontBold)
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddPage()
	pdf.RegisterImageOptionsReader("qr", gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))

	draw(pdf, pay)

	return pdf.OutputFileAndClose(outputPath)
}

func draw(pdf *gofpdf.Fpdf, pay *ips.Payment) {
	// === Grid (light gray, thin) ===
	pdf.SetLineWidth(0.2)
	pdf.SetDrawColor(229, 231, 235)

	pdf.Rect(0, 0, pageW, pageH, "D")
	pdf.Line(0, headerH, pageW, headerH)           // below header
	pdf.Line(divX, headerH, divX, pageH)           // left/right split
	pdf.Line(0, headerH+rowH, pageW, headerH+rowH) // between rows 1-2
	pdf.Line(0, headerH+2*rowH, pageW, headerH+2*rowH) // between rows 2-3
	pdf.Line(0, pageH-bottomH, pageW, pageH-bottomH)   // above bottom strip

	// Right row 1: sub-dividers for Šifra | Valuta | Iznos
	pdf.Line(divX+codeW, headerH, divX+codeW, headerH+rowH)
	pdf.Line(divX+codeW+currW, headerH, divX+codeW+currW, headerH+rowH)

	// Right rows 2-3: QR boundary
	pdf.Line(qrX-1, headerH+rowH, qrX-1, pageH-bottomH)

	// Bottom strip sub-dividers
	pdf.Line(stampW, pageH-bottomH, stampW, pageH)

	// === Header ===
	pdf.SetFont("DejaVu", "B", 7.5)
	pdf.SetTextColor(75, 85, 99)
	pdf.SetXY(0, 2)
	pdf.CellFormat(pageW, headerH-2, "NALOG ZA UPLATU", "", 0, "C", false, 0, "")

	// === Left column: PP50 standard order ===
	field(pdf, 0, headerH, divX, rowH, "Uplatilac", joinNonEmpty(pay.P, formatAccount(pay.O)))
	field(pdf, 0, headerH+rowH, divX, rowH, "Svrha uplate", pay.S)
	field(pdf, 0, headerH+2*rowH, divX, rowH, "Primalac", pay.N)

	// === Right column ===
	// Row 1: Šifra | Valuta | Iznos
	currency, amount := splitAmountParts(pay.I)
	field(pdf, divX, headerH, codeW, rowH, "Šifra plaćanja", pay.SF)
	field(pdf, divX+codeW, headerH, currW, rowH, "Valuta", currency)
	fieldLarge(pdf, divX+codeW+currW, headerH, pageW-divX-codeW-currW, rowH, "Iznos", amount)

	// Rows 2-3: account and reference (narrowed for QR)
	narrowW := qrX - 1 - divX
	field(pdf, divX, headerH+rowH, narrowW, rowH, "Račun primaoca", formatAccount(pay.R))
	field(pdf, divX, headerH+2*rowH, narrowW, rowH, "Model i poziv na broj (odobrenje)", pay.RO)

	// QR code centered over rows 2-3
	pdf.ImageOptions("qr", qrX, qrY, qrSz, qrSz, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// === Bottom strip (cashier fills in) ===
	field(pdf, 0, pageH-bottomH, stampW, bottomH, "Pečat i potpis uplatilaca", "")
	field(pdf, stampW, pageH-bottomH, divX-stampW, bottomH, "Mesto i datum prijema", "")
	field(pdf, divX, pageH-bottomH, pageW-divX, bottomH, "Datum valute", "")
}

// field renders a label + value pair inside a grid cell.
func field(pdf *gofpdf.Fpdf, x, y, w, h float64, label, value string) {
	const pad = 2.5

	pdf.SetFont("DejaVu", "", 5.5)
	pdf.SetTextColor(75, 85, 99)
	pdf.SetXY(x+pad, y+1.5)
	pdf.Cell(w-2*pad, 3.5, label)

	if value == "" {
		return
	}

	pdf.SetFont("DejaVu", "", 9)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetXY(x+pad, y+5.5)
	pdf.MultiCell(w-2*pad, 4.5, value, "", "L", false)
}

// fieldLarge renders a field with a large bold value (used for Iznos).
func fieldLarge(pdf *gofpdf.Fpdf, x, y, w, h float64, label, value string) {
	const pad = 2.5

	pdf.SetFont("DejaVu", "", 5.5)
	pdf.SetTextColor(75, 85, 99)
	pdf.SetXY(x+pad, y+1.5)
	pdf.Cell(w-2*pad, 3.5, label)

	if value == "" {
		return
	}

	pdf.SetFont("DejaVu", "B", 15)
	pdf.SetTextColor(31, 41, 55)
	pdf.SetXY(x+pad, y+6.5)
	pdf.Cell(w-2*pad, 8, value)
}

func formatAccount(s string) string {
	if len(s) != 18 {
		return s
	}
	return s[:3] + "-" + s[3:16] + "-" + s[16:]
}

func splitAmountParts(i string) (currency, amount string) {
	a, err := ips.ParseAmount(i)
	if err != nil {
		return "", i
	}
	return a.Currency, strings.ReplaceAll(a.Value, ".", ",")
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
