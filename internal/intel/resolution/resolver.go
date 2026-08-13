package resolution

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// EntityType, varlık tipini temsil eder.
type EntityType string

const (
	EntityEmail    EntityType = "email"
	EntityDomain   EntityType = "domain"
	EntityIP       EntityType = "ip"
	EntityPerson   EntityType = "person"
	EntityUsername EntityType = "username"
	EntityHash     EntityType = "hash"
)

// Entity, çözümlenmiş bir varlığı temsil eder.
// Birden fazla bulgu aynı gerçek varlığa işaret ediyorsa tek Entity altında birleştirilir.
type Entity struct {
	ID           string     `json:"id"`
	Type         EntityType `json:"type"`
	PrimaryValue string     `json:"primary_value"` // Birincil tanımlayıcı
	Aliases      []string   `json:"aliases"`       // Alternatif değerler
	FindingIDs   []string   `json:"finding_ids"`   // İlişkili bulgu ID'leri
	Sources      []string   `json:"sources"`       // Bu varlığı bulan connector'lar
	Confidence   int        `json:"confidence"`    // Toplam güven puanı

	// Attributes, bulguların Context alanından çıkarılan gerçek istihbarat
	// verisidir: ad, konum, bio, takipçi sayısı, hesap açılış tarihi vb.
	//
	// Bu alan olmadan sistem yalnızca URL listeliyordu. Veri connector'lar
	// tarafından zaten toplanıyordu ama Finding.Context içinde bir JSON
	// string olarak gömülü kalıyor ve hiçbir katmana taşınmıyordu.
	Attributes map[string]any `json:"attributes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MergeEvent, iki entity'nin birleştirilme geçmişini saklar.
type MergeEvent struct {
	SourceID  string    `json:"source_id"`
	TargetID  string    `json:"target_id"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

// FindingInput, resolver'a verilen ham bulgu verisidir.
type FindingInput struct {
	ID      string
	Type    string // "email", "ip", "domain", vb.
	Value   string
	Source  string // Bulguyu üreten connector
	Context string // Connector'ın ürettiği JSON metadata (ad, konum, bio, ...)
}

// contextNoiseKeys, Entity.Attributes'a taşınmayacak anahtarlardır.
// "source" zaten Entity.Sources'ta, "platform" ise değerin kendisinden belli.
var contextNoiseKeys = map[string]bool{
	"source": true,
}

// mergeContext, bir bulgunun Context JSON'ını entity'nin Attributes haritasına
// katar. Aynı entity'ye birden çok bulgu bağlanabildiği için birleştirme
// yapılır; ilk dolu değer korunur, boş değerler hiç yazılmaz.
func mergeContext(entity *Entity, rawContext string) {
	if strings.TrimSpace(rawContext) == "" {
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(rawContext), &parsed); err != nil {
		// Bozuk JSON sessizce yok sayılır — bazı connector'lar Context'i
		// hâlâ fmt.Sprintf ile elle kuruyor ve kaçış hataları olabiliyor.
		return
	}

	for k, v := range parsed {
		if contextNoiseKeys[k] || isEmptyValue(v) {
			continue
		}
		if entity.Attributes == nil {
			entity.Attributes = make(map[string]any)
		}
		if _, exists := entity.Attributes[k]; !exists {
			entity.Attributes[k] = v
		}
	}
}

// isEmptyValue, gösterilmeye değmeyecek değerleri eler ("bio: " gibi
// boş satırlar raporu kirletiyordu).
func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// Resolver, bulguları entity'lere çözümleyen motordir.
type Resolver struct {
	entities     map[string]*Entity // ID → Entity
	valueIndex   map[string]string  // normalizedValue → entityID (hızlı lookup)
	mergeHistory []MergeEvent
	mu           sync.RWMutex
}

// NewResolver, yeni bir varlık çözümleyici oluşturur.
func NewResolver() *Resolver {
	return &Resolver{
		entities:   make(map[string]*Entity),
		valueIndex: make(map[string]string),
	}
}

// Resolve, bulgu listesini entity'lere çözümler.
// Belirleyici eşleşme: tam aynı değer → otomatik merge.
func (r *Resolver) Resolve(findings []FindingInput) []*Entity {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, f := range findings {
		r.resolveOne(f)
	}

	result := make([]*Entity, 0, len(r.entities))
	for _, e := range r.entities {
		result = append(result, e)
	}
	return result
}

// resolveOne, tek bir bulguyu mevcut entity'lerle eşleştirir veya yeni entity oluşturur.
func (r *Resolver) resolveOne(f FindingInput) {
	normalized := normalizeValue(f.Value, f.Type)

	// Belirleyici eşleşme: aynı değer zaten var mı?
	if existingID, exists := r.valueIndex[normalized]; exists {
		entity := r.entities[existingID]
		// Mevcut entity'ye bulguyu ekle
		entity.FindingIDs = appendUnique(entity.FindingIDs, f.ID)
		entity.Sources = appendUnique(entity.Sources, f.Source)
		mergeContext(entity, f.Context)
		entity.UpdatedAt = time.Now().UTC()
		return
	}

	// Yeni entity oluştur
	id := fmt.Sprintf("entity-%s-%d", f.Type, len(r.entities)+1)
	entity := &Entity{
		ID:           id,
		Type:         EntityType(f.Type),
		PrimaryValue: f.Value,
		FindingIDs:   []string{f.ID},
		Sources:      []string{f.Source},
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	mergeContext(entity, f.Context)

	r.entities[id] = entity
	r.valueIndex[normalized] = id
}

// MergeEntities, iki entity'yi birleştirir. source → target'a merge edilir.
// source entity silinir, tüm verileri target'a aktarılır.
func (r *Resolver) MergeEntities(sourceID, targetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	source, ok := r.entities[sourceID]
	if !ok {
		return fmt.Errorf("resolution: source entity %q not found", sourceID)
	}
	target, ok := r.entities[targetID]
	if !ok {
		return fmt.Errorf("resolution: target entity %q not found", targetID)
	}

	// Target'a verileri aktar
	target.Aliases = appendUnique(target.Aliases, source.PrimaryValue)
	for _, alias := range source.Aliases {
		target.Aliases = appendUnique(target.Aliases, alias)
	}
	for _, fid := range source.FindingIDs {
		target.FindingIDs = appendUnique(target.FindingIDs, fid)
	}
	for _, src := range source.Sources {
		target.Sources = appendUnique(target.Sources, src)
	}
	target.UpdatedAt = time.Now().UTC()

	// Value index'i güncelle
	sourceNorm := normalizeValue(source.PrimaryValue, string(source.Type))
	r.valueIndex[sourceNorm] = targetID
	for _, alias := range source.Aliases {
		aliasNorm := normalizeValue(alias, string(source.Type))
		r.valueIndex[aliasNorm] = targetID
	}

	// Merge geçmişini kaydet
	r.mergeHistory = append(r.mergeHistory, MergeEvent{
		SourceID:  sourceID,
		TargetID:  targetID,
		Reason:    "manual_merge",
		Timestamp: time.Now().UTC(),
	})

	// Source'u sil
	delete(r.entities, sourceID)

	return nil
}

// GetEntity, ID ile entity döndürür.
func (r *Resolver) GetEntity(id string) (*Entity, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entities[id]
	return e, ok
}

// GetByValue, değer ile entity döndürür.
func (r *Resolver) GetByValue(value, typ string) (*Entity, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	normalized := normalizeValue(value, typ)
	if id, ok := r.valueIndex[normalized]; ok {
		return r.entities[id], true
	}
	return nil, false
}

// AllEntities, tüm entity'leri döndürür.
func (r *Resolver) AllEntities() []*Entity {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Entity, 0, len(r.entities))
	for _, e := range r.entities {
		result = append(result, e)
	}
	return result
}

// MergeHistory, birleştirme geçmişini döndürür.
func (r *Resolver) MergeHistory() []MergeEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make([]MergeEvent, len(r.mergeHistory))
	copy(cp, r.mergeHistory)
	return cp
}

// --- Yardımcılar ---

// normalizeValue, değeri karşılaştırma için normalize eder.
func normalizeValue(value, typ string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	return fmt.Sprintf("%s:%s", typ, v)
}

// appendUnique, slice'a tekrarsız eleman ekler.
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// GetAll, çözümlenmiş tüm varlıkları döner.
func (r *Resolver) GetAll() []*Entity {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Entity, 0, len(r.entities))
	for _, e := range r.entities {
		list = append(list, e)
	}
	return list
}
