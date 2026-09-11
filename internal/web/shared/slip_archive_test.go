package shared

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"buh/internal/sliprecord"
)

func TestZipSlipPDFs(t *testing.T) {
	record := sliprecord.SlipRecord{PaymentCode: "253", Amount: "5000.00", Currency: "RSD", Purpose: "Porez / PDV", Payee: "Tax Authority", PayeeAccount: "840355353184320", Payer: "Test Entrepreneur"}
	archive, err := ZipSlipPDFs([]SlipDownload{{Record: record, EntrepreneurName: "Žika & Šić d.o.o."}, {Record: record, EntrepreneurName: "Žika & Šić d.o.o."}})
	if err != nil {
		t.Fatalf("ZipSlipPDFs: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("archive contains %d files, want 2", len(zr.File))
	}
	if zr.File[0].Name != "uplatnica-zika-sic-d-o-o-porez-pdv.pdf" || zr.File[1].Name != "uplatnica-zika-sic-d-o-o-porez-pdv-2.pdf" {
		t.Errorf("archive filenames = %q, %q", zr.File[0].Name, zr.File[1].Name)
	}
	f, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open PDF: %v", err)
	}
	defer f.Close()
	pdfHeader := make([]byte, 4)
	if _, err := io.ReadFull(f, pdfHeader); err != nil {
		t.Fatalf("read PDF header: %v", err)
	}
	if string(pdfHeader) != "%PDF" {
		t.Errorf("PDF header = %q, want %%PDF", pdfHeader)
	}
}

func TestZipSlipPDFsReservesNaturalNames(t *testing.T) {
	record := sliprecord.SlipRecord{PaymentCode: "253", Amount: "5000.00", Currency: "RSD", Purpose: "x", Payee: "Tax Authority", PayeeAccount: "840355353184320", Payer: "Test Entrepreneur"}
	downloads := []SlipDownload{
		{Record: record, EntrepreneurName: "Entrepreneur"},
		{Record: record, EntrepreneurName: "Entrepreneur"},
		{Record: sliprecord.SlipRecord{PaymentCode: "253", Amount: "5000.00", Currency: "RSD", Purpose: "x-2", Payee: "Tax Authority", PayeeAccount: "840355353184320", Payer: "Test Entrepreneur"}, EntrepreneurName: "Entrepreneur"},
	}
	archive, err := ZipSlipPDFs(downloads)
	if err != nil {
		t.Fatalf("ZipSlipPDFs: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	want := []string{
		"uplatnica-entrepreneur-x.pdf",
		"uplatnica-entrepreneur-x-3.pdf",
		"uplatnica-entrepreneur-x-2.pdf",
	}
	if len(zr.File) != len(want) {
		t.Fatalf("archive contains %d files, want %d", len(zr.File), len(want))
	}
	names := make(map[string]bool, len(zr.File))
	for i, file := range zr.File {
		if names[file.Name] {
			t.Errorf("duplicate archive filename %q", file.Name)
		}
		names[file.Name] = true
		if file.Name != want[i] {
			t.Errorf("archive filename %d = %q, want %q", i, file.Name, want[i])
		}
	}
}

func TestSlipFilenameSluggingAndFallbacks(t *testing.T) {
	tests := []struct{ name, payer, purpose, want string }{
		{"Žika / Šić, d.o.o.", "ignored", "  Porez: PDV! ", "uplatnica-zika-sic-d-o-o-porez-pdv.pdf"},
		{"", "Пера Ђурић", "", "uplatnica-pera-djuric-slip.pdf"},
		{"", "Đurić", "", "uplatnica-djuric-slip.pdf"},
		{"  / \\ ", "", "Накнада за услугу", "uplatnica-slip-naknada-za-uslugu.pdf"},
		{"Slip", "Payer Name", "Purpose", "uplatnica-slip-purpose.pdf"},
		{strings.Repeat("Official Name ", 30), "ignored", strings.Repeat("Long purpose ", 30), ""},
	}
	for _, tt := range tests {
		got := SlipFilename(tt.name, tt.payer, tt.purpose)
		if tt.want != "" {
			if got != tt.want {
				t.Errorf("SlipFilename(%q, %q, %q) = %q, want %q", tt.name, tt.payer, tt.purpose, got, tt.want)
			}
			continue
		}
		if len(got) >= 255 {
			t.Errorf("long SlipFilename length = %d, want less than 255: %q", len(got), got)
		}
	}
}

func TestSlipArchiveFilenameUsesSlipFallbacks(t *testing.T) {
	if got := SlipArchiveFilename("", "Пера Ђурић"); got != "uplatnice-pera-djuric.zip" {
		t.Errorf("SlipArchiveFilename fallback = %q", got)
	}
	if got := SlipArchiveFilename("Slip", "ignored"); got != "uplatnice-slip.zip" {
		t.Errorf("SlipArchiveFilename official name = %q", got)
	}
}
