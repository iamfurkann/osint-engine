package connectors

import "encoding/json"

// mustJSONContext, plugin.Result.Context alanı için güvenli JSON üretir.
//
// Connector'ların çoğu bu alanı fmt.Sprintf ile elle kuruyor:
//
//	fmt.Sprintf(`{"org":"%s"}`, org)
//
// Bu, org içinde tırnak, ters bölü, yeni satır veya kontrol karakteri
// geçtiğinde BOZUK JSON üretir — ve bu alan kazınmış web içeriğinden,
// profil bio'larından ve banner'lardan besleniyor, yani saldırgan
// kontrolündeki veri. Bozuk Context daha sonra rapor/graf katmanında
// sessizce ayrıştırma hatasına dönüşür.
//
// json.Marshal kaçışları doğru yapar. Hata yalnızca serileştirilemeyen
// tipler (kanal, fonksiyon) için olabilir ki burada kullanılan map[string]any
// içerikleri her zaman serileştirilebilir; yine de sessizce bozuk JSON
// döndürmemek için güvenli bir varsayılana düşülür.
func mustJSONContext(v map[string]any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"context serialization failed"}`
	}
	return string(data)
}
