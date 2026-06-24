package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"buh/internal/extractor"
	"buh/internal/ips"
	"buh/internal/slip"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <file.pdf> [file2.pdf ...]",
	Short: "Extract IPS QR codes from Serbian APR tax PDFs and generate payment slips",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, path := range args {
			fmt.Printf("=== %s ===\n", path)
			if err := processFile(path); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
		}
		return nil
	},
}

func processFile(pdfPath string) error {
	pageCount, codes, err := extractor.ExtractQRCodes(pdfPath)
	if err != nil {
		return err
	}
	fmt.Printf("pages: %d\n", pageCount)

	slipIdx := 1
	for i, code := range codes {
		fmt.Printf("  QR #%d: %s\n", i+1, code)
		printIPS(code)
		writeSlip(code, pdfPath, slipIdx)
		slipIdx++
	}
	return nil
}

func printIPS(qrText string) {
	if !ips.IsIPS(qrText) {
		return
	}
	pay, err := ips.Parse(qrText)
	if err != nil {
		fmt.Printf("    [IPS parse error: %v]\n", err)
		return
	}
	if err := pay.Validate(); err != nil {
		fmt.Printf("    [IPS validation errors: %v]\n", err)
	} else {
		fmt.Printf("    [IPS NBS — valid]\n")
	}
	for _, f := range pay.Fields() {
		fmt.Printf("    %-38s %s\n", f.Name+"  ("+f.Tag+"):", f.Value)
	}
}

func writeSlip(qrText, pdfPath string, idx int) {
	if !ips.IsIPS(qrText) {
		return
	}
	pay, err := ips.Parse(qrText)
	if err != nil {
		return
	}
	dir := filepath.Dir(pdfPath)
	base := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	outPath := filepath.Join(dir, fmt.Sprintf("%s-slip-%d.pdf", base, idx))

	if err := slip.GeneratePDF(pay, outPath); err != nil {
		fmt.Printf("    [slip error: %v]\n", err)
	} else {
		fmt.Printf("    [slip → %s]\n", filepath.Base(outPath))
	}
}
