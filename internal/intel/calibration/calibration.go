// Package calibration, güven puanlarının GERÇEKTEN ne kadar doğru olduğunu ölçer.
//
// Evidence Engine'in ağırlıkları mühendislik tahminidir. "%70 güven" ifadesi,
// ölçülmüş bir doğrulama setine dayanmıyorsa araştırmacıyı yanıltır — ve bu,
// sistemin verebileceği en büyük zarardır.
//
// Bu paket sorulması gereken soruyu sorar: sistem "%70" dediğinde gerçekten
// 10 vakanın 7'sinde haklı mı?
package calibration

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/iamfurkann/osint-engine/internal/intel/evidence"
)

// Item, tek bir tahmin ve onun bilinen doğruluğudur.
type Item struct {
	// Ref, insan tarafından tanınabilir tanımlayıcı (URL, değer).
	Ref string

	// Predicted, sistemin verdiği güven (0.0 - 1.0).
	Predicted float64

	// LogOdds, sıcaklık ölçekleme için ham birikim.
	LogOdds float64

	// Label, YER GERÇEĞİ: bu bulgu doğru mu?
	Label bool

	// Groups, puana katkı veren bağımsızlık sınıfları.
	Groups []evidence.CueGroup
}

// Bin, güven aralığı başına ölçüm sonucudur (güvenilirlik diyagramı satırı).
type Bin struct {
	Lower, Upper  float64
	Count         int
	AvgConfidence float64 // sistemin bu kovada ortalama iddiası
	Accuracy      float64 // gerçekte ne kadar haklı çıktı
	Gap           float64 // Accuracy - AvgConfidence (negatif = aşırı güven)
}

// GroupReliability, bir bağımsızlık sınıfının ÖLÇÜLMÜŞ güvenilirliğidir.
type GroupReliability struct {
	Group    evidence.CueGroup
	Label    string
	Count    int
	Correct  int
	Accuracy float64
}

// Report, kalibrasyon ölçümünün tam sonucudur.
type Report struct {
	Samples  int
	Positive int // yer gerçeğinde doğru olan sayısı

	// ECE, Expected Calibration Error: kovaların ağırlıklı ortalama sapması.
	// 0'a yakın = iyi kalibre. 0.10 üzeri = ciddi sorun.
	ECE float64

	// MCE, en kötü kovadaki sapma.
	MCE float64

	// Brier, ortalama kare hata. Hem kalibrasyonu hem ayırt ediciliği ölçer.
	Brier float64

	// Overconfidence, sistemin ortalama iddiası ile gerçek doğruluğu
	// arasındaki fark. Pozitif = sistem kendine olduğundan fazla güveniyor.
	Overconfidence float64

	Bins   []Bin
	Groups []GroupReliability

	// SuggestedTemperature ve SuggestedBias, düzeltilmiş olasılık için:
	//
	//	p' = sigmoid(logOdds / T + b)
	//
	// T tek başına YETMEZ. Sıcaklık ölçekleme yalnızca keskinliği değiştirir;
	// logOdds pozitifken sigmoid(logOdds/T) T ne olursa olsun 0.5'in altına
	// inemez. Oysa elle ayarlanmış ağırlıklardan beklenen asıl hata tam da
	// sistematik yanlılıktır. Kayma terimi (b) bunu düzeltir.
	SuggestedTemperature float64
	SuggestedBias        float64
}

const defaultBins = 10

// Evaluate, etiketli tahminlerden kalibrasyon raporu üretir.
func Evaluate(items []Item) Report {
	r := Report{Samples: len(items)}
	if len(items) == 0 {
		return r
	}

	sumPredicted := 0.0
	sumBrier := 0.0
	for _, it := range items {
		if it.Label {
			r.Positive++
		}
		sumPredicted += it.Predicted
		y := 0.0
		if it.Label {
			y = 1.0
		}
		sumBrier += (it.Predicted - y) * (it.Predicted - y)
	}

	r.Brier = sumBrier / float64(len(items))
	actualAccuracy := float64(r.Positive) / float64(len(items))
	r.Overconfidence = sumPredicted/float64(len(items)) - actualAccuracy

	r.Bins = buildBins(items, defaultBins)

	// ECE: kova büyüklüğüyle ağırlıklandırılmış |doğruluk - güven|
	for _, b := range r.Bins {
		if b.Count == 0 {
			continue
		}
		gap := math.Abs(b.Accuracy - b.AvgConfidence)
		r.ECE += float64(b.Count) / float64(len(items)) * gap
		if gap > r.MCE {
			r.MCE = gap
		}
	}

	r.Groups = groupReliability(items)
	r.SuggestedTemperature, r.SuggestedBias = fitPlattScaling(items)

	return r
}

