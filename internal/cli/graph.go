package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/intel/graph"
	"github.com/spf13/cobra"
)

// globalGraph, CLI oturumu sırasında kullanılan graf instance'ı.
// Gerçek entegrasyonda orchestrator'dan alınacak.
var globalGraph = graph.NewGraph()

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "İstihbarat grafı sorguları",
	}

	cmd.AddCommand(newGraphNeighborsCmd())
	cmd.AddCommand(newGraphPathCmd())
	cmd.AddCommand(newGraphStatsCmd())

	return cmd
}

// --- graph neighbors ---

var hopDepth int

func newGraphNeighborsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "neighbors <entity-id>",
		Short: "Entity'nin komşularını listele",
		Long: `Belirtilen entity'nin N-hop komşularını gösterir.

Örnekler:
  osint graph neighbors entity-email-1
  osint graph neighbors entity-email-1 --hops 2`,
		Args: cobra.ExactArgs(1),
		RunE: runGraphNeighbors,
	}

	cmd.Flags().IntVar(&hopDepth, "hops", 1, "Komşuluk derinliği (1-5)")

	return cmd
}

func runGraphNeighbors(cmd *cobra.Command, args []string) error {
	entityID := args[0]

	node, ok := globalGraph.GetNode(entityID)
	if !ok {
		return fmt.Errorf("entity bulunamadı: %s", entityID)
	}

	neighbors := globalGraph.Neighbors(entityID, hopDepth)

	color.New(color.FgCyan, color.Bold).Printf("🔗 %s (%s) komşuları (%d hop)\n\n", node.Value, node.Type, hopDepth)

	if len(neighbors) == 0 {
		fmt.Println("  Komşu bulunamadı.")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	color.New(color.FgHiBlack).Fprintf(tw, "  ID\tTİP\tDEĞER\tGÜVEN\n")
	color.New(color.FgHiBlack).Fprintf(tw, "  ──\t───\t─────\t─────\n")

	for _, n := range neighbors {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%d%%\n", n.ID, n.Type, n.Value, n.Confidence)
	}
	return tw.Flush()
}

// --- graph path ---

func newGraphPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <entity-a> <entity-b>",
		Short: "İki entity arasındaki en kısa yolu göster",
		Long: `BFS ile iki entity arasındaki en kısa yolu bulur.

Örnekler:
  osint graph path entity-email-1 entity-ip-3`,
		Args: cobra.ExactArgs(2),
		RunE: runGraphPath,
	}
}

func runGraphPath(cmd *cobra.Command, args []string) error {
	from, to := args[0], args[1]

	path := globalGraph.ShortestPath(from, to)
	if path == nil {
		return fmt.Errorf("yol bulunamadı: %s → %s", from, to)
	}

	color.New(color.FgGreen, color.Bold).Printf("📍 En kısa yol (%d adım)\n\n", len(path)-1)

	for i, node := range path {
		prefix := "  ├─"
		if i == len(path)-1 {
			prefix = "  └─"
		}
		if i == 0 {
			prefix = "  ┌─"
		}
		fmt.Printf("%s [%s] %s (%s) güven: %d%%\n", prefix, node.ID, node.Value, node.Type, node.Confidence)
	}

	return nil
}

// --- graph stats ---

func newGraphStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Graf istatistiklerini göster",
		RunE: func(cmd *cobra.Command, args []string) error {
			color.New(color.FgCyan, color.Bold).Println("📊 Graf İstatistikleri")
			fmt.Printf("  Düğüm sayısı:  %d\n", globalGraph.NodeCount())
			fmt.Printf("  Kenar sayısı:  %d\n", globalGraph.EdgeCount())
			return nil
		},
	}
}
