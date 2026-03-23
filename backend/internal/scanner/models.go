package scanner

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CategoryName mirrors the TypeScript CategoryName type from the scanner CLI.
type CategoryName string

const (
	CategoryDataQuality      CategoryName = "data-quality"
	CategoryProductDiscovery CategoryName = "product-discovery"
	CategoryCheckoutFlow     CategoryName = "checkout-flow"
	CategoryProtocol         CategoryName = "protocol-compliance"
)

// ScoreKiller holds score-killer conditions for a category.
type ScoreKiller struct {
	Condition string `json:"condition" bson:"condition"`
	Cap       int    `json:"cap" bson:"cap"`
}

// CategoryResult mirrors CategoryResult from the scanner CLI.
// We omit the full scenarios array — just summary fields needed by the API.
type CategoryResult struct {
	Category        CategoryName  `json:"category" bson:"category"`
	Tested          bool          `json:"tested" bson:"tested"`
	Score           float64       `json:"score" bson:"score"`
	CappedScore     float64       `json:"cappedScore" bson:"cappedScore"`
	Weight          float64       `json:"weight" bson:"weight"`
	EffectiveWeight float64       `json:"effectiveWeight" bson:"effectiveWeight"`
	ScoreKillers    []ScoreKiller `json:"scoreKillers" bson:"scoreKillers"`
}

// ScanResult is the parsed output of the Node.js scanner CLI (TestRunResult shape).
type ScanResult struct {
	ServerIdentifier string           `json:"serverIdentifier" bson:"serverIdentifier"`
	Timestamp        int64            `json:"timestamp" bson:"timestamp"`
	DurationMs       int              `json:"durationMs" bson:"durationMs"`
	CompositeScore   int              `json:"compositeScore" bson:"compositeScore"`
	Categories       []CategoryResult `json:"categories" bson:"categories"`
	TestedCategories []CategoryName   `json:"testedCategories" bson:"testedCategories"`
	PartialResults   bool             `json:"partialResults" bson:"partialResults"`
	PartialReason    string           `json:"partialReason,omitempty" bson:"partialReason,omitempty"`
	Version          string           `json:"version" bson:"version"`
	SchemaVersion    string           `json:"schemaVersion" bson:"schemaVersion"`
	ScenarioCount    int              `json:"scenarioCount" bson:"scenarioCount"`
}

// StoredScan is ScanResult enriched with storage metadata.
type StoredScan struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Domain    string             `json:"domain" bson:"domain"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	TenantID  *primitive.ObjectID `json:"tenantId,omitempty" bson:"tenantId,omitempty"`
	ScanResult
}
