package httpserver

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"llm-gateway/gateway/internal/audit"
	"llm-gateway/gateway/internal/parser"
)

// parserRegistry 全局解析器注册表，延迟初始化。
var fileParserRegistry = parser.NewParserRegistry()

// mountFileParserRoutes 注册文件解析相关路由。
func (s *Server) mountFileParserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/files/parse", s.requireUser(s.handleFileParse))
}

// handleFileParse 处理文件上传并返回提取的文本。
// 支持 multipart form 上传，字段名为 "file"。
// 返回 JSON: {text, filename, size}
func (s *Server) handleFileParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, r)
		return
	}

	claims := getUserClaims(r.Context())
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"message": "not authenticated", "type": "authentication_error"},
		})
		return
	}
	// 限制上传大小：10MB
	const maxUploadSize = 10 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		badRequest(w, fmt.Sprintf("failed to parse form or file too large (max %d MB): %s", maxUploadSize>>20, err.Error()))
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, "missing 'file' field in multipart form")
		return
	}
	defer file.Close()

	filename := strings.TrimSpace(header.Filename)
	if filename == "" {
		badRequest(w, "empty filename")
		return
	}

	// 读取文件内容
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		internalError(w, fmt.Errorf("failed to read file: %w", err))
		return
	}
	content := buf.Bytes()

	// 解析文件
	text, err := fileParserRegistry.Parse(r.Context(), filename, content)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unsupported") {
			badRequest(w, msg)
			return
		}
		internalError(w, err)
		return
	}

	s.writeAuditAsync(audit.Event{
		RequestPayload: map[string]any{
			"action":    "file_parsed",
			"filename":  filename,
			"size":      len(content),
			"actor_id":  fmt.Sprintf("%d", claims.UserID),
		},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"text":     text,
		"filename": filename,
		"size":     len(content),
	})
}
