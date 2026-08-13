package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Sürekli izleme (watchlist) yönetimi",
	}

	cmd.AddCommand(newWatchAddCmd())
	cmd.AddCommand(newWatchListCmd())
	cmd.AddCommand(newWatchRemoveCmd())

	return cmd
}

var watchType string
var watchInterval string

func newWatchAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <target>",
		Short: "Yeni bir hedefi izleme listesine ekler",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor")
			}

			target := args[0]
			interval, err := time.ParseDuration(watchInterval)
			if err != nil {
				return fmt.Errorf("geçersiz süre formatı: %s (örnek: 6h, 30m)", watchInterval)
			}
			if interval < 5*time.Minute {
				return fmt.Errorf("minimum izleme aralığı 5 dakikadır")
			}

			params := map[string]interface{}{
				"target":   target,
				"type":     watchType,
				"interval": int64(interval),
			}

			_, err = client.Call("watch.add", params)
			if err != nil {
				return err
			}

			color.Green("✅ '%s' izleme listesine eklendi (Aralık: %s)", target, watchInterval)
			return nil
		},
	}

	cmd.Flags().StringVar(&watchType, "type", "auto", "Hedef tipi (domain, email, ip, vb.)")
	cmd.Flags().StringVar(&watchInterval, "interval", "24h", "Kontrol aralığı (örnek: 12h, 30m)")

	return cmd
}

func newWatchListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "İzleme listesini gösterir",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			if !client.IsRunning() {
				return fmt.Errorf("daemon çalışmıyor")
			}

			res, err := client.Call("watch.list", nil)
			if err != nil {
				return err
			}

			var list []map[string]interface{}
			if err := json.Unmarshal(res, &list); err != nil {
				return fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
			}

			if len(list) == 0 {
				fmt.Println("İzleme listesi boş.")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			color.New(color.FgHiBlack).Fprintf(tw, "HEDEF\tTİP\tARALIK\tSON KONTROL\n")
			color.New(color.FgHiBlack).Fprintf(tw, "─────\t───\t──────\t───────────\n")

			for _, item := range list {
				target := item["target"].(string)
				typ := item["type"].(string)
				interval := time.Duration(item["interval"].(float64))
				lastRunStr := item["last_run"].(string)

				lastRun := "Hiç çalışmadı"
				if t, err := time.Parse(time.RFC3339, lastRunStr); err == nil && !t.IsZero() {
					lastRun = t.Local().Format("15:04 02-01")
				}

				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", target, typ, interval.String(), lastRun)
			}
			tw.Flush()
			return nil
		},
	}
}

func newWatchRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <target> <type>",
		Short: "Bir hedefi izleme listesinden çıkarır",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := getIPCClient()
			target := args[0]
			typ := args[1]

			id := fmt.Sprintf("%s:%s", typ, target)
			_, err := client.Call("watch.remove", map[string]string{"id": id})
			if err != nil {
				return err
			}

			color.Green("✅ '%s' izleme listesinden çıkarıldı.", target)
			return nil
		},
	}
}
