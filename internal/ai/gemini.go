package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GeminiClient struct {
	httpClient *http.Client
}

func NewGeminiClient() *GeminiClient {
	return &GeminiClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// AnalyzeFindings, OSINT bulgularını LLM'e gönderir ve Siber Güvenlik Analizi döner.
func (c *GeminiClient) AnalyzeFindings(ctx context.Context, apiKey, target, findingsJSON string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("gemini API key is required")
	}

	prompt := fmt.Sprintf(`Sen kıdemli bir OSINT (Açık Kaynak İstihbaratı) analistisin. 
Aşağıda '%s' hedefine yönelik otomatik araçlarla toplanan ham veriler (JSON) verilmiştir.

Bu verileri analiz ederek aşağıdaki formatta kapsamlı bir KİŞİ PROFİLİ RAPORU oluştur.
Sadece DOĞRULANMIŞ verilere dayanarak yaz. Yoksa "Bilgi bulunamadı" yaz.
Raporu Türkçe olarak, Markdown formatında yaz.

## 📋 Kişi Özet Profili
- Tam Ad:
- Bilinen Kullanıcı Adları:
- Tespit Edilen E-Postalar:
- Konum/Şehir (varsa):
- Meslek/İlgi Alanları (profil bilgilerinden çıkarım):

## 🌐 Sosyal Medya Varlığı
Her tespit edilen platform için:
- Platform adı + profil linki
- Profil açıklaması/bio (varsa)
- Takipçi/takip sayıları (varsa)
- Aktiflik durumu tahmini

## 🔍 İnternet'teki İzler
- Google/Web aramalarında bulunan sayfalar
- Kişinin adının geçtiği siteler ve bağlam
- Forumlar, bloglar, haberler
- GitHub repoları ve teknik aktivite

## 🔗 Bağlantılar ve İlişkiler
- Bağlantılı olabilecek diğer hesaplar
- Aynı e-posta/username ile bağlantılı platformlar
- Ortak patern analizi

## ⚠️ Güvenlik ve Mahremiyet Değerlendirmesi
- Açık bırakılmış kişisel bilgiler
- Potansiyel riskler (e-posta sızıntısı, konum ifşası vb.)
- Mahremiyet skoru (1-10, 10=çok açık)

## 📊 Güvenilirlik Notu
Hangi bulgular yüksek güvenilirlikte, hangileri düşük — kısa değerlendirme.

Ham Veriler:
%s
`, target, findingsJSON)

	// API anahtarı bilerek query string'de DEĞİL — 'x-goog-api-key' başlığıyla
	// gönderiliyor (aşağıda). URL'ler log'lara ve proxy kayıtlarına sızar.
	const url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-flash-latest:generateContent"

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyErr, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(bodyErr))
	}

	var respData struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respData.Candidates) == 0 || len(respData.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	return respData.Candidates[0].Content.Parts[0].Text, nil
}
