package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Service orchestrates running the Node.js scanner CLI and persisting results.
type Service struct {
	store        *Store
	trackedStore *TrackedStoreStore
	scannerPath  string // path to the scanner dist directory (containing cli.js)
}

// NewService creates a new scanner Service.
// scannerPath can be overridden by the SCANNER_PATH environment variable;
// otherwise it is resolved relative to the running binary.
func NewService(database *mongo.Database) *Service {
	scannerPath := resolveScanner()
	slog.Info("Scanner service initialised", "scannerPath", scannerPath)
	return &Service{
		store:        NewStore(database),
		trackedStore: NewTrackedStoreStore(database),
		scannerPath:  scannerPath,
	}
}

// TrackedStores returns the TrackedStoreStore for use by handlers.
func (s *Service) TrackedStores() *TrackedStoreStore {
	return s.trackedStore
}

// resolveScanner resolves the path to scanner/dist relative to the binary,
// or uses the SCANNER_PATH env var when set.
func resolveScanner() string {
	if p := os.Getenv("SCANNER_PATH"); p != "" {
		return p
	}

	// Executable-relative: <binary>/../scanner/dist
	exe, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "scanner", "dist")
		if _, err := os.Stat(filepath.Join(candidate, "cli.js")); err == nil {
			return candidate
		}
	}

	// Source-tree relative (development): walk up until we find scanner/dist
	// This works when running via `go run ./cmd/server` from the backend dir.
	wd, _ := os.Getwd()
	for dir := wd; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "scanner", "dist")
		if _, err := os.Stat(filepath.Join(candidate, "cli.js")); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // filesystem root
		}
	}

	// Last resort: return a sensible default that will produce a clear error.
	return filepath.Join("..", "scanner", "dist")
}

// nodeExecutable returns "node" or "node.exe" depending on OS.
func nodeExecutable() string {
	if runtime.GOOS == "windows" {
		return "node.exe"
	}
	return "node"
}

// ScanStore runs the scanner CLI against domain and stores the result.
// tenantID may be nil for unauthenticated (free-tier) scans.
func (s *Service) ScanStore(ctx context.Context, domain string, tenantID *primitive.ObjectID) (*StoredScan, error) {
	cliPath := filepath.Join(s.scannerPath, "cli.js")

	// Sanitise domain — strip schemes and paths to avoid shell-injection via exec args.
	domain = sanitiseDomain(domain)
	if domain == "" {
		return nil, fmt.Errorf("empty or invalid domain")
	}

	// The CLI writes JSON to a file (not stdout). Use a temp file so we can read the result.
	tmpFile, err := os.CreateTemp("", "mcplens-scan-*.json")
	if err != nil {
		return nil, fmt.Errorf("create temp output file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // clean up temp file regardless of outcome

	node := nodeExecutable()
	cmd := exec.CommandContext(ctx, node, cliPath, "scan", domain, "--format", "json", "--out", tmpPath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Info("Running scanner CLI", "domain", domain, "cli", cliPath, "outFile", tmpPath)
	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		slog.Error("Scanner CLI failed", "domain", domain, "stderr", stderrStr, "stdout", stdoutStr, "error", err)
		return nil, fmt.Errorf("scanner CLI error for %q: %w\nstderr: %s", domain, err, stderrStr)
	}

	// Log stdout progress lines at debug level.
	if stdoutStr := strings.TrimSpace(stdout.String()); stdoutStr != "" {
		slog.Debug("Scanner CLI stdout", "domain", domain, "output", stdoutStr)
	}

	// Read the JSON result from the temp file.
	jsonData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read scanner output file: %w", err)
	}

	var result ScanResult
	if err := json.Unmarshal(jsonData, &result); err != nil {
		preview := len(jsonData)
		if preview > 500 {
			preview = 500
		}
		slog.Error("Failed to parse scanner JSON output", "domain", domain, "raw", string(jsonData[:preview]))
		return nil, fmt.Errorf("parse scanner output: %w", err)
	}

	id, err := s.store.SaveScan(ctx, domain, &result, tenantID)
	if err != nil {
		return nil, fmt.Errorf("save scan: %w", err)
	}

	stored := &StoredScan{
		ID:         id,
		Domain:     domain,
		ScanResult: result,
	}
	return stored, nil
}

// GetScan retrieves a previously stored scan by ID.
func (s *Service) GetScan(ctx context.Context, scanID string) (*StoredScan, error) {
	return s.store.GetScan(ctx, scanID)
}

// GetLatestScan returns the most recent stored scan for a domain.
func (s *Service) GetLatestScan(ctx context.Context, domain string) (*StoredScan, error) {
	domain = sanitiseDomain(domain)
	return s.store.GetLatestScan(ctx, domain)
}

// ListScans returns a paginated list of scans for a tenant.
func (s *Service) ListScans(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]StoredScan, int64, error) {
	return s.store.ListScans(ctx, tenantID, page, limit)
}

// sanitiseDomain strips leading scheme/slashes so "https://allbirds.com/foo" → "allbirds.com".
func sanitiseDomain(domain string) string {
	// Remove scheme
	for _, scheme := range []string{"https://", "http://"} {
		domain = strings.TrimPrefix(domain, scheme)
	}
	// Keep only the host part (no path)
	if idx := strings.IndexByte(domain, '/'); idx >= 0 {
		domain = domain[:idx]
	}
	domain = strings.TrimSpace(domain)
	return domain
}

