package parser

import (
	"context"
	"strings"
	"testing"
)

func TestMarkdownParser_Parse(t *testing.T) {
	ctx := context.Background()
	p := &MarkdownParser{}

	tests := []struct {
		name     string
		filename string
		content  string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple markdown",
			filename: "test.md",
			content:  "# Hello\n\nThis is **bold** text.",
			want:     "# Hello\n\nThis is **bold** text.",
		},
		{
			name:     "empty content",
			filename: "empty.md",
			content:  "",
			want:     "",
		},
		{
			name:     "chinese content",
			filename: "chinese.md",
			content:  "# 标题\n\n这是一段中文内容。",
			want:     "# 标题\n\n这是一段中文内容。",
		},
		{
			name:     "code block",
			filename: "code.md",
			content:  "```go\nfunc main() {}\n```",
			want:     "```go\nfunc main() {}\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Parse(ctx, tt.filename, []byte(tt.content))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Parse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkdownParser_InvalidUTF8(t *testing.T) {
	ctx := context.Background()
	p := &MarkdownParser{}

	// 构造无效 UTF-8 字节
	invalid := []byte{0xff, 0xfe, 0x00}
	_, err := p.Parse(ctx, "bad.md", invalid)
	if err == nil {
		t.Error("expected error for invalid UTF-8, got nil")
	}
}

func TestParserRegistry_Resolve(t *testing.T) {
	r := NewParserRegistry()

	tests := []struct {
		ext     string
		wantErr bool
	}{
		{".md", false},
		{".markdown", false},
		{".txt", false},
		{".pdf", false},
		{".docx", false},
		{".MD", false},   // 大小写不敏感
		{".PDF", false},
		{".doc", true},   // 不支持
		{".xlsx", true},  // 不支持
		{"", true},       // 空扩展名
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			_, err := r.Resolve(tt.ext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve(%q) error = %v, wantErr %v", tt.ext, err, tt.wantErr)
			}
		})
	}
}

func TestParserRegistry_Parse(t *testing.T) {
	ctx := context.Background()
	r := NewParserRegistry()

	got, err := r.Parse(ctx, "test.md", []byte("# Title\n\nContent here."))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got != "# Title\n\nContent here." {
		t.Errorf("Parse() = %q, want %q", got, "# Title\n\nContent here.")
	}
}

func TestParserRegistry_ParseUnsupported(t *testing.T) {
	ctx := context.Background()
	r := NewParserRegistry()

	_, err := r.Parse(ctx, "test.xlsx", []byte("data"))
	if err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got %q", err.Error())
	}
}

func TestGetExt(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"test.md", ".md"},
		{"path/to/file.pdf", ".pdf"},
		{"noext", ""},
		{".hidden", ".hidden"},
		{"file.DOCX", ".DOCX"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := getExt(tt.filename)
			if got != tt.want {
				t.Errorf("getExt(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestPDFParser_Empty(t *testing.T) {
	ctx := context.Background()
	p := &PDFParser{}

	_, err := p.Parse(ctx, "empty.pdf", []byte{})
	if err == nil {
		t.Error("expected error for empty PDF, got nil")
	}
}

func TestDocxParser_Empty(t *testing.T) {
	ctx := context.Background()
	p := &DocxParser{}

	_, err := p.Parse(ctx, "empty.docx", []byte{})
	if err == nil {
		t.Error("expected error for empty DOCX, got nil")
	}
}
