package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/iamfurkann/osint-engine/internal/intel/calibration"
	"github.com/iamfurkann/osint-engine/internal/intel/evidence"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// dataset, etiketli doğrulama setinin dosya biçimidir.
//
// Kalibrasyonun tek zor kısmı YER GERÇEĞİDİR: sistemin "%70" dediğinde
// gerçekten haklı olup olmadığını ancak doğruyu bilen bir insan söyleyebilir.
// Bu yüzden set elle etiketlenir; `calibrate init` iskeleti üretip işi
// URL yazmaktan işaretlemeye indirger.
type dataset struct {
	Case []datasetCase `toml:"case"`
}

type datasetCase struct {
	InvestigationID string   `toml:"investigation_id"`
	Note            string   `toml:"note,omitempty"`
	Confirmed       []string `toml:"confirmed"`           // gerçekten hedefe ait
	Rejected        []string `toml:"rejected"`            // hedefe ait DEĞİL
	Unlabeled       []string `toml:"unlabeled,omitempty"` // henüz karar verilmedi
}

func newCalibrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "calibrate",
		Short: "Güven puanlarının gerçekten ne kadar doğru olduğunu ölçer",
		Long: `Evidence Engine'in ağırlıkları mühendislik tahminidir; "%70 güven"
ifadesi ölçülmüş bir doğrulama setine dayanmadıkça yanıltıcıdır.

Bu komut sorulması gereken soruyu sorar: sistem "%70" dediğinde gerçekten
10 vakanın 7'sinde haklı mı?

Kullanım:
  1. osint calibrate init <inv-id> --out cases.toml    (iskelet üret)
  2. cases.toml'u elle etiketle: unlabeled → confirmed / rejected
  3. osint calibrate run cases.toml                     (ölç)`,
	}

	cmd.AddCommand(newCalibrateInitCmd())
	cmd.AddCommand(newCalibrateRunCmd())
	return cmd
}

// --- calibrate init ---

func newCalibrateInitCmd() *cobra.Command {
	var out string
	var appendMode bool

	cmd := &cobra.Command{
		Use:   "init <inv-id>",
		Short: "Bir araştırmadan etiketlenecek iskelet dosya üretir",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			entities, err := fetchEntities(args[0])
			if err != nil {
				return err
			}
			if len(entities) == 0 {
				return fmt.Errorf("bu araştırmada varlık yok: %s", args[0])
			}

			values := make([]string, 0, len(entities))
			for _, e := range entities {
				values = append(values, e.PrimaryValue)
			}
			sort.Strings(values)

			ds := dataset{}
			if appendMode {
				if existing, err := loadDataset(out); err == nil {
					ds = existing
				}
			}
			ds.Case = append(ds.Case, datasetCase{
				InvestigationID: args[0],
				Note:            "unlabeled listesindeki her değeri confirmed veya rejected'a taşıyın",
				Confirmed:       []string{},
				Rejected:        []string{},
				Unlabeled:       values,
			})

			data, err := toml.Marshal(ds)
			if err != nil {
				return fmt.Errorf("iskelet oluşturulamadı: %w", err)
			}
			if err := os.WriteFile(out, data, 0600); err != nil {
				return fmt.Errorf("dosya yazılamadı: %w", err)
			}

			color.Green("✅ %s oluşturuldu (%d etiketlenecek değer)", out, len(values))
			fmt.Println("   Şimdi dosyayı açıp 'unlabeled' altındakileri")
			fmt.Println("   'confirmed' veya 'rejected' listelerine taşıyın.")
			return nil
		},
	}

	// Not: "-o" kısayolu global --output bayrağına ait, burada kullanılamaz.
	cmd.Flags().StringVar(&out, "out", "calibration.toml", "Çıktı dosyası")
	cmd.Flags().BoolVarP(&appendMode, "append", "a", false, "Mevcut dosyaya yeni vaka ekle")
	return cmd
}

// --- calibrate run ---

func newCalibrateRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <dataset.toml>",
		Short: "Etiketli set üzerinde kalibrasyonu ölçer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ds, err := loadDataset(args[0])
			if err != nil {
				return err
			}

			var items []calibration.Item
			unlabeled := 0
			missing := 0

			for _, c := range ds.Case {
				entities, err := fetchEntities(c.InvestigationID)
				if err != nil {
					return fmt.Errorf("%s: %w", c.InvestigationID, err)
				}

				byValue := make(map[string]scoredEntity, len(entities))
				for _, e := range entities {
					byValue[e.PrimaryValue] = e
				}

				unlabeled += len(c.Unlabeled)

				for label, values := range map[bool][]string{
					true:  c.Confirmed,
					false: c.Rejected,
				} {
					for _, v := range values {
						e, ok := byValue[v]
						if !ok {
							missing++
							continue
						}
						items = append(items, calibration.Item{
							Ref:       v,
							Predicted: float64(e.Confidence) / 100,
							LogOdds:   e.logOdds(),
							Label:     label,
							Groups:    e.groups(),
						})
					}
				}
			}

			if missing > 0 {
				color.Yellow("⚠ %d etiketli değer araştırmada bulunamadı (atlandı)", missing)
			}
			if unlabeled > 0 {
				color.Yellow("⚠ %d değer hâlâ etiketsiz — ölçüme dahil edilmedi", unlabeled)
			}
			if len(items) == 0 {
				return fmt.Errorf("etiketli örnek yok — önce 'calibrate init' ile iskelet üretip etiketleyin")
			}

			report := calibration.Evaluate(items)

			color.Cyan("\n📏 KALİBRASYON RAPORU\n")
			fmt.Println(report.String())

			if report.Samples < 30 {
				color.Yellow("\nNot: güvenilir bir ölçüm için en az 30, tercihen 100+ etiketli örnek gerekir.")
			}
			return nil
		},
	}
}

// --- yardımcılar ---

// scoredEntity, IPC'den dönen varlığın kalibrasyon için gereken alanlarıdır.
type scoredEntity struct {
	PrimaryValue string         `json:"primary_value"`
	Confidence   int            `json:"confidence"`
	Attributes   map[string]any `json:"attributes"`
}

func (e scoredEntity) logOdds() float64 {
	if v, ok := e.Attributes["confidence_logodds"].(float64); ok {
		return v
	}
	return 0
}

func (e scoredEntity) groups() []evidence.CueGroup {
	raw, ok := e.Attributes["confidence_groups"].([]any)
	if !ok {
		return nil
	}
	out := make([]evidence.CueGroup, 0, len(raw))
	for _, g := range raw {
		if s, isStr := g.(string); isStr {
			out = append(out, evidence.CueGroup(s))
		}
	}
	return out
}

// fetchEntities, daemon'dan bir araştırmanın puanlanmış varlıklarını çeker.
func fetchEntities(invID string) ([]scoredEntity, error) {
	client := getIPCClient()
	if !client.IsRunning() {
		return nil, fmt.Errorf("daemon çalışmıyor. Önce 'osintd start' ile başlatın")
	}

	res, err := client.Call("investigation.graph", map[string]string{"id": invID})
	if err != nil {
		return nil, err
	}

	var payload struct {
		Entities []scoredEntity `json:"entities"`
	}
	if err := json.Unmarshal(res, &payload); err != nil {
		return nil, fmt.Errorf("daemon yanıtı çözümlenemedi: %w", err)
	}
	return payload.Entities, nil
}

func loadDataset(path string) (dataset, error) {
	var ds dataset
	data, err := os.ReadFile(path)
	if err != nil {
		return ds, fmt.Errorf("doğrulama seti okunamadı: %w", err)
	}
	if err := toml.Unmarshal(data, &ds); err != nil {
		return ds, fmt.Errorf("doğrulama seti ayrıştırılamadı: %w", err)
	}
	if len(ds.Case) == 0 {
		return ds, fmt.Errorf("doğrulama setinde hiç vaka yok")
	}
	for i := range ds.Case {
		if strings.TrimSpace(ds.Case[i].InvestigationID) == "" {
			return ds, fmt.Errorf("vaka #%d: investigation_id boş", i+1)
		}
	}
	return ds, nil
}