func buildBins(items []Item, n int) []Bin {
	bins := make([]Bin, n)
	sums := make([]float64, n)
	hits := make([]int, n)

	for i := range bins {
		bins[i].Lower = float64(i) / float64(n)
		bins[i].Upper = float64(i+1) / float64(n)
	}

	for _, it := range items {
		idx := int(it.Predicted * float64(n))
		if idx >= n {
			idx = n - 1 // Predicted == 1.0
		}
		if idx < 0 {
			idx = 0
		}
		bins[idx].Count++
		sums[idx] += it.Predicted
		if it.Label {
			hits[idx]++
		}
	}

	for i := range bins {
		if bins[i].Count == 0 {
			continue
		}
		bins[i].AvgConfidence = sums[i] / float64(bins[i].Count)
		bins[i].Accuracy = float64(hits[i]) / float64(bins[i].Count)
		bins[i].Gap = bins[i].Accuracy - bins[i].AvgConfidence
	}

	return bins
}

// groupReliability, her bağımsızlık sınıfının ölçülmüş isabet oranını çıkarır.
//
// Bu, Evidence Engine'deki groupWeights değerlerinin GERÇEK karşılığıdır.
// Ölçüm, tahmin edilen ağırlıklarla çeliştiğinde ağırlıklar düzeltilmelidir.
func groupReliability(items []Item) []GroupReliability {
	type acc struct{ total, correct int }
	stats := make(map[evidence.CueGroup]*acc)

	for _, it := range items {
		for _, g := range it.Groups {
			if stats[g] == nil {
				stats[g] = &acc{}
			}
			stats[g].total++
			if it.Label {
				stats[g].correct++
			}
		}
	}

	out := make([]GroupReliability, 0, len(stats))
	for g, a := range stats {
		gr := GroupReliability{
			Group:   g,
			Label:   g.Label(),
			Count:   a.total,
			Correct: a.correct,
		}
		if a.total > 0 {
			gr.Accuracy = float64(a.correct) / float64(a.total)
		}
		out = append(out, gr)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Accuracy != out[j].Accuracy {
			return out[i].Accuracy > out[j].Accuracy
		}
		return out[i].Count > out[j].Count
	})
	return out
}

// fitPlattScaling, düzeltme parametrelerini (T, b) arar:
//
//	p' = sigmoid(logOdds / T + b)
//
// Platt ölçekleme, aşırı güvenli modelleri düzeltmenin en basit ve en etkili
// yöntemidir. İki parametre olduğu için küçük doğrulama setlerinde bile
// aşırı öğrenme riski düşüktür.
//
// Neden yalnızca sıcaklık değil: T keskinliği ayarlar ama işareti değiştiremez.
// logOdds = +3.0 iken sigmoid(3/T) her zaman > 0.5'tir; gerçek doğruluk %30 ise
// sıcaklık bunu asla düzeltemez. Kayma terimi gerekir.
//
// Izgara araması: parametre uzayı küçük, kapalı çözüme gerek yok.
func fitPlattScaling(items []Item) (temperature, bias float64) {
	bestT, bestB, bestNLL := 1.0, 0.0, math.Inf(1)

	eval := func(tMin, tMax, tStep, bMin, bMax, bStep float64) {
		for t := tMin; t <= tMax; t += tStep {
			if t <= 0 {
				continue
			}
			for b := bMin; b <= bMax; b += bStep {
				nll := 0.0
				for _, it := range items {
					p := sigmoid(it.LogOdds/t + b)
					p = math.Max(math.Min(p, 1-1e-9), 1e-9)
					if it.Label {
						nll -= math.Log(p)
					} else {
						nll -= math.Log(1 - p)
					}
				}
				if nll < bestNLL {
					bestNLL, bestT, bestB = nll, t, b
				}
			}
		}
	}

	// Kaba tarama, ardından en iyi nokta çevresinde ince tarama.
	eval(0.25, 5.0, 0.25, -5.0, 5.0, 0.25)
	eval(math.Max(0.05, bestT-0.25), bestT+0.25, 0.05, bestB-0.25, bestB+0.25, 0.05)

	return math.Round(bestT*100) / 100, math.Round(bestB*100) / 100
}

