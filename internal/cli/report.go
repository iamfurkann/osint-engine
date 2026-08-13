package cli

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/report"
	"github.com/spf13/cobra"
)

var reportFormat string
var reportOutput string

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Araştırma raporu oluşturur",
	}

	cmd.AddCommand(newReportGenerateCmd())

	return cmd
}

func newReportGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <inv-id>",
		Short: "Belirtilen araştırma için rapor dosyası oluşturur",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			invID := args[0]

			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor. Rapor verilerini almak için 'osintd start' ile daemon'ı başlatın")
			}

			// Daemon'dan rapor verisini çek
			res, err := client.Call("investigation.report", map[string]string{"id": invID})
			if err != nil {
				return err
			}

			var reportData report.ReportData
			if err := json.Unmarshal(res, &reportData); err != nil {
				return fmt.Errorf("rapor verisi okunamadı: %w", err)
			}

			if reportFormat != "html" {
				return fmt.Errorf("sadece 'html' formatı destekleniyor")
			}

			if reportOutput == "" {
				reportOutput = fmt.Sprintf("report_%s.html", invID)
			}

			gen := report.NewGenerator()
			if err := gen.GenerateHTML(reportData, reportOutput); err != nil {
				return err
			}

			color.Green("✅ Rapor oluşturuldu: %s", reportOutput)
			return nil
		},
	}

	cmd.Flags().StringVar(&reportFormat, "format", "html", "Rapor formatı (html)")
	cmd.Flags().StringVar(&reportOutput, "out", "", "Çıktı dosyası yolu (varsayılan: report_<id>.html)")

	return cmd
}
