package extractor

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/makiuchi-d/gozxing"
	mqrcode "github.com/makiuchi-d/gozxing/multi/qrcode"
)

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
