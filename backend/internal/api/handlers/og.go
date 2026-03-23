package handlers

import (
	"fmt"
	"net/http"

	"lastsaas/internal/scanner"

	"github.com/gorilla/mux"
)

// OGHandler serves OG image and badge endpoints for social sharing.
type OGHandler struct {
	service *scanner.Service
}

// NewOGHandler creates a new OGHandler.
func NewOGHandler(svc *scanner.Service) *OGHandler {
	return &OGHandler{service: svc}
}

// scoreColorOG returns a hex colour string based on the score.
func scoreColorOG(score int) string {
	if score >= 80 {
		return "#22c55e"
	}
	if score >= 50 {
		return "#f59e0b"
	}
	return "#ef4444"
}

// OGImage handles GET /api/og/{domain}.
// Returns a 1200x630 SVG image suitable for use as an OG image.
func (h *OGHandler) OGImage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}

	scan, err := h.service.GetLatestScan(r.Context(), domain)
	if err != nil {
		http.Error(w, "failed to retrieve scan", http.StatusInternalServerError)
		return
	}

	var svg string
	if scan == nil {
		svg = buildOGSVG(domain, -1)
	} else {
		svg = buildOGSVG(domain, scan.CompositeScore)
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	fmt.Fprint(w, svg)
}

// buildOGSVG generates the SVG string. Pass score=-1 to show "Not Scanned".
func buildOGSVG(domain string, score int) string {
	const width = 1200
	const height = 630

	// Escape domain for safe inclusion in SVG text (handle < > & " ')
	safeDomain := svgEscape(domain)

	if score < 0 {
		// Not scanned variant
		return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="%d" height="%d" fill="#0f172a"/>
  <!-- grid lines for visual texture -->
  <line x1="0" y1="210" x2="%d" y2="210" stroke="#1e293b" stroke-width="1"/>
  <line x1="0" y1="420" x2="%d" y2="420" stroke="#1e293b" stroke-width="1"/>
  <line x1="400" y1="0" x2="400" y2="%d" stroke="#1e293b" stroke-width="1"/>
  <line x1="800" y1="0" x2="800" y2="%d" stroke="#1e293b" stroke-width="1"/>
  <!-- domain -->
  <text x="600" y="180" font-family="ui-sans-serif, system-ui, sans-serif" font-size="52" font-weight="700" fill="#f8fafc" text-anchor="middle" dominant-baseline="middle">%s</text>
  <!-- score placeholder -->
  <text x="600" y="315" font-family="ui-sans-serif, system-ui, sans-serif" font-size="140" font-weight="800" fill="#475569" text-anchor="middle" dominant-baseline="middle">—</text>
  <!-- label -->
  <text x="600" y="450" font-family="ui-sans-serif, system-ui, sans-serif" font-size="36" font-weight="600" fill="#94a3b8" text-anchor="middle" dominant-baseline="middle">Not Yet Scanned</text>
  <!-- powered by -->
  <text x="600" y="575" font-family="ui-sans-serif, system-ui, sans-serif" font-size="24" fill="#475569" text-anchor="middle" dominant-baseline="middle">Powered by MCPLens</text>
</svg>`, width, height, width, height, width, height, width, width, height, height, safeDomain)
	}

	color := scoreColorOG(score)
	scoreStr := fmt.Sprintf("%d", score)

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="%d" height="%d" fill="#0f172a"/>
  <!-- grid lines for visual texture -->
  <line x1="0" y1="210" x2="%d" y2="210" stroke="#1e293b" stroke-width="1"/>
  <line x1="0" y1="420" x2="%d" y2="420" stroke="#1e293b" stroke-width="1"/>
  <line x1="400" y1="0" x2="400" y2="%d" stroke="#1e293b" stroke-width="1"/>
  <line x1="800" y1="0" x2="800" y2="%d" stroke="#1e293b" stroke-width="1"/>
  <!-- domain -->
  <text x="600" y="160" font-family="ui-sans-serif, system-ui, sans-serif" font-size="52" font-weight="700" fill="#f8fafc" text-anchor="middle" dominant-baseline="middle">%s</text>
  <!-- score number -->
  <text x="600" y="320" font-family="ui-sans-serif, system-ui, sans-serif" font-size="200" font-weight="800" fill="%s" text-anchor="middle" dominant-baseline="middle">%s</text>
  <!-- /100 -->
  <text x="720" y="430" font-family="ui-sans-serif, system-ui, sans-serif" font-size="36" fill="#64748b" text-anchor="start" dominant-baseline="middle">/ 100</text>
  <!-- label -->
  <text x="600" y="510" font-family="ui-sans-serif, system-ui, sans-serif" font-size="36" font-weight="600" fill="#94a3b8" text-anchor="middle" dominant-baseline="middle">Agent Readiness Score</text>
  <!-- powered by -->
  <text x="600" y="590" font-family="ui-sans-serif, system-ui, sans-serif" font-size="24" fill="#475569" text-anchor="middle" dominant-baseline="middle">Powered by MCPLens</text>
</svg>`, width, height, width, height, width, height, width, width, height, height, safeDomain, color, scoreStr)
}

// svgEscape replaces characters that are not safe in SVG text content.
func svgEscape(s string) string {
	out := make([]byte, 0, len(s)+8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '"':
			out = append(out, '&', 'q', 'u', 'o', 't', ';')
		case '\'':
			out = append(out, '&', 'a', 'p', 'o', 's', ';')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}

// Badge handles GET /api/badge/{domain}.
// Returns a shields.io-compatible JSON endpoint response.
func (h *OGHandler) Badge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	domain := vars["domain"]
	if domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}

	scan, err := h.service.GetLatestScan(r.Context(), domain)
	if err != nil {
		http.Error(w, "failed to retrieve scan", http.StatusInternalServerError)
		return
	}

	var message, color string
	if scan == nil {
		message = "not scanned"
		color = "lightgrey"
	} else {
		score := scan.CompositeScore
		message = fmt.Sprintf("%d", score)
		switch {
		case score >= 80:
			color = "brightgreen"
		case score >= 50:
			color = "yellow"
		default:
			color = "red"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	fmt.Fprintf(w, `{"schemaVersion":1,"label":"agent readiness","message":%q,"color":%q}`, message, color)
}
