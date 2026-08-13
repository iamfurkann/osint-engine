package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/input"
	"github.com/iamfurkann/osint-engine/internal/output"
	"github.com/spf13/cobra"
)

var (
	searchOutput  string
	searchNoCache bool
	searchBg      bool
)

func newSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <type|auto> <target>",
		Short: "Belirtilen hedef için araştırma başlatır",
		Long: `Belirtilen hedef üzerinde OSINT sorgusu çalıştırır.

Desteklenen tipler: email, domain, ip, username, hash, url, auto
'auto' kullanıldığında girdi tipi otomatik algılanır.

Örnekler:
  osint search domain example.com
  osint search email user@example.com --output json
  osint search auto 8.8.8.8
  osint search ip 8.8.8.8 --no-cache
  osint search email target@example.com --bg`,
		Args: cobra.ExactArgs(2),
		RunE: runSearch,
	}

	cmd.Flags().StringVar(&searchOutput, "output", "table", "Çıktı formatı (table, json, csv)")
	cmd.Flags().BoolVar(&searchNoCache, "no-cache", false, "Cache'i yoksay ve taze veri getir")
	cmd.Flags().BoolVar(&searchBg, "bg", false, "Araştırmayı daemon üzerinden arka planda çalıştır")

	return cmd
}

func runSearch(cmd *cobra.Command, args []string) error {
	inputTypeStr := strings.ToLower(args[0])
	target := args[1]

	// Eğer --bg seçildiyse, komutu IPC üzerinden daemon'a gönder.
	if searchBg {
		client := getIPCClient()
		if !client.IsRunning() {
			return fmt.Errorf("daemon çalışmıyor. Arka planda çalıştırmak için önce 'osintd start' ile daemon'ı başlatın")
		}

		params := map[string]string{"target": target, "type": inputTypeStr}
		res, err := client.Call("investigation.start", params)
		if err != nil {
			return err
		}

		var resp map[string]string
		if err := json.Unmarshal(res, &resp); err != nil {
			return fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
		}

		color.Green("✅ Araştırma arka planda başlatıldı (ID: %s)", resp["investigation_id"])
		fmt.Printf("Durumunu izlemek için: osint inv status %s\n", resp["investigation_id"])
		return nil
	}

	// Çıktı formatını parse et
	format, err := output.ParseFormat(searchOutput)
	if err != nil {
		return err
	}

	// Girdi tipini belirle
	var inputType input.InputType
	if inputTypeStr == "auto" {
		inputType = input.Detect(target)
		if inputType == input.TypeUnknown {
			return fmt.Errorf("girdi tipi otomatik algılanamadı: %q\nLütfen tipi belirtin: osint search <type> %s", target, target)
		}
		if !quiet {
			color.New(color.FgCyan).Fprintf(os.Stderr, "⟳ Algılanan tip: %s\n", inputType)
		}
	} else {
		inputType = input.InputType(inputTypeStr)
		if err := input.Validate(target, inputType); err != nil {
			return err
		}
	}

	// Doğrulama başarılı — aramayı başlat
	if !quiet {
		color.New(color.FgGreen, color.Bold).Fprintf(os.Stderr, "🔍 Aranıyor: ")
		fmt.Fprintf(os.Stderr, "%s (%s)\n", target, inputType)
	}

	// orchestrator.StartInvestigation çağrılacak ve sonuçlar formatlanacak.
	// Şimdilik boş sonuç gösteriyoruz.

	formatter := output.NewFormatter(format)
	return formatter.Render(os.Stdout, nil)
}
