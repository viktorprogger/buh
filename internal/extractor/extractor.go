package extractor

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/makiuchi-d/gozxing"
	mqrcode "github.com/makiuchi-d/gozxing/multi/qrcode"
)

var (
	// rePIB matches "ПИБ:" followed by optional whitespace/newlines and captures the numeric ID.
	rePIB = regexp.MustCompile(`ПИБ:\s*\n?\s*(\d+)`)
	// reBusinessName matches "ПОСЛОВНО СЕДИШТЕ:" and captures the firm name on the same line.
	reBusinessName = regexp.MustCompile(`ПОСЛОВНО СЕДИШТЕ:\s*(.+)`)

	// reTextAccount matches Serbian bank account in dash format: "840-711122843-32".
	reTextAccount = regexp.MustCompile(`\b(\d{3}-\d{9}-\d{2})\b`)
	// reTextAmount matches "износи 10.512,42 динара" and captures the amount.
	reTextAmount = regexp.MustCompile(`износи\s+([\d.]+,\d{2})\s+динара`)
)

// EntrepreneurInfo holds the entrepreneur's firm name and PIB extracted from a PDF.
type EntrepreneurInfo struct {
	// Name is the firm name from the "ПОСЛОВНО СЕДИШТЕ:" field (Latin script).
	Name string
	// PIB is the entrepreneur's tax identification number (Порески идентификациони број).
	PIB string
}

// ExtractEntrepreneurInfo extracts the entrepreneur's firm name and PIB from a Serbian APR/ePorezi PDF.
// Returns an EntrepreneurInfo with empty fields (not an error) if the data cannot be found.
func ExtractEntrepreneurInfo(pdfPath string) (EntrepreneurInfo, error) {
	out, err := exec.Command("pdftotext", pdfPath, "-").Output()
	if err != nil {
		return EntrepreneurInfo{}, fmt.Errorf("pdftotext: %w", err)
	}
	text := string(out)

	var info EntrepreneurInfo

	if m := rePIB.FindStringSubmatch(text); m != nil {
		info.PIB = strings.TrimSpace(m[1])
	}
	if m := reBusinessName.FindStringSubmatch(text); m != nil {
		info.Name = strings.TrimSpace(m[1])
	}

	return info, nil
}

// ExtractQRCodes renders the last pages of a PDF and returns all QR code strings found.
func ExtractQRCodes(pdfPath string) (pageCount int, codes []string, err error) {
	pageCount, err = countPages(pdfPath)
	if err != nil {
		return 0, nil, err
	}

	tmp, err := os.MkdirTemp("", "qr-*")
	if err != nil {
		return pageCount, nil, err
	}
	defer os.RemoveAll(tmp)

	first := pageCount - 1
	if first < 1 {
		first = 1
	}

	images, err := renderPages(pdfPath, first, pageCount, tmp)
	if err != nil {
		return pageCount, nil, err
	}

	for _, img := range images {
		found, err := scanQR(img)
		if err != nil {
			continue
		}
		codes = append(codes, found...)
	}
	return pageCount, codes, nil
}

func countPages(pdfPath string) (int, error) {
	out, err := exec.Command("pdfinfo", pdfPath).Output()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			return strconv.Atoi(s)
		}
	}
	return 0, fmt.Errorf("page count not found")
}

func renderPages(pdfPath string, first, last int, dir string) ([]string, error) {
	prefix := filepath.Join(dir, "p")
	err := exec.Command("pdftoppm",
		"-r", "400",
		"-png",
		"-f", strconv.Itoa(first),
		"-l", strconv.Itoa(last),
		pdfPath, prefix,
	).Run()
	if err != nil {
		return nil, fmt.Errorf("pdftoppm: %w", err)
	}
	return filepath.Glob(filepath.Join(dir, "p-*.png"))
}

func scanQR(imagePath string) ([]string, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("bitmap: %w", err)
	}

	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	reader := mqrcode.NewQRCodeMultiReader()
	results, err := reader.DecodeMultiple(bmp, hints)
	if err != nil || len(results) == 0 {
		return nil, fmt.Errorf("decode QR: %w", err)
	}

	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.GetText()
	}
	return out, nil
}

// ExtractAmountsByAccount parses the text of a Serbian Tax Authority PAUS decision PDF and
// returns a map of 18-digit account number → IPS amount string (e.g. "RSD10512,42").
// It works by finding each bank account in the text and looking backward up to 400 characters
// for the nearest "износи X,XX динара" pattern, which is the monthly payment amount for that account.
// Returns nil (no error) when no accounts are found.
func ExtractAmountsByAccount(pdfPath string) (map[string]string, error) {
	out, err := exec.Command("pdftotext", pdfPath, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}
	text := string(out)

	accountLocs := reTextAccount.FindAllStringIndex(text, -1)
	if len(accountLocs) == 0 {
		return nil, nil
	}

	result := make(map[string]string)
	for _, loc := range accountLocs {
		account := text[loc[0]:loc[1]]
		normalized := normalizeAccount(account)
		if _, exists := result[normalized]; exists {
			continue // first match wins (body text precedes payment slip templates)
		}

		start := loc[0] - 1500
		if start < 0 {
			start = 0
		}
		window := text[start:loc[0]]
		amMatches := reTextAmount.FindAllStringSubmatch(window, -1)
		if len(amMatches) == 0 {
			continue
		}
		// last match is closest to the account
		amountStr := amMatches[len(amMatches)-1][1]
		// strip thousands-separator dots; IPS format uses comma as decimal separator
		ipsAmount := "RSD" + strings.ReplaceAll(amountStr, ".", "")
		result[normalized] = ipsAmount
	}
	return result, nil
}

// normalizeAccount converts a Serbian bank account from dash format ("840-711122843-32")
// to the 18-digit format used in IPS NBS QR codes ("840000071112284332").
func normalizeAccount(s string) string {
	stripped := strings.ReplaceAll(s, "-", "")
	if len(stripped) != 14 {
		return stripped
	}
	// bank(3) + "0000" + account(9) + control(2) = 18 digits
	return stripped[:3] + "0000" + stripped[3:]
}
