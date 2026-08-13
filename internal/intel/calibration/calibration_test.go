package calibration

import (
	"math"
	"strings"
	"testing"

	"github.com/iamfurkann/osint-engine/internal/intel/evidence"
)

// mkItems, verilen (güven, doğruluk) çiftlerinden örnek üretir.
func mkItems(pairs []struct {
	conf float64
	ok   bool
}) []Item {
	out := make([]Item, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, Item{
			Predicted: p.conf,
			LogOdds:   math.Log(p.conf/(1-p.conf) + 1e-12),
			Label:     p.ok,
		})
	}
	return out
}

// repeat, aynı (güven, doğruluk) çiftinden n adet üretir.
func repeat(conf float64, ok bool, n int) []struct {
	conf float64
	ok   bool
} {
	out := make([]struct {
		conf float64
		ok   bool
	}, n)
	for i := range out {
		out[i] = struct {
			conf float64
			ok   bool
		}{conf, ok}
	}
	return out
}

// Mükemmel kalibre bir sistem: %80 dediğinde 10 vakanın 8'inde haklı.
func TestEvaluate_PerfectCalibration(t *testing.T) {
	var pairs []struct {
		conf float64
		ok   bool
	}
	pairs = append(pairs, repeat(0.8, true, 80)...)
	pairs = append(pairs, repeat(0.8, false, 20)...)

	r := Evaluate(mkItems(pairs))

	if r.ECE > 0.02 {
		t.Errorf("mükemmel kalibrasyonda ECE ~0 olmalı: %.3f", r.ECE)
	}
	if math.Abs(r.Overconfidence) > 0.02 {
		t.Errorf("aşırı/eksik güven olmamalı: %.3f", r.Overconfidence)
	}
	if r.Verdict() != "iyi kalibre" {
		t.Errorf("verdict 'iyi kalibre' olmalı: %q", r.Verdict())
	}
}

// Aşırı güvenli sistem: %90 diyor ama 10 vakanın yalnızca 3'ünde haklı.
// Evidence Engine'in kalibre edilmemiş ağırlıklarında beklenen durum budur.
func TestEvaluate_DetectsOverconfidence(t *testing.T) {
	var pairs []struct {
		conf float64
		ok   bool
	}
	pairs = append(pairs, repeat(0.9, true, 30)...)
	pairs = append(pairs, repeat(0.9, false, 70)...)

	r := Evaluate(mkItems(pairs))

	if r.Overconfidence < 0.4 {
		t.Errorf("ciddi aşırı güven tespit edilmeliydi: %.3f", r.Overconfidence)
	}
	if r.ECE < 0.4 {
		t.Errorf("ECE yüksek olmalı: %.3f", r.ECE)
	}
	if r.Verdict() != "kötü kalibrasyon — puanlar yanıltıcı" {
		t.Errorf("verdict kötü olmalı: %q", r.Verdict())
	}

	// Sıcaklık > 1 olmalı: puanlar yumuşatılmalı.
	if r.SuggestedTemperature <= 1.0 {
		t.Errorf("aşırı güven için T>1 önerilmeliydi: %.2f", r.SuggestedTemperature)
	}

	out := r.String()
	if !strings.Contains(out, "Aşırı güven") {
		t.Errorf("çıktı aşırı güveni belirtmeli:\n%s", out)
	}
}

// Fazla temkinli sistem: %20 diyor ama 10 vakanın 8'inde haklı.
func TestEvaluate_DetectsUnderconfidence(t *testing.T) {
	var pairs []struct {
		conf float64
		ok   bool
	}
	pairs = append(pairs, repeat(0.2, true, 80)...)
	pairs = append(pairs, repeat(0.2, false, 20)...)

	r := Evaluate(mkItems(pairs))

	if r.Overconfidence > -0.4 {
		t.Errorf("eksik güven tespit edilmeliydi: %.3f", r.Overconfidence)
	}
	if !strings.Contains(r.String(), "Temkinli") {
		t.Error("çıktı temkinliliği belirtmeli")
	}
}

