// Package parser 提供多格式文件文本提取服务。
// 支持 Markdown（纯文本）、PDF、DOCX 格式的解析，
// 通过 ParserRegistry 根据文件扩展名自动分发到对应解析器。
package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Parser 定义文件解析器接口。
// filename 用于日志和错误信息，content 为原始文件字节。
type Parser interface {
	Parse(ctx context.Context, filename string, content []byte) (text string, err error)
}

// MarkdownParser 直接返回 UTF-8 文本内容。
type MarkdownParser struct{}

func (p *MarkdownParser) Parse(ctx context.Context, filename string, content []byte) (string, error) {
	if !utf8.Valid(content) {
		return "", fmt.Errorf("parser: markdown file %q is not valid UTF-8", filename)
	}
	return string(content), nil
}

// PDFParser 从 PDF 文件中提取文本。
// 当前实现使用简单的文本流提取策略：
// 扫描 PDF 内容流中的 BT...ET 文本块，提取括号中的可读文本。
// 对于生产环境，建议替换为 pdfcpu 等成熟库。
type PDFParser struct{}

func (p *PDFParser) Parse(ctx context.Context, filename string, content []byte) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("parser: PDF file %q is empty", filename)
	}

	// 简单的 PDF 文本提取：查找 BT(文本)ET 模式
	var buf bytes.Buffer
	text := extractPDFText(content)
	if strings.TrimSpace(text) == "" {
		// 如果提取不到结构化文本，尝试直接提取可读 ASCII 片段
		text = extractReadableText(content)
	}
	buf.WriteString(text)

	return buf.String(), nil
}

// extractPDFText 尝试从 PDF 内容流中提取 BT...ET 块中的文本。
func extractPDFText(data []byte) string {
	var result bytes.Buffer
	inTextBlock := false
	depth := 0
	var currentText bytes.Buffer

	for i := 0; i < len(data)-1; i++ {
		if !inTextBlock {
			// 查找 BT 标记
			if data[i] == 'B' && data[i+1] == 'T' {
				// 确认前面是空白或换行
				if i == 0 || data[i-1] == '\n' || data[i-1] == '\r' || data[i-1] == ' ' {
					inTextBlock = true
					depth = 0
					currentText.Reset()
					i++ // skip T
					continue
				}
			}
		} else {
			// 查找 ET 标记
			if data[i] == 'E' && data[i+1] == 'T' {
				if i+2 < len(data) && (data[i+2] == '\n' || data[i+2] == '\r' || data[i+2] == ' ') {
					if depth == 0 {
						trimmed := strings.TrimSpace(currentText.String())
						if trimmed != "" {
							result.WriteString(trimmed)
							result.WriteString("\n")
						}
						inTextBlock = false
						i++ // skip T
						continue
					}
				}
			}

			// 提取括号中的文本 (text)
			if data[i] == '(' {
				depth++
				if depth == 1 {
					// 开始收集括号内容
					j := i + 1
					var parenText bytes.Buffer
					for j < len(data) && depth > 0 {
						if data[j] == '\\' && j+1 < len(data) {
							// 处理转义字符
							switch data[j+1] {
							case 'n':
								parenText.WriteByte('\n')
							case 'r':
								parenText.WriteByte('\r')
							case 't':
								parenText.WriteByte('\t')
							case '\\':
								parenText.WriteByte('\\')
							case '(':
								parenText.WriteByte('(')
							case ')':
								parenText.WriteByte(')')
							default:
								parenText.WriteByte(data[j+1])
							}
							j += 2
							continue
						}
						if data[j] == '(' {
							depth++
						} else if data[j] == ')' {
							depth--
							if depth == 0 {
								break
							}
						}
						if depth > 0 {
							parenText.WriteByte(data[j])
						}
						j++
					}
					t := strings.TrimSpace(parenText.String())
					if t != "" {
						currentText.WriteString(t)
						currentText.WriteByte(' ')
					}
					i = j
					continue
				}
			}

			// 处理 <hex> 格式的文本（TJ 操作符）
			if data[i] == '<' && depth == 0 {
				j := i + 1
				var hexBuf bytes.Buffer
				for j < len(data) && data[j] != '>' {
					hexBuf.WriteByte(data[j])
					j++
				}
				if j < len(data) && data[j] == '>' {
					hexStr := hexBuf.String()
					decoded := decodePDFHexString(hexStr)
					if decoded != "" {
						currentText.WriteString(decoded)
					}
					i = j
					continue
				}
			}
		}
	}

	return result.String()
}

// decodePDFHexString 解码 PDF 十六进制字符串。
func decodePDFHexString(hex string) string {
	var result bytes.Buffer
	for i := 0; i+1 < len(hex); i += 2 {
		var b byte
		_, err := fmt.Sscanf(hex[i:i+2], "%02x", &b)
		if err == nil && b >= 0x20 && b < 0x7f {
			result.WriteByte(b)
		}
	}
	return result.String()
}

