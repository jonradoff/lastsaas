package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"lastsaas/internal/version"
)

// OpenAPI JSON types (subset of OpenAPI 3.0 spec sufficient for our API)

type openAPISpec struct {
	OpenAPI    string                     `json:"openapi"`
	Info       openAPIInfo                `json:"info"`
	Servers    []openAPIServer            `json:"servers"`
	Paths      map[string]openAPIPathItem `json:"paths"`
	Components openAPIComponents          `json:"components"`
}

type openAPIInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type openAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type openAPIPathItem map[string]*openAPIOperation // method -> operation

type openAPIOperation struct {
	Summary     string              `json:"summary"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags"`
	Security    []openAPISecurity   `json:"security,omitempty"`
	Parameters  []openAPIParameter  `json:"parameters,omitempty"`
	RequestBody *openAPIRequestBody `json:"requestBody,omitempty"`
	Responses   openAPIResponses    `json:"responses"`
}

type openAPISecurity map[string][]string

type openAPIParameter struct {
	Name        string          `json:"name"`
	In          string          `json:"in"`
	Required    bool            `json:"required"`
	Description string          `json:"description"`
	Schema      openAPISchema   `json:"schema"`
}

type openAPISchema struct {
	Type    string `json:"type"`
	Example any    `json:"example,omitempty"`
}

type openAPIRequestBody struct {
	Required bool                          `json:"required"`
	Content  map[string]openAPIMediaType   `json:"content"`
}

type openAPIMediaType struct {
	Schema  openAPISchemaRef `json:"schema,omitempty"`
	Example any              `json:"example,omitempty"`
}

type openAPISchemaRef struct {
	Type       string                   `json:"type,omitempty"`
	Properties map[string]openAPISchema `json:"properties,omitempty"`
}

type openAPIResponses map[string]openAPIResponse

type openAPIResponse struct {
	Description string                       `json:"description"`
	Content     map[string]openAPIMediaType   `json:"content,omitempty"`
}

type openAPIComponents struct {
	SecuritySchemes map[string]openAPISecurityScheme `json:"securitySchemes"`
}

type openAPISecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
	Description  string `json:"description,omitempty"`
}

// DocsOpenAPI handles GET /api/docs/openapi.json — serves an auto-generated OpenAPI 3.0 spec.
func DocsOpenAPI(w http.ResponseWriter, r *http.Request) {
	sections := apiReference()

	spec := openAPISpec{
		OpenAPI: "3.0.3",
		Info: openAPIInfo{
			Title:   "MCPLens API",
			Version: version.Current,
		},
		Servers: []openAPIServer{
			{URL: "/", Description: "Current server"},
		},
		Paths: make(map[string]openAPIPathItem),
		Components: openAPIComponents{
			SecuritySchemes: map[string]openAPISecurityScheme{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
					Description:  "JWT access token or API key (lsk_...)",
				},
				"tenantId": {
					Type:        "apiKey",
					Name:        "X-Tenant-ID",
					In:          "header",
					Description: "Tenant context for multi-tenant routes",
				},
			},
		},
	}

	for _, section := range sections {
		tag := section.Title
		for _, ep := range section.Endpoints {
			method := strings.ToLower(ep.Method)
			path := ep.Path

			op := &openAPIOperation{
				Summary:     ep.Summary,
				Description: stripHTML(ep.Detail),
				Tags:        []string{tag},
				Responses:   make(openAPIResponses),
			}

			// Security
			switch ep.Auth {
			case "jwt":
				op.Security = []openAPISecurity{{"bearerAuth": {}}}
			case "jwt+tenant":
				op.Security = []openAPISecurity{{"bearerAuth": {}, "tenantId": {}}}
			case "admin", "owner":
				op.Security = []openAPISecurity{{"bearerAuth": {}, "tenantId": {}}}
			}

			// Path parameters (extract {param} from path)
			for _, p := range ep.Params {
				in := "query"
				if strings.Contains(path, "{"+p.Name+"}") {
					in = "path"
				}
				op.Parameters = append(op.Parameters, openAPIParameter{
					Name:        p.Name,
					In:          in,
					Required:    p.Required,
					Description: p.Desc,
					Schema:      openAPISchema{Type: mapParamType(p.Type)},
				})
			}

			// Request body
			if ep.Body != "" {
				var bodyExample any
				json.Unmarshal([]byte(ep.Body), &bodyExample)
				op.RequestBody = &openAPIRequestBody{
					Required: true,
					Content: map[string]openAPIMediaType{
						"application/json": {
							Example: bodyExample,
						},
					},
				}
			}

			// Response
			if ep.Response != "" {
				var respExample any
				json.Unmarshal([]byte(ep.Response), &respExample)
				op.Responses["200"] = openAPIResponse{
					Description: "Success",
					Content: map[string]openAPIMediaType{
						"application/json": {
							Example: respExample,
						},
					},
				}
			} else {
				op.Responses["200"] = openAPIResponse{Description: "Success"}
			}

			// Error responses
			op.Responses["400"] = openAPIResponse{Description: "Bad request"}
			if ep.Auth != "none" && ep.Auth != "stripe" {
				op.Responses["401"] = openAPIResponse{Description: "Unauthorized"}
			}

			// Add to paths
			if _, ok := spec.Paths[path]; !ok {
				spec.Paths[path] = make(openAPIPathItem)
			}
			spec.Paths[path][method] = op
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(spec)
}

func mapParamType(t string) string {
	switch strings.ToLower(t) {
	case "int", "integer":
		return "integer"
	case "objectid":
		return "string"
	default:
		return "string"
	}
}
