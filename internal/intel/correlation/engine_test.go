package correlation

import (
	"strings"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/intel/resolution"
)

func makeEntity(id string, typ resolution.EntityType, value string, sources ...string) *resolution.Entity {
	return &resolution.Entity{
		ID:           id,
		Type:         typ,
		PrimaryValue: value,
		Sources:      sources,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
}

// TestEngine_SameGroupSharedSourceCreatesNoEdge, K₁₃ patlamasını önleyen
// kuralı doğrular.
//
// Canlı ölçüm: tek bir HTTP isteğinden 13 varlık çıkaran bir araştırma
// 78 "ilişki" raporluyordu. "Aynı connector buldu" bir İLİŞKİ DEĞİLDİR —
// ortak kökendir ve bu bilgi zaten Entity.Sources alanında durur.
func TestEngine_SameGroupSharedSourceCreatesNoEdge(t *testing.T) {
	e := NewEngine()

	// Her ikisi de yalnızca username-check tarafından bulundu (tek grup).
	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityUsername, "https://x.com/a", "username-check"),
		makeEntity("e2", resolution.EntityUsername, "https://t.me/a", "username-check"),
	}

	for _, c := range e.Correlate(entities) {
		if c.Type == TypeDeterministic {
			t.Errorf("tek bağımsızlık sınıfından kenar kurulmamalıydı: %+v", c)
		}
	}
}

// Farklı bağımsızlık sınıfları aynı iki varlığı gördüyse bu GERÇEK bir
// çapraz doğrulamadır ve kenar kurulmalıdır.
func TestEngine_CrossGroupSharedSourceCreatesEdge(t *testing.T) {
	e := NewEngine()

	// hunter = e-posta istihbaratı, dns-whois = DNS → iki farklı grup
	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityDomain, "a.example.com", "hunter", "dns-whois"),
		makeEntity("e2", resolution.EntityDomain, "b.example.com", "hunter", "dns-whois"),
	}

	found := false
	for _, c := range e.Correlate(entities) {
		if c.Type == TypeDeterministic {
			found = true
			if c.Rule != "independent_corroboration" {
				t.Errorf("kural adı 'independent_corroboration' olmalı: %q", c.Rule)
			}
			if c.Confidence < 60 {
				t.Errorf("iki bağımsız grup için güven ≥60 olmalı: %d", c.Confidence)
			}
			if !strings.Contains(c.Evidence, "bağımsız kaynak grubu") {
				t.Errorf("kanıt metni bağımsızlığı belirtmeli: %q", c.Evidence)
			}
		}
	}
	if !found {
		t.Error("iki bağımsız gruptan kenar bekleniyordu")
	}
}

func TestEngine_EmailDomainRule(t *testing.T) {
	e := NewEngine()

	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityEmail, "admin@example.com", "src1"),
		makeEntity("e2", resolution.EntityDomain, "example.com", "src2"),
	}

	correlations := e.Correlate(entities)

	foundRule := false
	for _, c := range correlations {
		if c.Rule == "email_domain" {
			foundRule = true
			if c.Confidence != 90 {
				t.Errorf("expected confidence 90 for email_domain, got %d", c.Confidence)
			}
		}
	}
	if !foundRule {
		t.Error("expected email_domain correlation")
	}
}

func TestEngine_SubdomainRule(t *testing.T) {
	e := NewEngine()

	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityDomain, "mail.example.com", "src1"),
		makeEntity("e2", resolution.EntityDomain, "example.com", "src2"),
	}

	correlations := e.Correlate(entities)

	foundRule := false
	for _, c := range correlations {
		if c.Rule == "subdomain_parent" {
			foundRule = true
			if c.Confidence != 85 {
				t.Errorf("expected confidence 85, got %d", c.Confidence)
			}
		}
	}
	if !foundRule {
		t.Error("expected subdomain_parent correlation")
	}
}

func TestEngine_NoCorrelation(t *testing.T) {
	e := NewEngine()

	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityIP, "8.8.8.8", "src1"),
		makeEntity("e2", resolution.EntityUsername, "johndoe", "src2"),
	}

	correlations := e.Correlate(entities)
	if len(correlations) != 0 {
		t.Errorf("expected 0 correlations, got %d", len(correlations))
	}
}

func TestEngine_CustomRule(t *testing.T) {
	e := NewEngine()

	e.AddRule(Rule{
		Name:       "custom_test",
		SourceType: resolution.EntityIP,
		TargetType: resolution.EntityIP,
		MatchFunc: func(src, tgt *resolution.Entity) (bool, int, string) {
			return true, 75, "custom match"
		},
	})

	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityIP, "1.2.3.4", "src1"),
		makeEntity("e2", resolution.EntityIP, "5.6.7.8", "src2"),
	}

	correlations := e.Correlate(entities)
	found := false
	for _, c := range correlations {
		if c.Rule == "custom_test" {
			found = true
		}
	}
	if !found {
		t.Error("expected custom_test correlation")
	}
}

func TestEngine_NoDuplicates(t *testing.T) {
	e := NewEngine()

	entities := []*resolution.Entity{
		makeEntity("e1", resolution.EntityEmail, "a@x.com", "s1"),
		makeEntity("e2", resolution.EntityDomain, "x.com", "s1"),
	}

	correlations := e.Correlate(entities)

	// Aynı pair + rule sadece 1 kez eklenmeli
	ruleCount := make(map[string]int)
	for _, c := range correlations {
		ruleCount[c.Rule]++
	}
	for rule, count := range ruleCount {
		if count > 1 {
			t.Errorf("duplicate correlation for rule %q: found %d times", rule, count)
		}
	}
}
