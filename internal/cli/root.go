package cli

import (
	"github.com/spf13/cobra"
)

// Global flag'ler
var (
	outputFormat string
	verbose      bool
	quiet        bool
)

// NewRootCmd, CLI'nin kök komutunu oluşturur.
func NewRootCmd(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "osint",
		Short: "OSINT Engine — Profesyonel Açık Kaynak İstihbarat Aracı",
		Long: `OSINT Engine, hedef hakkında açık kaynaklardan istihbarat toplayan,
analiz eden ve raporlayan profesyonel bir OSINT aracıdır.

Kullanım örnekleri:
  osint search domain example.com
  osint search email user@example.com --output json
  osint search ip 8.8.8.8 --verbose
  osint plugin list
  osint version`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flag'ler
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Çıktı formatı (table, json, csv)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Detaylı çıktı göster")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Sadece sonuçları göster")

	// Alt komutları ekle
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newPluginCmd())
	rootCmd.AddCommand(newKeysCmd())
	rootCmd.AddCommand(newGraphCmd())
	rootCmd.AddCommand(newInvestigateCmd())
	rootCmd.AddCommand(newWatchCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newViewCmd())
	rootCmd.AddCommand(newCalibrateCmd())
	rootCmd.AddCommand(newVersionCmd(version))

	return rootCmd
}
