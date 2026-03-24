package rescan

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRescanJobFields(t *testing.T) {
	job := rescanJob{
		TrackedStoreID: primitive.NewObjectID(),
		TenantID:       primitive.NewObjectID(),
		Domain:         "allbirds.com",
		CurrentScore:   75,
		FrequencyHours: 24,
	}

	if job.Domain != "allbirds.com" {
		t.Errorf("expected domain 'allbirds.com', got '%s'", job.Domain)
	}
	if job.FrequencyHours != 24 {
		t.Errorf("expected frequency 24h, got %d", job.FrequencyHours)
	}
}

func TestScoreDropThreshold(t *testing.T) {
	tests := []struct {
		name         string
		currentScore int
		newScore     int
		shouldAlert  bool
	}{
		{"no drop", 75, 75, false},
		{"small drop", 75, 72, false},
		{"at threshold", 75, 70, true},
		{"large drop", 80, 50, true},
		{"score increase", 60, 80, false},
		{"drop of exactly 4", 75, 71, false},
		{"drop of exactly 5", 75, 70, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drop := tt.currentScore - tt.newScore
			wouldAlert := drop >= scoreDropThreshold
			if wouldAlert != tt.shouldAlert {
				t.Errorf("currentScore=%d, newScore=%d: expected alert=%v, got alert=%v (drop=%d)",
					tt.currentScore, tt.newScore, tt.shouldAlert, wouldAlert, drop)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	if leaseTTL != 2*time.Minute {
		t.Errorf("leaseTTL = %v, want 2m", leaseTTL)
	}
	if renewalTick != 30*time.Second {
		t.Errorf("renewalTick = %v, want 30s", renewalTick)
	}
	if cycleTick != 5*time.Minute {
		t.Errorf("cycleTick = %v, want 5m", cycleTick)
	}
	if batchSize != 10 {
		t.Errorf("batchSize = %d, want 10", batchSize)
	}
	if scanTimeout != 90*time.Second {
		t.Errorf("scanTimeout = %v, want 90s", scanTimeout)
	}
	if scoreDropThreshold != 5 {
		t.Errorf("scoreDropThreshold = %d, want 5", scoreDropThreshold)
	}
}

func TestNewService(t *testing.T) {
	svc := New(nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.holderID == "" {
		t.Error("expected non-empty holderID")
	}
	if svc.stop == nil {
		t.Error("expected non-nil stop channel")
	}
}

func TestServiceStopDoesNotPanic(t *testing.T) {
	svc := New(nil, nil, nil, nil)
	// Stop without Start should not panic
	// (stop channel is buffered by close semantics)
	svc.Stop()
}
