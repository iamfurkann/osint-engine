package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/daemon"
	"github.com/iamfurkann/osint-engine/internal/ipc"
	"github.com/spf13/cobra"
)

func newInvestigateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "investigate",
		Aliases: []string{"inv"},
		Short:   "Arka plan araştırmalarını yönetir",
	}

	cmd.AddCommand(newInvCreateCmd())
	cmd.AddCommand(newInvListCmd())
	cmd.AddCommand(newInvStatusCmd())
	cmd.AddCommand(newInvPauseCmd())
	cmd.AddCommand(newInvResumeCmd())
	cmd.AddCommand(newInvCancelCmd())
	cmd.AddCommand(newInvExportCmd())

	return cmd
}

// IPC Client Helper
func getIPCClient() *ipc.Client {
	cfg := daemon.DefaultConfig()
	return ipc.NewClient(cfg.SocketPath)
}

// --- investigate create ---

func newInvCreateCmd() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "create <target>",
		Short: "Yeni bir arka plan araştırması başlatır",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor. Önce 'osintd start' ile başlatın")
			}

			target := args[0]
			params := map[string]string{"target": target, "type": "auto"}
			if recursive {
				// Bulunan e-posta/site/kullanıcı adları yeni tarama tetikler.
				params["recursive"] = "true"
			}

			res, err := client.Call("investigation.start", params)
			if err != nil {
				return err
			}

			var resp map[string]string
			if err := json.Unmarshal(res, &resp); err != nil {
				return fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
			}

			color.Green("✅ Araştırma arka planda başlatıldı (ID: %s)", resp["investigation_id"])
			if recursive {
				color.HiBlack("   Özyinelemeli mod açık — bulunan e-posta/site/kullanıcı adları yeni tarama başlatacak.")
			}
			fmt.Printf("Durumunu izlemek için: osint inv status %s\n", resp["investigation_id"])
			return nil
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false,
		"Bulunan varlıklardan yeni taramalar türet (biyografideki e-posta, site, @kullanıcı adı)")

	return cmd
}

// --- investigate list ---

func newInvListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Bellekteki aktif araştırmaları listeler",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor")
			}

			res, err := client.Call("investigation.list", nil)
			if err != nil {
				return err
			}

			// orchestrator.Progress ile birebir eşleşen tiplenmiş yapı.
			//
			// Önceden map[string]interface{} üzerinden "id", "target", "status",
			// "progress" ve "start_time" okunuyordu. Bunların HİÇBİRİ yanıtta
			// yok — investigation.list, []Progress döndürüyor. Sonuç: nil
			// değere yapılan denetimsiz .(string) dönüşümü komutu PANİKLETİYORDU:
			//   panic: interface conversion: interface {} is nil, not string
			var list []struct {
				InvestigationID string  `json:"investigation_id"`
				Total           int     `json:"total"`
				Completed       int     `json:"completed"`
				Failed          int     `json:"failed"`
				Percent         float64 `json:"percent"`
			}
			if err := json.Unmarshal(res, &list); err != nil {
				return fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
			}

			if len(list) == 0 {
				fmt.Println("Bellekte aktif araştırma yok.")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			color.New(color.FgHiBlack).Fprintf(tw, "ID\tİLERLEME\tTAMAMLANAN\tBAŞARISIZ\n")
			color.New(color.FgHiBlack).Fprintf(tw, "──\t────────\t──────────\t─────────\n")

			for _, inv := range list {
				failed := fmt.Sprintf("%d", inv.Failed)
				if inv.Failed > 0 {
					failed = color.YellowString("%d", inv.Failed)
				}
				fmt.Fprintf(tw, "%s\t%.1f%%\t%d/%d\t%s\n",
					inv.InvestigationID, inv.Percent, inv.Completed, inv.Total, failed)
			}
			return tw.Flush()
		},
	}
}

// --- investigate status ---

func newInvStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <inv-id>",
		Short: "Araştırmanın ilerleme durumunu gösterir",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			invID := args[0]
			res, err := client.Call("investigation.status", map[string]string{"id": invID})
			if err != nil {
				return err
			}

			// orchestrator.Progress ile birebir eşleşen tiplenmiş yapı.
			// Önceden map[string]interface{} üzerinden "id"/"target"/"status"
			// anahtarları okunuyordu; bunların hiçbiri yanıtta yok, dolayısıyla
			// komut her alan için "%!s(<nil>)" basıyordu.
			var status struct {
				InvestigationID string  `json:"investigation_id"`
				Total           int     `json:"total"`
				Completed       int     `json:"completed"`
				Failed          int     `json:"failed"`
				Percent         float64 `json:"percent"`
			}
			if err := json.Unmarshal(res, &status); err != nil {
				return fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
			}

			color.Cyan("Araştırma: %s", status.InvestigationID)
			fmt.Printf("İlerleme:  %.1f%% (%d/%d görev)\n",
				status.Percent, status.Completed, status.Total)
			if status.Failed > 0 {
				color.Yellow("Başarısız: %d görev", status.Failed)
			}

			return nil
		},
	}
}

// --- investigate action commands (pause, resume, cancel) ---

func newInvActionCmd(use, short, method, successMsg string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <inv-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			invID := args[0]

			_, err := client.Call(method, map[string]string{"id": invID})
			if err != nil {
				return err
			}

			color.Green("✅ %s", successMsg)
			return nil
		},
	}
}

func newInvPauseCmd() *cobra.Command {
	return newInvActionCmd("pause", "Araştırmayı duraklatır", "investigation.pause", "Araştırma duraklatıldı.")
}

func newInvResumeCmd() *cobra.Command {
	return newInvActionCmd("resume", "Araştırmayı devam ettirir", "investigation.resume", "Araştırma devam ediyor.")
}

func newInvCancelCmd() *cobra.Command {
	return newInvActionCmd("cancel", "Araştırmayı iptal eder", "investigation.cancel", "Araştırma iptal edildi.")
}

// --- investigate export ---

var exportFormat string

func newInvExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <inv-id>",
		Short: "Araştırma sonuçlarını dışa aktar",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			invID := args[0]

			// Graf formatında veri iste
			res, err := client.Call("investigation.graph", map[string]string{"id": invID})
			if err != nil {
				return err
			}

			var exportData map[string]interface{}
			if err := json.Unmarshal(res, &exportData); err != nil {
				return fmt.Errorf("veri ayrıştırma hatası: %w", err)
			}

			switch exportFormat {
			case "json":
				out, _ := json.MarshalIndent(exportData, "", "  ")
				fmt.Println(string(out))
			case "csv":
				// Sadece node listesini basit bir CSV olarak bas (MVP seviyesi)
				fmt.Println("ID,Label,Type,Confidence")
				if nodes, ok := exportData["nodes"].([]interface{}); ok {
					for _, n := range nodes {
						node := n.(map[string]interface{})["data"].(map[string]interface{})
						fmt.Printf("%s,%s,%s,%v\n", node["id"], node["label"], node["type"], node["confidence"])
					}
				}
			case "graphml":
				// Basit GraphML (XML) çıktısı
				fmt.Println(`<?xml version="1.0" encoding="UTF-8"?>`)
				fmt.Println(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">`)
				fmt.Println(`  <graph id="G" edgedefault="directed">`)
				if nodes, ok := exportData["nodes"].([]interface{}); ok {
					for _, n := range nodes {
						node := n.(map[string]interface{})["data"].(map[string]interface{})
						fmt.Printf(`    <node id="%s"/>`+"\n", node["id"])
					}
				}
				if edges, ok := exportData["edges"].([]interface{}); ok {
					for _, e := range edges {
						edge := e.(map[string]interface{})["data"].(map[string]interface{})
						fmt.Printf(`    <edge source="%s" target="%s"/>`+"\n", edge["source"], edge["target"])
					}
				}
				fmt.Println(`  </graph>`)
				fmt.Println(`</graphml>`)
			default:
				return fmt.Errorf("desteklenmeyen format: %s. (json, csv, graphml kullanın)", exportFormat)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&exportFormat, "format", "json", "Çıktı formatı (json, csv, graphml)")
	return cmd
}