// Kaynak grubu güvenilirliği ölçülmeli — Evidence Engine ağırlıklarının
// gerçek karşılığı budur.
func TestEvaluate_MeasuresGroupReliability(t *testing.T) {
	items := []Item{
		// profil içeriği: 3/3 doğru
		{Predicted: 0.7, Label: true, Groups: []evidence.CueGroup{evidence.GroupProfile}},
		{Predicted: 0.7, Label: true, Groups: []evidence.CueGroup{evidence.GroupProfile}},
		{Predicted: 0.7, Label: true, Groups: []evidence.CueGroup{evidence.GroupProfile}},
		// platform varlığı: 1/4 doğru
		{Predicted: 0.4, Label: true, Groups: []evidence.CueGroup{evidence.GroupPresence}},
		{Predicted: 0.4, Label: false, Groups: []evidence.CueGroup{evidence.GroupPresence}},
		{Predicted: 0.4, Label: false, Groups: []evidence.CueGroup{evidence.GroupPresence}},
		{Predicted: 0.4, Label: false, Groups: []evidence.CueGroup{evidence.GroupPresence}},
	}

	r := Evaluate(items)

	byGroup := map[evidence.CueGroup]GroupReliability{}
	for _, g := range r.Groups {
		byGroup[g.Group] = g
	}

	if got := byGroup[evidence.GroupProfile].Accuracy; got != 1.0 {
		t.Errorf("profil içeriği %%100 olmalı: %.2f", got)
	}
	if got := byGroup[evidence.GroupPresence].Accuracy; got != 0.25 {
		t.Errorf("platform varlığı %%25 olmalı: %.2f", got)
	}

	// En güvenilir grup başta listelenmeli.
	if r.Groups[0].Group != evidence.GroupProfile {
		t.Errorf("en güvenilir grup başta olmalı: %v", r.Groups[0].Group)
	}
}

// Az örnekle çıkan sonuç güvenilir sayılmamalı — bu, kalibrasyonun
// kendi kendini kandırmasını engelleyen koruma.
func TestEvaluate_RefusesToTrustSmallSamples(t *testing.T) {
	r := Evaluate(mkItems(repeat(0.8, true, 5)))
	if r.Verdict() != "yetersiz örnek — sonuçlar güvenilir değil" {
		t.Errorf("az örnekte uyarı verilmeli: %q", r.Verdict())
	}
}

func TestEvaluate_Empty(t *testing.T) {
	r := Evaluate(nil)
	if r.Samples != 0 || r.ECE != 0 {
		t.Errorf("boş girdi sıfır rapor dönmeli: %+v", r)
	}
	if !strings.Contains(r.String(), "örnek yok") {
		t.Errorf("boş rapor açıkça belirtilmeli: %q", r.String())
	}
}

func TestBuildBins_HandlesEdgeValues(t *testing.T) {
	items := []Item{
		{Predicted: 0.0, Label: false},
		{Predicted: 1.0, Label: true}, // sınır: son kovaya düşmeli
		{Predicted: 0.5, Label: true},
	}
	bins := buildBins(items, 10)

	total := 0
	for _, b := range bins {
		total += b.Count
	}
	if total != 3 {
		t.Errorf("tüm örnekler kovalara dağıtılmalı: %d", total)
	}
	if bins[9].Count != 1 {
		t.Errorf("Predicted=1.0 son kovaya düşmeli: %d", bins[9].Count)
	}
	if bins[0].Count != 1 {
		t.Errorf("Predicted=0.0 ilk kovaya düşmeli: %d", bins[0].Count)
	}
}

// Sıcaklık ölçekleme gerçekten kalibrasyonu iyileştirmeli.
func TestFitPlattScaling_CorrectsSystematicBias(t *testing.T) {
	// Aşırı güvenli: yüksek log-odds ama düşük doğruluk
	var items []Item
	for i := 0; i < 70; i++ {
		items = append(items, Item{Predicted: 0.95, LogOdds: 3.0, Label: false})
	}
	for i := 0; i < 30; i++ {
		items = append(items, Item{Predicted: 0.95, LogOdds: 3.0, Label: true})
	}

	tmp, bias := fitPlattScaling(items)

	// Düzeltilmiş olasılık gerçek orana (0.30) yaklaşmalı.
	corrected := sigmoid(3.0/tmp + bias)
	if math.Abs(corrected-0.30) > 0.08 {
		t.Errorf("düzeltilmiş olasılık ~0.30 olmalı, alınan %.2f (T=%.2f, b=%.2f)",
			corrected, tmp, bias)
	}
}

func TestReport_StringIncludesDiagram(t *testing.T) {
	var pairs []struct {
		conf float64
		ok   bool
	}
	pairs = append(pairs, repeat(0.85, true, 40)...)
	pairs = append(pairs, repeat(0.35, false, 40)...)

	out := Evaluate(mkItems(pairs)).String()
	for _, want := range []string{"GÜVENİLİRLİK DİYAGRAMI", "ECE", "Önerilen düzeltme"} {
		if !strings.Contains(out, want) {
			t.Errorf("çıktıda %q bekleniyordu:\n%s", want, out)
		}
	}
}
