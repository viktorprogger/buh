package importer

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"strings"

	"github.com/google/uuid"

	"buh/internal/entrepreneur"
	"buh/internal/extractor"
	"buh/internal/ips"
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
}

func New(entrepreneurs *entrepreneur.Repo, slips *sliprecord.Repo) *Importer {
	return &Importer{entrepreneurs: entrepreneurs, slips: slips}
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

func (imp *Importer) processFile(ctx context.Context, accountantID uuid.UUID, fh *multipart.FileHeader) ([]SlipResult, EntrepreneurResult, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при отварању")
	}

	tmp, err := os.CreateTemp("", "buh-upload-*.pdf")
	if err != nil {
		f.Close()
		return nil, EntrepreneurResult{}, fmt.Errorf("грешка при чувању")
	}
	var buf bytes.Buffer
	buf.ReadFrom(f)
	f.Close()
	tmp.Write(buf.Bytes())
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

	var slipResults []SlipResult
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

		currency, amount := splitAmount(pay.I)
		rec := sliprecord.SlipRecord{
			EntrepreneurID: e.ID,
			PaymentCode:    pay.SF,
			Amount:         amount,
			Currency:       currency,
			Purpose:        pay.S,
			PayeeAccount:   pay.R,
			Reference:      pay.RO,
			Payee:          pay.N,
			Payer:          pay.P,
		}

		saved, upsertStatus, err := imp.slips.FindOrUpdateByPurpose(ctx, rec)
		if err != nil {
			continue
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
