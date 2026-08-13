package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/engine"
	"github.com/iamfurkann/osint-engine/pkg/plugin"
	"github.com/spf13/cobra"
)

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin yönetimi",
	}

	cmd.AddCommand(newPluginListCmd())
	cmd.AddCommand(newPluginCreateCmd())
	cmd.AddCommand(newPluginInfoCmd())

	return cmd
}

// --- plugin list ---

func newPluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Yüklü plugin'leri listele",
		RunE:  runPluginList,
	}
}

func runPluginList(cmd *cobra.Command, args []string) error {
	// Geçici: global registry olmadığı için basit çıktı
	// TODO: Bootstrap'tan registry alınacak
	fmt.Fprintln(os.Stderr, color.CyanString("📦 Yüklü Plugin'ler"))
	fmt.Fprintln(os.Stderr, color.HiBlackString("   (Plugin sistemi CLI'dan yüklenecek — bkz. osint plugin create)"))
	return nil
}

// --- plugin create ---

var (
	pluginType     string
	pluginLanguage string
)

func newPluginCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Yeni plugin iskeleti oluştur",
		Long: `Yeni bir plugin iskelet dosyaları oluşturur.

Örnekler:
  osint plugin create shodan-scanner --type connector
  osint plugin create nlp-analyzer --type analyzer --lang python`,
		Args: cobra.ExactArgs(1),
		RunE: runPluginCreate,
	}

	cmd.Flags().StringVarP(&pluginType, "type", "t", "connector", "Plugin tipi (connector, analyzer, reporter, ai-provider)")
	cmd.Flags().StringVarP(&pluginLanguage, "lang", "l", "go", "Plugin dili (go, python)")

	return cmd
}

func runPluginCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// PluginsDir'i config'den al
	pluginsDir := os.ExpandEnv("$HOME/.osint/plugins")

	opts := engine.ScaffoldOptions{
		Name:       name,
		Type:       pluginTypeFromString(pluginType),
		Language:   pluginLanguage,
		PluginsDir: pluginsDir,
	}

	createdDir, err := engine.Scaffold(opts)
	if err != nil {
		return err
	}

	color.New(color.FgGreen, color.Bold).Println("✅ Plugin iskeleti oluşturuldu!")
	fmt.Printf("   Dizin: %s\n", createdDir)
	fmt.Println()

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "   Dosya\tAçıklama\n")
	fmt.Fprintf(tw, "   ─────\t────────\n")
	fmt.Fprintf(tw, "   manifest.toml\tPlugin kimlik kartı\n")
	if pluginLanguage == "python" {
		fmt.Fprintf(tw, "   main.py\tPython plugin şablonu\n")
	} else {
		fmt.Fprintf(tw, "   plugin.go\tGo plugin şablonu\n")
	}
	fmt.Fprintf(tw, "   README.md\tDokümantasyon\n")
	tw.Flush()

	return nil
}

func pluginTypeFromString(s string) plugin.PluginType {
	switch s {
	case "connector":
		return plugin.TypeConnector
	case "analyzer":
		return plugin.TypeAnalyzer
	case "reporter":
		return plugin.TypeReporter
	case "ai-provider":
		return plugin.TypeAI
	default:
		return plugin.TypeConnector
	}
}

// --- plugin info ---

func newPluginInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Plugin detaylarını göster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Plugin: %s\n", args[0])
			fmt.Println("(Detaylı bilgi için plugin registry entegrasyonu gereklidir)")
			return nil
		},
	}
}
