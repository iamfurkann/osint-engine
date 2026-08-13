package graph

import (
	"testing"
)

func buildTestGraph() *Graph {
	//  A ---> B ---> C
	//  |             |
	//  +----> D -----+
	g := NewGraph()
	g.AddNode(Node{ID: "A", Type: "email", Value: "a@x.com", Confidence: 90})
	g.AddNode(Node{ID: "B", Type: "domain", Value: "x.com", Confidence: 80})
	g.AddNode(Node{ID: "C", Type: "ip", Value: "1.2.3.4", Confidence: 70})
	g.AddNode(Node{ID: "D", Type: "domain", Value: "y.com", Confidence: 60})

	g.AddEdge(Edge{Source: "A", Target: "B", Type: "email_domain", Confidence: 90})
	g.AddEdge(Edge{Source: "B", Target: "C", Type: "dns_resolve", Confidence: 85})
	g.AddEdge(Edge{Source: "A", Target: "D", Type: "shared_source", Confidence: 70})
	g.AddEdge(Edge{Source: "D", Target: "C", Type: "dns_resolve", Confidence: 75})

	return g
}

func TestGraph_AddAndCount(t *testing.T) {
	g := buildTestGraph()

	if g.NodeCount() != 4 {
		t.Errorf("expected 4 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 4 {
		t.Errorf("expected 4 edges, got %d", g.EdgeCount())
	}
}

func TestGraph_GetNode(t *testing.T) {
	g := buildTestGraph()

	node, ok := g.GetNode("A")
	if !ok {
		t.Fatal("expected to find node A")
	}
	if node.Value != "a@x.com" {
		t.Errorf("expected 'a@x.com', got %q", node.Value)
	}

	_, ok = g.GetNode("Z")
	if ok {
		t.Error("expected no node Z")
	}
}

func TestGraph_Neighbors_1Hop(t *testing.T) {
	g := buildTestGraph()

	neighbors := g.Neighbors("A", 1)
	if len(neighbors) != 2 { // B, D
		t.Errorf("expected 2 neighbors for A (1-hop), got %d", len(neighbors))
	}
}

func TestGraph_Neighbors_2Hop(t *testing.T) {
	g := buildTestGraph()

	neighbors := g.Neighbors("A", 2)
	if len(neighbors) != 3 { // B, D (1-hop) + C (2-hop)
		t.Errorf("expected 3 neighbors for A (2-hop), got %d", len(neighbors))
	}
}

func TestGraph_Neighbors_Nonexistent(t *testing.T) {
	g := buildTestGraph()
	neighbors := g.Neighbors("Z", 1)
	if neighbors != nil {
		t.Error("expected nil for nonexistent node")
	}
}

func TestGraph_ShortestPath(t *testing.T) {
	g := buildTestGraph()

	// A → B → C (2 hops) veya A → D → C (2 hops)
	path := g.ShortestPath("A", "C")
	if path == nil {
		t.Fatal("expected to find path from A to C")
	}
	if len(path) != 3 { // A → ? → C
		t.Errorf("expected path of length 3, got %d", len(path))
	}
	if path[0].ID != "A" {
		t.Errorf("path should start with A, got %s", path[0].ID)
	}
	if path[len(path)-1].ID != "C" {
		t.Errorf("path should end with C, got %s", path[len(path)-1].ID)
	}
}

func TestGraph_ShortestPath_SameNode(t *testing.T) {
	g := buildTestGraph()
	path := g.ShortestPath("A", "A")
	if len(path) != 1 {
		t.Errorf("expected path of length 1, got %d", len(path))
	}
}

func TestGraph_ShortestPath_NoPath(t *testing.T) {
	g := NewGraph()
	g.AddNode(Node{ID: "X", Type: "ip", Value: "1.1.1.1"})
	g.AddNode(Node{ID: "Y", Type: "ip", Value: "2.2.2.2"})
	// Bağlantı yok

	path := g.ShortestPath("X", "Y")
	if path != nil {
		t.Error("expected nil path for disconnected nodes")
	}
}

func TestGraph_Subgraph(t *testing.T) {
	g := buildTestGraph()

	sub := g.Subgraph("A", 1)
	if sub.NodeCount() != 3 { // A, B, D
		t.Errorf("expected 3 nodes in subgraph, got %d", sub.NodeCount())
	}
}

func TestGraph_Filter(t *testing.T) {
	g := buildTestGraph()

	// Confidence ≥ 80
	filtered := g.Filter(80, nil)
	if filtered.NodeCount() != 2 { // A(90), B(80)
		t.Errorf("expected 2 nodes with confidence ≥80, got %d", filtered.NodeCount())
	}

	// Type filter
	domainOnly := g.Filter(0, []string{"domain"})
	if domainOnly.NodeCount() != 2 { // B, D
		t.Errorf("expected 2 domain nodes, got %d", domainOnly.NodeCount())
	}
}

func TestGraph_AllNodes(t *testing.T) {
	g := buildTestGraph()
	nodes := g.AllNodes()
	if len(nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(nodes))
	}
}
