package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/report"
	"github.com/spf13/cobra"
)

func newViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <inv-id>",
		Short: "Araştırma bulgularını (findings) terminalde gösterir",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			invID := args[0]

			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor. Önce 'osintd start' ile başlatın")
			}

			// Daemon'dan rapor verisini çek
			res, err := client.Call("investigation.report", map[string]string{"id": invID})
			if err != nil {
				return err
			}

			var reportData report.ReportData
			if err := json.Unmarshal(res, &reportData); err != nil {
				return fmt.Errorf("veri okunamadı: %w", err)
			}

			// Başlık
			color.Cyan("\n🔍 OSINT Engine - Araştırma Bulguları\n")
			fmt.Printf("Hedef ID: %s\n", color.YellowString(reportData.InvestigationID))
			fmt.Printf("İlerleme: %s\n\n", color.GreenString("%%%.0f", reportData.Progress))

			if len(reportData.Entities) == 0 {
				color.Red("Bu araştırma için henüz hiçbir bulgu (entity) kaydedilmemiş.")
				return nil
			}

			// Bilgi yoğunluğuna göre sırala — dolu kayıtlar üstte.
			report.SortByInformation(reportData.Entities)

			// --- KİMLİK İDDİALARI ---
			// Her iddia KAYNAĞIYLA gösterilir: aynı kullanıcı adı farklı
			// platformlarda farklı kişilere ait olabilir.
			if summary := report.IdentitySummary(reportData.Entities); len(summary) > 0 {
				color.HiWhite("👤 KİMLİK İDDİALARI")
				color.HiBlack("   ⚠ Farklı platformlardaki aynı kullanıcı adı aynı kişi olmayabilir.")
				color.HiBlack("     Her satır, o platformun iddiasıdır — doğrulanmış gerçek değil.\n")

				sw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				lastLabel := ""
				for _, a := range summary {
					label := a.Label
					if label == lastLabel {
						label = "" // aynı alanın tekrarında etiketi tekrarlama
					} else {
						lastLabel = a.Label
					}
					// Uzun biyografiler kaynak sütununu ekranın dışına itiyordu.
					val := a.Value
					if r := []rune(val); len(r) > 74 {
						val = string(r[:73]) + "…"
					}
					fmt.Fprintf(sw, "   %s\t %s\t %s\n",
						color.HiBlackString(label),
						color.HiGreenString(val),
						color.HiBlackString("← "+a.Source))
				}
				if err := sw.Flush(); err != nil {
					return err
				}
				fmt.Println()
			}

			// Tablo formatı
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.Debug)
			fmt.Fprintln(w, color.HiWhiteString("TİP\t DEĞER\t GÜVEN (%)\t KAYNAK\t BİLGİ\t"))

			for _, ent := range reportData.Entities {
				var sources string
				for i, s := range ent.Sources {
					if i > 0 {
						sources += ", "
					}
					sources += s
				}

				// Tip rengi
				var typStr string
				switch ent.Type {
				case "email", "domain", "ip", "url":
					typStr = color.HiCyanString(ent.Type)
				case "malware", "threat", "vulnerability":
					typStr = color.HiRedString(ent.Type)
				case "username", "social", "account", "profile":
					typStr = color.HiMagentaString(ent.Type)
				case "company", "organization":
					typStr = color.HiYellowString(ent.Type)
				default:
					typStr = color.HiBlueString(ent.Type)
				}

				confStr := fmt.Sprintf("%d", ent.Confidence)
				if ent.Confidence >= 90 {
					confStr = color.HiGreenString(confStr)
				} else if ent.Confidence >= 50 {
					confStr = color.HiYellowString(confStr)
				} else {
					confStr = color.HiRedString(confStr)
				}

				info := report.CompactAttributes(ent.Attributes, 4)
				fmt.Fprintf(w, "%s\t %s\t %s\t %s\t %s\t\n",
					typStr, ent.Value, confStr, sources, color.HiBlackString(info))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Println()

			return nil
		},
	}

	return cmd
}
