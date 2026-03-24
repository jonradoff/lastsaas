package scanner

import (
	"testing"
)

func TestSanitiseDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain domain", "allbirds.com", "allbirds.com"},
		{"https prefix", "https://allbirds.com", "allbirds.com"},
		{"http prefix", "http://allbirds.com", "allbirds.com"},
		{"with path", "https://allbirds.com/products", "allbirds.com"},
		{"with trailing slash", "https://allbirds.com/", "allbirds.com"},
		{"whitespace", "  allbirds.com  ", "allbirds.com"},
		{"empty string", "", ""},
		{"only scheme", "https://", ""},
		{"subdomain", "shop.example.com", "shop.example.com"},
		{"port included", "localhost:3000/api", "localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitiseDomain(tt.input)
			if got != tt.expected {
				t.Errorf("sanitiseDomain(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNodeExecutable(t *testing.T) {
	exe := nodeExecutable()
	if exe != "node" && exe != "node.exe" {
		t.Errorf("nodeExecutable() = %q, want 'node' or 'node.exe'", exe)
	}
}

func TestCategoryNameConstants(t *testing.T) {
	categories := []CategoryName{
		CategoryDataQuality,
		CategoryProductDiscovery,
		CategoryCheckoutFlow,
		CategoryProtocol,
	}

	expected := []string{
		"data-quality",
		"product-discovery",
		"checkout-flow",
		"protocol-compliance",
	}

	for i, cat := range categories {
		if string(cat) != expected[i] {
			t.Errorf("category %d: got %q, want %q", i, cat, expected[i])
		}
	}
}

func TestStoredScanEmbedding(t *testing.T) {
	scan := StoredScan{
		Domain: "test.com",
		ScanResult: ScanResult{
			CompositeScore: 85,
			Version:        "1.0",
			ScenarioCount:  10,
		},
	}

	if scan.CompositeScore != 85 {
		t.Errorf("embedded CompositeScore = %d, want 85", scan.CompositeScore)
	}
	if scan.Domain != "test.com" {
		t.Errorf("Domain = %q, want 'test.com'", scan.Domain)
	}
}

func TestTrackedStoreTrendValues(t *testing.T) {
	validTrends := []string{"up", "down", "stable"}
	for _, trend := range validTrends {
		ts := TrackedStore{Trend: trend}
		if ts.Trend != trend {
			t.Errorf("Trend = %q, want %q", ts.Trend, trend)
		}
	}
}

func TestTrackedStoreLabelField(t *testing.T) {
	ts := TrackedStore{
		Domain: "allbirds.com",
		Label:  "My Store",
	}
	if ts.Label != "My Store" {
		t.Errorf("Label = %q, want 'My Store'", ts.Label)
	}

	// Label can be empty
	ts2 := TrackedStore{Domain: "competitor.com"}
	if ts2.Label != "" {
		t.Errorf("Empty label = %q, want empty string", ts2.Label)
	}
}

func TestComparisonSeriesStructure(t *testing.T) {
	series := ComparisonSeries{
		Domain: "allbirds.com",
		Label:  "My Store",
		Points: []ComparisonPoint{
			{Score: 85},
			{Score: 90},
		},
	}

	if len(series.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(series.Points))
	}
	if series.Points[0].Score != 85 {
		t.Errorf("first point score = %d, want 85", series.Points[0].Score)
	}
	if series.Label != "My Store" {
		t.Errorf("label = %q, want 'My Store'", series.Label)
	}
}