func sigmoid(x float64) float64 { return 1 / (1 + math.Exp(-x)) }

// Verdict, ECE değerinin insan-okunur yorumudur.
func (r Report) Verdict() string {
	switch {
	case r.Samples < 30:
		return "yetersiz örnek — sonuçlar güvenilir değil"
	case r.ECE < 0.05:
		return "iyi kalibre"
	case r.ECE < 0.10:
		return "kabul edilebilir"
	case r.ECE < 0.20:
		return "zayıf kalibrasyon"
	default:
		return "kötü kalibrasyon — puanlar yanıltıcı"
	}
}

// String, raporu terminalde okunacak biçime getirir.
func (r Report) String() string {
	if r.Samples == 0 {
		return "kalibrasyon: örnek yok"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Örnek: %d  ·  doğru: %d (%.0f%%)\n",
		r.Samples, r.Positive, float64(r.Positive)/float64(r.Samples)*100)
	fmt.Fprintf(&b, "ECE: %.3f (%s)  ·  MCE: %.3f  ·  Brier: %.3f\n",
		r.ECE, r.Verdict(), r.MCE, r.Brier)

	if r.Overconfidence > 0.02 {
		fmt.Fprintf(&b, "⚠ Aşırı güven: sistem ortalama %.0f puan fazla iddia ediyor\n",
			r.Overconfidence*100)
	} else if r.Overconfidence < -0.02 {
		fmt.Fprintf(&b, "ℹ Temkinli: sistem ortalama %.0f puan az iddia ediyor\n",
			-r.Overconfidence*100)
	}
	fmt.Fprintf(&b, "Önerilen düzeltme: p' = sigmoid(logOdds / %.2f %+.2f)",
		r.SuggestedTemperature, r.SuggestedBias)
	if r.SuggestedTemperature > 1.05 {
		b.WriteString("  (keskinlik azaltılmalı)")
	}
	if r.SuggestedBias < -0.2 {
		b.WriteString("  (puanlar aşağı kaydırılmalı)")
	} else if r.SuggestedBias > 0.2 {
		b.WriteString("  (puanlar yukarı kaydırılmalı)")
	}
	b.WriteString("\n\nGÜVENİLİRLİK DİYAGRAMI\n")
	b.WriteString("  aralık      n   iddia   gerçek   sapma\n")
	for _, bin := range r.Bins {
		if bin.Count == 0 {
			continue
		}
		flag := ""
		if bin.Gap < -0.15 {
			flag = "  ← aşırı güven"
		} else if bin.Gap > 0.15 {
			flag = "  ← fazla temkinli"
		}
		fmt.Fprintf(&b, "  %3.0f-%3.0f%%  %3d   %5.0f%%   %5.0f%%   %+5.0f%%%s\n",
			bin.Lower*100, bin.Upper*100, bin.Count,
			bin.AvgConfidence*100, bin.Accuracy*100, bin.Gap*100, flag)
	}

	if len(r.Groups) > 0 {
		b.WriteString("\nKAYNAK GRUBU GÜVENİLİRLİĞİ (ölçülmüş)\n")
		for _, g := range r.Groups {
			fmt.Fprintf(&b, "  %-24s %3d/%-3d  %.0f%%\n",
				g.Label, g.Correct, g.Count, g.Accuracy*100)
		}
	}

	return b.String()
}
