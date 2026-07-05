package importer

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/extractor"
	"buh/internal/ips"
	"buh/internal/sliphistory"
	"buh/internal/sliprecord"
)

const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
)

type SlipResult struct {
	Slip   sliprecord.SlipRecord
	Status string // StatusCreated | StatusUpdated | StatusUnchanged
}

type EntrepreneurResult struct {
	Entrepreneur entrepreneur.Entrepreneur
	IsNew        bool
	Slips        []SlipResult
}

type Result struct {
	Entrepreneurs []EntrepreneurResult
	Errors        []string
}

type Importer struct {
	entrepreneurs *entrepreneur.Repo
	slips         *sliprecord.Repo
	history       *sliphistory.Repo
}

func New(entrepreneurs *entrepreneur.Repo, slips *sliprecord.Repo, history *sliphistory.Repo) *Importer {
	return &Importer{entrepreneurs: entrepreneurs, slips: slips, history: history}
}

// ProcessFiles processes uploaded PDF files for the given accountant.
func (imp *Importer) ProcessFiles(ctx context.Context, accountantID uuid.UUID, files []*multipart.FileHeader) Result {
	var res Result
	eIdx := make(map[uuid.UUID]int) // entrepreneur ID → index in res.Entrepreneurs

	for _, fh := range files {
		slipResults, er, err := imp.processFile(ctx, accountantID, fh)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", fh.Filename, err))
			continue
		}
		if len(slipResults) == 0 {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: нису пронађени QR кодови", fh.Filename))
			continue
		}

		eid := er.Entrepreneur.ID
		if idx, ok := eIdx[eid]; ok {
			res.Entrepreneurs[idx].Slips = append(res.Entrepreneurs[idx].Slips, slipResults...)
		} else {
			er.Slips = slipResults
			eIdx[eid] = len(res.Entrepreneurs)
			res.Entrepreneurs = append(res.Entrepreneurs, er)
		}
	}
	return res
}

var rePurposeYear = regexp.MustCompile(`\b(20\d{2})\b`)

// extractPurposeYear returns the 4-digit year embedded in a payment purpose string, or 0 if not found.
func extractPurposeYear(purpose string) int {
	m := rePurposeYear.FindStringSubmatch(purpose)
	if m == nil {
		return 0
	}
	y := 0
	for _, ch := range m[1] {
		y = y*10 + int(ch-'0')
	}
	return y
}

func (imp *Importer) processFile(ctx context.Context, accountantID uuid.UUID, fh *multipart.FileHeader) ([]SlipResult, EntrepreneurResult, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при отварању")
	}
	var buf bytes.Buffer
	buf.ReadFrom(f)
	f.Close()
	return imp.ProcessFileData(ctx, accountantID, fh.Filename, buf.Bytes())
}

// ProcessFileData processes a PDF given its raw bytes. Used by the async upload worker.
func (imp *Importer) ProcessFileData(ctx context.Context, accountantID uuid.UUID, filename string, data []byte) ([]SlipResult, EntrepreneurResult, error) {
	tmp, err := os.CreateTemp("", "buh-upload-*.pdf")
	if err != nil {
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при чувању")
	}
	tmp.Write(data)
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	info, _ := extractor.ExtractEntrepreneurInfo(tmpPath)

	var e entrepreneur.Entrepreneur
	var isNew bool

	if info.PIB == "" {
		return nil, EntrepreneurResult{}, fmt.Errorf("није пронађен ПИБ")
	}
	e, isNew, err = imp.entrepreneurs.FindOrCreate(ctx, accountantID, info.PIB, info.Name)
	if err != nil {
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при чувању предузетника: %w", err)
	}

	_, codes, err := extractor.ExtractQRCodes(tmpPath)
	if err != nil {
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при читању QR кода: %w", err)
	}

	// Extract amounts from text for accounts where QR codes carry I:RSD0,00.
	amountByAccount, _ := extractor.ExtractAmountsByAccount(tmpPath)

	// Parse all valid IPS payments from QR codes.
	type parsedPayment struct {
		pay         *ips.Payment
		purposeYear int
	}
	var payments []parsedPayment
	for _, code := range codes {
		if !ips.IsIPS(code) {
			continue
		}
		pay, err := ips.Parse(code)
		if err != nil {
			continue
		}
		if pay.P == "" && info.Name != "" {
			pay.P = info.Name
		}
		if (pay.I == "" || pay.I == "RSD0,00") && amountByAccount != nil {
			if amt, ok := amountByAccount[pay.R]; ok {
				pay.I = amt
			}
		}
		payments = append(payments, parsedPayment{pay: pay, purposeYear: extractPurposeYear(pay.S)})
	}

	// Determine decision year (minYear) and which payments are advance.
	// When a PDF contains 2 distinct purpose-years, the older year is the decision year;
	// slips for the newer year are advance payments for the next period.
	// All slips from the same PDF are stored under the decision year.
	minYear, maxYear := 0, 0
	for _, p := range payments {
		if p.purposeYear == 0 {
			continue
		}
		if minYear == 0 || p.purposeYear < minYear {
			minYear = p.purposeYear
		}
		if p.purposeYear > maxYear {
			maxYear = p.purposeYear
		}
	}
	decisionYear := minYear
	multipleYears := minYear != 0 && maxYear != 0 && minYear != maxYear

	var slipResults []SlipResult
	for _, pp := range payments {
		advance := multipleYears && pp.purposeYear == maxYear
		currency, amount := splitAmount(pp.pay.I)
		rec := sliprecord.SlipRecord{
			EntrepreneurID: e.ID,
			PaymentCode:    pp.pay.SF,
			Amount:         amount,
			Currency:       currency,
			Purpose:        pp.pay.S,
			PayeeAccount:   pp.pay.R,
			Reference:      pp.pay.RO,
			Payee:          pp.pay.N,
			Payer:          pp.pay.P,
			Year:           decisionYear,
			Advance:        advance,
		}

		oldSlip, saved, upsertStatus, err := imp.slips.FindOrUpdateByPurpose(ctx, rec)
		if err != nil {
			continue
		}
		if imp.history != nil && upsertStatus != sliprecord.UpsertUnchanged {
			var changes map[string]any
			if upsertStatus == sliprecord.UpsertCreated {
				changes = sliprecord.Snapshot(saved)
			} else {
				changes = sliprecord.Diff(oldSlip, saved)
			}
			if logErr := imp.history.Log(ctx, saved.ID, sliphistory.EventImported, "accountant", accountantID, changes); logErr != nil {
				log.Printf("slip history log failed: %v", logErr)
			}
		}
		status := upsertStatusString(upsertStatus)
		slipResults = append(slipResults, SlipResult{Slip: saved, Status: status})
	}

	return slipResults, EntrepreneurResult{Entrepreneur: e, IsNew: isNew}, nil
}

func upsertStatusString(s sliprecord.UpsertStatus) string {
	switch s {
	case sliprecord.UpsertCreated:
		return StatusCreated
	case sliprecord.UpsertUpdated:
		return StatusUpdated
	default:
		return StatusUnchanged
	}
}

func splitAmount(i string) (currency, amount string) {
	a, err := ips.ParseAmount(i)
	if err != nil {
		return "RSD", i
	}
	return a.Currency, strings.ReplaceAll(a.Value, ".", ",")
}