// extractReadableText 从二进制数据中提取连续的可读 ASCII 文本片段。
func extractReadableText(data []byte) string {
	var result bytes.Buffer
	var current bytes.Buffer

	for _, b := range data {
		if b >= 0x20 && b < 0x7f {
			current.WriteByte(b)
		} else {
			if current.Len() > 4 { // 至少 4 个连续可读字符
				result.WriteString(strings.TrimSpace(current.String()))
				result.WriteString("\n")
			}
			current.Reset()
		}
	}
	if current.Len() > 4 {
		result.WriteString(strings.TrimSpace(current.String()))
		result.WriteString("\n")
	}
	return result.String()
}

// DocxParser 从 DOCX 文件中提取文本。
// DOCX 本质是 ZIP 包，内含 word/document.xml。
// 当前实现使用简单的 XML 标签剥离策略提取 <w:t> 标签中的文本。
// 对于生产环境，建议替换为专门的 DOCX 解析库。
type DocxParser struct{}

func (p *DocxParser) Parse(ctx context.Context, filename string, content []byte) (string, error) {
	if len(content) == 0 {
		return "", fmt.Errorf("parser: DOCX file %q is empty", filename)
	}

	// 尝试从 ZIP 中提取 document.xml 并解析
	text, err := extractDocxText(content)
	if err != nil {
		// 如果 ZIP 解析失败，尝试直接作为 XML 解析
		text = extractXMLText(content)
	}
	return text, nil
}

// extractDocxText 尝试解压 DOCX（ZIP）并提取 word/document.xml 中的文本。
func extractDocxText(data []byte) (string, error) {
	// 检查 ZIP 签名
	if len(data) < 4 || data[0] != 0x50 || data[1] != 0x4b {
		return "", fmt.Errorf("not a valid ZIP/DOCX file")
	}

	// 使用标准库 archive/zip 解压
	readerAt := bytes.NewReader(data)
	zipReader, err := openZipReader(readerAt, int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			var buf bytes.Buffer
			buf.ReadFrom(rc)
			return extractXMLText(buf.Bytes()), nil
		}
	}
	return "", fmt.Errorf("word/document.xml not found in DOCX")
}

// openZipReader 打开 ZIP 读取器。
func openZipReader(r *bytes.Reader, size int64) (*zip.Reader, error) {
	return zip.NewReader(r, size)
}

// extractXMLText 从 XML 数据中提取 <w:t> 标签内的文本。
func extractXMLText(data []byte) string {
	var result bytes.Buffer
	content := string(data)

	for {
		start := strings.Index(content, "<w:t")
		if start == -1 {
			break
		}
		// 找到标签结束位置
		tagEnd := strings.Index(content[start:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += start + 1

		// 找到闭合标签
		end := strings.Index(content[tagEnd:], "</w:t>")
		if end == -1 {
			break
		}
		end += tagEnd

		text := content[tagEnd:end]
		text = decodeXMLEntities(text)
		if strings.TrimSpace(text) != "" {
			result.WriteString(text)
			result.WriteString(" ")
		}
		content = content[end+5:]
	}

	return strings.TrimSpace(result.String())
}

// decodeXMLEntities 解码常见 XML 实体。
func decodeXMLEntities(s string) string {
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	return s
}

// ParserRegistry 根据文件扩展名分发到对应的解析器。
type ParserRegistry struct {
	parsers map[string]Parser
}

// NewParserRegistry 创建解析器注册表并注册内置解析器。
func NewParserRegistry() *ParserRegistry {
	r := &ParserRegistry{
		parsers: make(map[string]Parser),
	}
	r.Register(".md", &MarkdownParser{})
	r.Register(".markdown", &MarkdownParser{})
	r.Register(".txt", &MarkdownParser{})
	r.Register(".pdf", &PDFParser{})
	r.Register(".docx", &DocxParser{})
	return r
}

// Register 注册扩展名对应的解析器（ext 必须包含点号，如 ".pdf"）。
func (r *ParserRegistry) Register(ext string, p Parser) {
	r.parsers[strings.ToLower(ext)] = p
}

// Resolve 根据文件扩展名查找解析器。
func (r *ParserRegistry) Resolve(ext string) (Parser, error) {
	p, ok := r.parsers[strings.ToLower(ext)]
	if !ok {
		return nil, fmt.Errorf("parser: unsupported file extension %q", ext)
	}
	return p, nil
}

// Parse 根据文件扩展名自动选择解析器并解析内容。
func (r *ParserRegistry) Parse(ctx context.Context, filename string, content []byte) (string, error) {
	ext := strings.ToLower(getExt(filename))
	p, err := r.Resolve(ext)
	if err != nil {
		return "", err
	}
	return p.Parse(ctx, filename, content)
}

// getExt 获取文件扩展名（包含点号）。
func getExt(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return ""
	}
	return filename[idx:]
}
