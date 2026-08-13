package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/domain"
)

// Format, çıktı formatını temsil eder.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// ParseFormat, string'i Format'a çevirir. Geçersizse hata döner.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(s) {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "csv":
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unsupported output format: %q (supported: table, json, csv)", s)
	}
}

// Formatter, bulguları belirli bir formatta çıktılayan arayüzdür.
type Formatter interface {
	Render(w io.Writer, findings []domain.Finding) error
}

// NewFormatter, belirtilen formata göre Formatter döndürür.
func NewFormatter(format Format) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{}
	case FormatCSV:
		return &CSVFormatter{}
	default:
		return &TableFormatter{}
	}
}

// --- Table Formatter ---

// TableFormatter, renkli terminal tablosu oluşturur.
type TableFormatter struct{}

func (f *TableFormatter) Render(w io.Writer, findings []domain.Finding) error {
	if len(findings) == 0 {
		fmt.Fprintln(w, color.YellowString("  Sonuç bulunamadı."))
		return nil
	}

	// Başlık
	header := color.New(color.FgCyan, color.Bold)
	separator := color.New(color.FgHiBlack)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	header.Fprintf(tw, "  TİP\tDEĞER\tKAYNAK\tTARİH\n")
	separator.Fprintf(tw, "  ───\t─────\t──────\t────\n")

	for _, finding := range findings {
		typeColor := getTypeColor(string(finding.Type))
		typeColor.Fprintf(tw, "  %s", finding.Type)
		fmt.Fprintf(tw, "\t%s\t%s\t%s\n",
			finding.Value,
			finding.FoundBy,
			finding.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	return tw.Flush()
}

// getTypeColor, bulgu tipine göre renk döndürür.
func getTypeColor(typ string) *color.Color {
	switch typ {
	case "email":
		return color.New(color.FgGreen)
	case "ip", "subdomain":
		return color.New(color.FgBlue)
	case "breach":
		return color.New(color.FgRed, color.Bold)
	case "certificate":
		return color.New(color.FgYellow)
	default:
		return color.New(color.FgWhite)
	}
}

// --- JSON Formatter ---

// JSONFormatter, ham JSON çıktısı üretir (script entegrasyonu için).
type JSONFormatter struct{}

func (f *JSONFormatter) Render(w io.Writer, findings []domain.Finding) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(findings)
}

// --- CSV Formatter ---

// CSVFormatter, CSV çıktısı üretir (spreadsheet/pipeline entegrasyonu için).
type CSVFormatter struct{}

func (f *CSVFormatter) Render(w io.Writer, findings []domain.Finding) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Başlık satırı
	if err := writer.Write([]string{"ID", "InvestigationID", "Type", "Value", "FoundBy", "Context", "CreatedAt"}); err != nil {
		return err
	}

	for _, finding := range findings {
		record := []string{
			finding.ID,
			finding.InvestigationID,
			string(finding.Type),
			finding.Value,
			finding.FoundBy,
			finding.Context,
			finding.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}
