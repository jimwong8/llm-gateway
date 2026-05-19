package httpserver

import (
	"encoding/json"
	"net/http"
)

type openAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type openAPIParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Schema      any    `json:"schema,omitempty"`
}

type openAPIResponse struct {
	Description string         `json:"description"`
	Content     map[string]any `json:"content,omitempty"`
}

type openAPIPathItem struct {
	Summary     string                    `json:"summary,omitempty"`
	Description string                    `json:"description,omitempty"`
	Parameters  []openAPIParameter        `json:"parameters,omitempty"`
	RequestBody map[string]any            `json:"requestBody,omitempty"`
	Responses   map[string]openAPIResponse `json:"responses,omitempty"`
	Security    []map[string][]string     `json:"security,omitempty"`
}

type openAPISpec struct {
	OpenAPI    string                    `json:"openapi"`
	Info       openAPIInfo               `json:"info"`
	Paths      map[string]map[string]any `json:"paths"`
	Components map[string]any            `json:"components,omitempty"`
	Security   []map[string][]string     `json:"security,omitempty"`
	Servers    []map[string]string       `json:"servers,omitempty"`
}

func (s *Server) mountOpenAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/openapi.json", s.handleOpenAPI)
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec := generateOpenAPISpec()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

func generateOpenAPISpec() openAPISpec {
	spec := openAPISpec{
		OpenAPI: "3.0.3",
		Info: openAPIInfo{
			Title:       "LLM Gateway API",
			Description: "LLM Gateway management and chat API",
			Version:     "1.0.0",
		},
		Paths: map[string]map[string]any{},
		Components: map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "JWT token obtained from /api/auth/login",
				},
			},
		},
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
		Servers: []map[string]string{
			{"url": "/", "description": "Current server"},
		},
	}

	security := []map[string][]string{{"bearerAuth": {}}}

	// ── /api/memory/presets ─────────────────────────────────
	spec.Paths["/api/memory/presets"] = map[string]any{
		"get": map[string]any{
			"summary":     "List prompt presets",
			"description": "List all prompt presets for the authenticated user",
			"security":    security,
			"responses": map[string]any{
				"200": map[string]any{
					"description": "List of presets",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"data": map[string]any{
										"type":  "array",
										"items": map[string]any{"$ref": "#/components/schemas/PromptPreset"},
									},
								},
							},
						},
					},
				},
				"401": unauthorizedResponse(),
			},
		},
		"post": map[string]any{
			"summary":     "Create prompt preset",
			"description": "Create a new prompt preset template",
			"security":    security,
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"$ref": "#/components/schemas/PromptPresetInput",
						},
					},
				},
			},
			"responses": map[string]any{
				"201": map[string]any{
					"description": "Preset created",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/PromptPreset"},
						},
					},
				},
				"400": badRequestResponse(),
				"401": unauthorizedResponse(),
			},
		},
	}

	// ── /api/memory/presets/{id} ────────────────────────────
	spec.Paths["/api/memory/presets/{id}"] = map[string]any{
		"get": map[string]any{
			"summary":  "Get preset by ID",
			"security": security,
			"parameters": []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Preset details"},
				"404": notFoundResponse(),
			},
		},
		"put": map[string]any{
			"summary":  "Update preset",
			"security": security,
			"parameters": []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/PromptPresetInput"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Updated preset"},
				"400": badRequestResponse(),
				"404": notFoundResponse(),
			},
		},
		"delete": map[string]any{
			"summary":  "Delete preset",
			"security": security,
			"parameters": []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Deleted"},
				"404": notFoundResponse(),
			},
		},
	}

	// ── /api/memory/masks ───────────────────────────────────
	spec.Paths["/api/memory/masks"] = map[string]any{
		"get": map[string]any{
			"summary":  "List mask rules",
			"security": security,
			"responses": map[string]any{
				"200": map[string]any{
					"description": "List of mask rules",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"data": map[string]any{
										"type":  "array",
										"items": map[string]any{"$ref": "#/components/schemas/MaskRule"},
									},
								},
							},
						},
					},
				},
			},
		},
		"post": map[string]any{
			"summary":  "Create mask rule",
			"security": security,
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/MaskRuleInput"},
					},
				},
			},
			"responses": map[string]any{
				"201": map[string]any{"description": "Mask rule created"},
				"400": badRequestResponse(),
			},
		},
	}

	// ── /api/memory/masks/{id} ──────────────────────────────
	spec.Paths["/api/memory/masks/{id}"] = map[string]any{
		"put": map[string]any{
			"summary":  "Update mask rule",
			"security": security,
			"parameters": []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/MaskRuleInput"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Updated"},
				"400": badRequestResponse(),
				"404": notFoundResponse(),
			},
		},
		"delete": map[string]any{
			"summary":  "Delete mask rule",
			"security": security,
			"parameters": []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
			"responses": map[string]any{
				"200": map[string]any{"description": "Deleted"},
				"404": notFoundResponse(),
			},
		},
	}

	// ── /api/files/parse ────────────────────────────────────
	spec.Paths["/api/files/parse"] = map[string]any{
		"post": map[string]any{
			"summary":     "Parse file content",
			"description": "Upload and parse a file (PDF, Word, Markdown) to extract text",
			"security":    security,
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"multipart/form-data": map[string]any{
						"schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"file": map[string]any{
									"type":        "string",
									"format":      "binary",
									"description": "File to parse (max 10MB)",
								},
							},
							"required": []string{"file"},
						},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Parsed content",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"text":     map[string]any{"type": "string"},
									"filename": map[string]any{"type": "string"},
									"size":     map[string]any{"type": "integer"},
								},
							},
						},
					},
				},
				"400": badRequestResponse(),
				"401": unauthorizedResponse(),
			},
		},
	}

	// ── /api/ws/chat ────────────────────────────────────────
	spec.Paths["/api/ws/chat"] = map[string]any{
		"get": map[string]any{
			"summary":     "WebSocket chat endpoint",
			"description": "Upgrade to WebSocket for real-time chat. Send {type:'ping'} for heartbeat, {type:'chat', content:'...'} for messages.",
			"security":    security,
			"responses": map[string]any{
				"101": map[string]any{"description": "WebSocket upgrade"},
				"401": unauthorizedResponse(),
			},
		},
	}

	// ── /api/openapi.json ───────────────────────────────────
	spec.Paths["/api/openapi.json"] = map[string]any{
		"get": map[string]any{
			"summary": "OpenAPI specification",
			"responses": map[string]any{
				"200": map[string]any{
					"description": "OpenAPI 3.0 spec",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"type": "object"},
						},
					},
				},
			},
		},
	}

	// ── Components / Schemas ────────────────────────────────
	spec.Components["schemas"] = map[string]any{
		"PromptPreset": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":          map[string]any{"type": "integer"},
				"user_id":     map[string]any{"type": "integer"},
				"name":        map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"template":    map[string]any{"type": "string"},
				"variables":   map[string]any{"type": "string"},
				"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"is_public":   map[string]any{"type": "boolean"},
				"created_at":  map[string]any{"type": "string", "format": "date-time"},
				"updated_at":  map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"PromptPresetInput": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
				"template":    map[string]any{"type": "string"},
				"variables":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"tags":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"is_public":   map[string]any{"type": "boolean"},
			},
			"required": []string{"name", "template"},
		},
		"MaskRule": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":         map[string]any{"type": "integer"},
				"user_id":    map[string]any{"type": "integer"},
				"name":       map[string]any{"type": "string"},
				"pattern":    map[string]any{"type": "string"},
				"replace":    map[string]any{"type": "string"},
				"is_active":  map[string]any{"type": "boolean"},
				"created_at": map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"MaskRuleInput": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string"},
				"pattern": map[string]any{"type": "string"},
				"replace": map[string]any{"type": "string"},
				"enabled": map[string]any{"type": "boolean"},
			},
			"required": []string{"name", "pattern"},
		},
		"Error": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
						"type":    map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	return spec
}

func unauthorizedResponse() map[string]any {
	return map[string]any{
		"description": "Unauthorized",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}

func badRequestResponse() map[string]any {
	return map[string]any{
		"description": "Bad request",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}

func notFoundResponse() map[string]any {
	return map[string]any{
		"description": "Not found",
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}


