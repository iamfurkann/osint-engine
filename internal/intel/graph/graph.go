package graph

import (
	"fmt"
	"sync"
)

// Node, graf'taki bir düğümdür (entity'ye karşılık gelir).
type Node struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Confidence int    `json:"confidence"`
}

// Edge, iki düğüm arasındaki ilişkidir (korelasyona karşılık gelir).
type Edge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Type       string `json:"type"`       // İlişki tipi
	Confidence int    `json:"confidence"` // İlişki güven puanı
	Evidence   string `json:"evidence"`
}

// Graph, in-memory adjacency list tabanlı graf yapısıdır.
type Graph struct {
	nodes map[string]Node
	edges map[string][]Edge // nodeID → çıkan kenarlar
	mu    sync.RWMutex
}

// NewGraph, yeni bir boş graf oluşturur.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]Node),
		edges: make(map[string][]Edge),
	}
}

// AddNode, grafa bir düğüm ekler.
func (g *Graph) AddNode(node Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[node.ID] = node
}

// AddEdge, grafa çift yönlü bir kenar ekler.
func (g *Graph) AddEdge(edge Edge) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.edges[edge.Source] = append(g.edges[edge.Source], edge)
	// Ters kenar (undirected graf)
	reverse := Edge{
		Source:     edge.Target,
		Target:     edge.Source,
		Type:       edge.Type,
		Confidence: edge.Confidence,
		Evidence:   edge.Evidence,
	}
	g.edges[edge.Target] = append(g.edges[edge.Target], reverse)
}

// GetNode, ID ile düğüm döndürür.
func (g *Graph) GetNode(id string) (Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n, ok := g.nodes[id]
	return n, ok
}

// NodeCount, graf'taki düğüm sayısını döndürür.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount, graf'taki benzersiz kenar sayısını döndürür (çift yönlü sayılmaz).
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	total := 0
	for _, edges := range g.edges {
		total += len(edges)
	}
	return total / 2 // Çift yönlü kenarları sayma
}

// AllNodes, tüm düğümleri döndürür.
func (g *Graph) AllNodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		result = append(result, n)
	}
	return result
}

// Neighbors, belirli bir düğümün N-hop komşularını döndürür (BFS).
func (g *Graph) Neighbors(entityID string, hops int) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if _, ok := g.nodes[entityID]; !ok {
		return nil
	}

	if hops <= 0 {
		hops = 1
	}

	visited := map[string]bool{entityID: true}
	queue := []string{entityID}
	var result []Node

	for depth := 0; depth < hops && len(queue) > 0; depth++ {
		nextQueue := []string{}
		for _, nodeID := range queue {
			for _, edge := range g.edges[nodeID] {
				if !visited[edge.Target] {
					visited[edge.Target] = true
					nextQueue = append(nextQueue, edge.Target)
					if node, ok := g.nodes[edge.Target]; ok {
						result = append(result, node)
					}
				}
			}
		}
		queue = nextQueue
	}

	return result
}

// ShortestPath, iki düğüm arasındaki en kısa yolu BFS ile bulur.
// Yol bulunamazsa nil döner.
func (g *Graph) ShortestPath(from, to string) []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if from == to {
		if n, ok := g.nodes[from]; ok {
			return []Node{n}
		}
		return nil
	}

	// BFS
	visited := map[string]bool{from: true}
	parent := map[string]string{}
	queue := []string{from}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, edge := range g.edges[current] {
			if !visited[edge.Target] {
				visited[edge.Target] = true
				parent[edge.Target] = current
				if edge.Target == to {
					return g.reconstructPath(from, to, parent)
				}
				queue = append(queue, edge.Target)
			}
		}
	}

	return nil // Yol bulunamadı
}

// reconstructPath, BFS parent map'inden yolu yeniden oluşturur.
func (g *Graph) reconstructPath(from, to string, parent map[string]string) []Node {
	var path []Node
	current := to
	for current != from {
		if node, ok := g.nodes[current]; ok {
			path = append([]Node{node}, path...)
		}
		current = parent[current]
	}
	if node, ok := g.nodes[from]; ok {
		path = append([]Node{node}, path...)
	}
	return path
}

// Subgraph, belirli bir entity merkezinde N-depth alt-graf çıkarır.
func (g *Graph) Subgraph(entityID string, depth int) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sub := NewGraph()
	if _, ok := g.nodes[entityID]; !ok {
		return sub
	}

	visited := map[string]bool{entityID: true}
	queue := []string{entityID}

	if node, ok := g.nodes[entityID]; ok {
		sub.AddNode(node)
	}

	for d := 0; d < depth && len(queue) > 0; d++ {
		nextQueue := []string{}
		for _, nodeID := range queue {
			for _, edge := range g.edges[nodeID] {
				if !visited[edge.Target] {
					visited[edge.Target] = true
					nextQueue = append(nextQueue, edge.Target)
					if node, ok := g.nodes[edge.Target]; ok {
						sub.AddNode(node)
					}
				}
				// Kenarı alt-grafa ekle (her iki uç da alt-graftaysa)
				if visited[edge.Source] && visited[edge.Target] {
					sub.addEdgeOnce(edge)
				}
			}
		}
		queue = nextQueue
	}

	return sub
}

// addEdgeOnce, tekrarsız kenar ekler (internal use).
func (g *Graph) addEdgeOnce(edge Edge) {
	for _, e := range g.edges[edge.Source] {
		if e.Target == edge.Target && e.Type == edge.Type {
			return // Zaten var
		}
	}
	g.edges[edge.Source] = append(g.edges[edge.Source], edge)
	reverse := Edge{Source: edge.Target, Target: edge.Source, Type: edge.Type, Confidence: edge.Confidence, Evidence: edge.Evidence}
	g.edges[edge.Target] = append(g.edges[edge.Target], reverse)
}

// Filter, minimum güven eşiği ve tip filtresiyle yeni bir graf döndürür.
func (g *Graph) Filter(minConfidence int, types []string) *Graph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	filtered := NewGraph()
	typeSet := make(map[string]bool)
	for _, t := range types {
		typeSet[t] = true
	}

	// Uygun node'ları ekle
	for _, node := range g.nodes {
		if node.Confidence >= minConfidence {
			if len(typeSet) == 0 || typeSet[node.Type] {
				filtered.AddNode(node)
			}
		}
	}

	// Uygun edge'leri ekle (her iki uç da filtered'da olmalı)
	added := make(map[string]bool)
	for _, edges := range g.edges {
		for _, edge := range edges {
			key := fmt.Sprintf("%s:%s", edge.Source, edge.Target)
			reverseKey := fmt.Sprintf("%s:%s", edge.Target, edge.Source)
			if added[key] || added[reverseKey] {
				continue
			}
			if _, ok := filtered.nodes[edge.Source]; !ok {
				continue
			}
			if _, ok := filtered.nodes[edge.Target]; !ok {
				continue
			}
			if edge.Confidence >= minConfidence {
				filtered.AddEdge(edge)
				added[key] = true
			}
		}
	}

	return filtered
}

// Nodes, graftaki tüm düğümleri döner.
func (g *Graph) Nodes() []Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	nodes := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// Edges, graftaki tüm kenarları döner.
func (g *Graph) Edges() []Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var edges []Edge
	for _, eList := range g.edges {
		edges = append(edges, eList...)
	}
	return edges
}
