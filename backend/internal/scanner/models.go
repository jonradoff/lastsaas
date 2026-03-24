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

// --- AI Quality Assessment ---

// AIFinding is a single finding from the LLM quality assessment.
type AIFinding struct {
	Title         string `json:"title" bson:"title"`
	Category      string `json:"category" bson:"category"`
	Severity      string `json:"severity" bson:"severity"`
	Explanation   string `json:"explanation" bson:"explanation"`
	RevenueImpact string `json:"revenueImpact" bson:"revenueImpact"`
	Fix           string `json:"fix" bson:"fix"`
}

// AIQuerySimulation is a simulated buyer query and its evaluation.
type AIQuerySimulation struct {
	Query           string `json:"query" bson:"query"`
	WouldFindResult bool   `json:"wouldFindResult" bson:"wouldFindResult"`
	Confidence      int    `json:"confidence" bson:"confidence"`
	Explanation     string `json:"explanation" bson:"explanation"`
}

// AISubScore holds a score and summary for a Layer 2 sub-category.
type AISubScore struct {
	Score   int    `json:"score" bson:"score"`
	Summary string `json:"summary" bson:"summary"`
}

// AIDataCompleteness includes missing attributes beyond the base sub-score.
type AIDataCompleteness struct {
	Score             int      `json:"score" bson:"score"`
	MissingAttributes []string `json:"missingAttributes" bson:"missingAttributes"`
	Summary           string   `json:"summary" bson:"summary"`
}

// AITokenUsage tracks the API token consumption.
type AITokenUsage struct {
	Input  int `json:"input" bson:"input"`
	Output int `json:"output" bson:"output"`
}

// AIAssessment is the full result of the LLM quality assessment.
type AIAssessment struct {
	OverallQualityScore   int                     `json:"overallQualityScore" bson:"overallQualityScore"`
	ProductRelevance      AISubScore           `json:"productRelevance" bson:"productRelevance"`
	DescriptionQuality    AISubScore           `json:"descriptionQuality" bson:"descriptionQuality"`
	DataCompleteness      AIDataCompleteness   `json:"dataCompleteness" bson:"dataCompleteness"`
	QuerySimulations      []AIQuerySimulation  `json:"querySimulations" bson:"querySimulations"`
	Findings              []AIFinding          `json:"findings" bson:"findings"`
	CompetitiveComparison string                   `json:"competitiveComparison" bson:"competitiveComparison"`
	ModelUsed             string                   `json:"modelUsed" bson:"modelUsed"`
	TokenUsage            AITokenUsage         `json:"tokenUsage" bson:"tokenUsage"`
	CostEstimateUsd       float64                  `json:"costEstimateUsd" bson:"costEstimateUsd"`
	DurationMs            int                      `json:"durationMs" bson:"durationMs"`
}

// --- Agent Simulation ---

// AgentStep is a single step in the agent's shopping journey.
type AgentStep struct {
	StepNumber int                    `json:"stepNumber" bson:"stepNumber"`
	Action     string                 `json:"action" bson:"action"` // think, tool_call, tool_result, decision, failure
	ToolName   string                 `json:"toolName,omitempty" bson:"toolName,omitempty"`
	ToolArgs   map[string]interface{} `json:"toolArgs,omitempty" bson:"toolArgs,omitempty"`
	ToolResult interface{}            `json:"toolResult,omitempty" bson:"toolResult,omitempty"`
	Reasoning  string                 `json:"reasoning" bson:"reasoning"`
	DurationMs int                    `json:"durationMs" bson:"durationMs"`
}

// SelectedProduct is the product the agent chose.
type SelectedProduct struct {
	ID     string  `json:"id" bson:"id"`
	Name   string  `json:"name" bson:"name"`
	Price  float64 `json:"price,omitempty" bson:"price,omitempty"`
	Reason string  `json:"reason" bson:"reason"`
}

// FailurePoint is where the agent got stuck.
type FailurePoint struct {
	StepNumber int    `json:"stepNumber" bson:"stepNumber"`
	Reason     string `json:"reason" bson:"reason"`
	Context    string `json:"context" bson:"context"`
}

// ShoppingScenario is one complete shopping intent with transcript.
type ShoppingScenario struct {
	Intent          string           `json:"intent" bson:"intent"`
	Persona         string           `json:"persona" bson:"persona"`
	Steps           []AgentStep      `json:"steps" bson:"steps"`
	Outcome         string           `json:"outcome" bson:"outcome"` // completed, failed, abandoned
	SelectedProduct *SelectedProduct `json:"selectedProduct,omitempty" bson:"selectedProduct,omitempty"`
	FailurePoint    *FailurePoint    `json:"failurePoint,omitempty" bson:"failurePoint,omitempty"`
	TotalSteps      int              `json:"totalSteps" bson:"totalSteps"`
	DurationMs      int              `json:"durationMs" bson:"durationMs"`
	TokenUsage      AITokenUsage     `json:"tokenUsage" bson:"tokenUsage"`
}

// AgentSimulation is the full result of buyer agent simulation.
type AgentSimulation struct {
	Scenarios       []ShoppingScenario `json:"scenarios" bson:"scenarios"`
	ModelUsed       string             `json:"modelUsed" bson:"modelUsed"`
	TotalTokenUsage AITokenUsage       `json:"totalTokenUsage" bson:"totalTokenUsage"`
	CostEstimateUsd float64            `json:"costEstimateUsd" bson:"costEstimateUsd"`
	DurationMs      int                `json:"durationMs" bson:"durationMs"`
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
	AIAssessment    *AIAssessment    `json:"aiAssessment,omitempty" bson:"aiAssessment,omitempty"`
	AgentSimulation *AgentSimulation `json:"agentSimulation,omitempty" bson:"agentSimulation,omitempty"`
}

// StoredScan is ScanResult enriched with storage metadata.
type StoredScan struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Domain    string             `json:"domain" bson:"domain"`
	CreatedAt time.Time          `json:"createdAt" bson:"createdAt"`
	TenantID  *primitive.ObjectID `json:"tenantId,omitempty" bson:"tenantId,omitempty"`
	ScanResult
}

// TrackedStore is a store domain that a tenant has pinned for ongoing monitoring.
type TrackedStore struct {
	ID            primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	Domain        string              `json:"domain" bson:"domain"`
	Label         string              `json:"label" bson:"label,omitempty"`
	TenantID      primitive.ObjectID  `json:"tenantId" bson:"tenantId"`
	AddedAt       time.Time           `json:"addedAt" bson:"addedAt"`
	LastScannedAt *time.Time          `json:"lastScannedAt,omitempty" bson:"lastScannedAt,omitempty"`
	CurrentScore  int                 `json:"currentScore" bson:"currentScore"`
	PreviousScore int                 `json:"previousScore" bson:"previousScore"`
	Trend         string              `json:"trend" bson:"trend"` // "up", "down", "stable"
}

// ComparisonSeries holds score history for a single tracked store (used in comparison chart).
type ComparisonSeries struct {
	Domain string           `json:"domain"`
	Label  string           `json:"label"`
	Points []ComparisonPoint `json:"points"`
}

// ComparisonPoint is a single score data point.
type ComparisonPoint struct {
	Date  time.Time `json:"date"`
	Score int       `json:"score"`
}
