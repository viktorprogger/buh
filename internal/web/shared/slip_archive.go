package shared

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"strings"

	"buh/internal/ips"
	"buh/internal/slip"
	"buh/internal/sliprecord"
)

// SlipDownload carries the persisted slip and the official entrepreneur name.
type SlipDownload struct {
	Record           sliprecord.SlipRecord
	EntrepreneurName string
}

// Filename components are capped in ASCII bytes after slugging. This leaves
// the complete PDF name well below the common 255-byte filename limit.
const slipFilenameComponentMax = 100

func slipFilenameName(entrepreneurName, payer string) string {
	name := Slug(entrepreneurName)
	if name == "" {
		name = Slug(payer)
	}
	if name == "" {
		name = "slip"
	}
	return trimSlugComponent(name)
}

func trimSlugComponent(component string) string {
	if len(component) > slipFilenameComponentMax {
		component = component[:slipFilenameComponentMax]
	}
	return strings.TrimRight(component, "-")
}

// SlipFilename builds the common filename for both direct and archived PDFs.
// EntrepreneurName is preferred; payer is the fallback for direct users.
func SlipFilename(entrepreneurName, payer, purpose string) string {
	name := slipFilenameName(entrepreneurName, payer)
	purposeSlug := Slug(purpose)
	if purposeSlug == "" {
		purposeSlug = "slip"
	}
	return fmt.Sprintf("uplatnica-%s-%s.pdf", name, trimSlugComponent(purposeSlug))
}

// SlipArchiveFilename builds the common filename for a bulk slip ZIP archive.
// It uses the same official-name, payer, and slip fallback as SlipFilename.
func SlipArchiveFilename(entrepreneurName, payer string) string {
	return fmt.Sprintf("uplatnice-%s.zip", slipFilenameName(entrepreneurName, payer))
}

// ZipSlipPDFs generates one PDF per slip and returns them in a ZIP archive.
func ZipSlipPDFs(downloads []SlipDownload) ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	reservedNames := make(map[string]bool, len(downloads))
	for _, download := range downloads {
		reservedNames[SlipFilename(download.EntrepreneurName, download.Record.Payer, download.Record.Purpose)] = true
	}
	usedNames := make(map[string]bool, len(downloads))
	nextSuffix := make(map[string]int, len(downloads))
	for _, download := range downloads {
		pdf, naturalName, err := GenerateSlipPDF(download.Record, download.EntrepreneurName)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		name := naturalName
		if usedNames[name] {
			base := strings.TrimSuffix(naturalName, ".pdf")
			for suffix := nextSuffix[naturalName]; ; suffix++ {
				if suffix < 2 {
					suffix = 2
				}
				candidate := fmt.Sprintf("%s-%d.pdf", base, suffix)
				if !usedNames[candidate] && !reservedNames[candidate] {
					name = candidate
					nextSuffix[naturalName] = suffix + 1
					break
				}
			}
		}
		usedNames[name] = true
		entry, err := zw.Create(name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := entry.Write(pdf); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// GenerateSlipPDF renders one slip and returns its bytes and contract filename.
func GenerateSlipPDF(record sliprecord.SlipRecord, entrepreneurName string) ([]byte, string, error) {
	pay := &ips.Payment{K: ips.CodePR, V: "01", C: "1", R: record.PayeeAccount, N: record.Payee, SF: record.PaymentCode, S: record.Purpose, RO: record.Reference, P: record.Payer}
	pay.I = ips.FormatAmount(record.Currency, strings.ReplaceAll(record.Amount, ",", "."))
	tmp, err := os.CreateTemp("", "buh-slip-*.pdf")
	if err != nil {
		return nil, "", err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, "", err
	}
	defer os.Remove(tmpPath)
	if err := slip.GeneratePDF(pay, tmpPath); err != nil {
		return nil, "", err
	}
	pdf, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, "", err
	}
	return pdf, SlipFilename(entrepreneurName, record.Payer, record.Purpose), nil
}
