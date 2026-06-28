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
	divX  = 110.0 // left/right column split

	headerH = 8.0
	leftRowH = (pageH - headerH) / 3 // ~30.67mm

	codeRowH = 20.0
	acctRowH = 20.0
	refRowH  = 20.0
	qrAreaY  = codeRowH + acctRowH + refRowH // 60mm

	codeW = 20.0
	currW = 20.0

	qrSize = 40.0
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
	// Outer border + vertical divider
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Rect(0, 0, pageW, pageH, "D")
	pdf.Line(divX, 0, divX, pageH)

	// Header (left column only)
	pdf.Line(0, headerH, divX, headerH)
	pdf.SetFont("DejaVu", "B", 7.5)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(0, 1.5)
	pdf.CellFormat(divX, headerH-1.5, "NALOG ZA UPLATU", "", 0, "C", false, 0, "")

	// Left: payer, purpose, payee
	fieldBox(pdf, 0, headerH+0*leftRowH, divX, leftRowH, "Uplatilac", joinNonEmpty(pay.P, formatAccount(pay.O)))
	fieldBox(pdf, 0, headerH+1*leftRowH, divX, leftRowH, "Svrha uplate", pay.S)
	fieldBox(pdf, 0, headerH+2*leftRowH, divX, leftRowH, "Primalac", pay.N)

	// Right: code row
	currency, amount := splitAmountParts(pay.I)
	fieldBox(pdf, divX, 0, codeW, codeRowH, "Sifra placanja", pay.SF)
	fieldBox(pdf, divX+codeW, 0, currW, codeRowH, "Valuta", currency)
	fieldBox(pdf, divX+codeW+currW, 0, pageW-divX-codeW-currW, codeRowH, "Iznos", amount)

	// Right: account + reference
	fieldBox(pdf, divX, codeRowH, pageW-divX, acctRowH, "Racun primaoca", formatAccount(pay.R))
	fieldBox(pdf, divX, codeRowH+acctRowH, pageW-divX, refRowH, "Poziv na broj", pay.RO)

	// QR: bottom-right, vertically centered in remaining area
	qrX := pageW - qrSize - 2.0
	qrY := qrAreaY + (pageH-qrAreaY-qrSize)/2
	pdf.ImageOptions("qr", qrX, qrY, qrSize, qrSize, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
}

func fieldBox(pdf *gofpdf.Fpdf, x, y, w, h float64, label, value string) {
	const m = 1.0 // margin from cell edge to field border

	pdf.SetLineWidth(0.2)
	pdf.SetDrawColor(180, 180, 180)
	pdf.Rect(x+m, y+m, w-2*m, h-2*m, "D")
	pdf.SetDrawColor(0, 0, 0)

	pdf.SetFont("DejaVu", "", 5.5)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(x+m+1, y+m+0.8)
	pdf.Cell(w-2*(m+1), 3.5, label)

	pdf.SetFont("DejaVu", "B", 8)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetXY(x+m+1, y+m+0.8+3.5)
	pdf.MultiCell(w-2*(m+1), 4.5, value, "", "L", false)
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
