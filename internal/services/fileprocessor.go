package services

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

var (
	ErrUnsupportedFileType = errors.New("tipo de archivo no soportado")
	ErrFileTooLarge        = errors.New("archivo demasiado grande")
	ErrEmptyFile           = errors.New("archivo vacio")
)

// Supported file extensions
var supportedExtensions = map[string]bool{
	".txt":  true,
	".md":   true,
	".csv":  true,
	".json": true,
	".xml":  true,
	".html": true,
	".css":  true,
	".js":   true,
	".go":   true,
	".py":   true,
	".java": true,
	".c":    true,
	".cpp":  true,
	".h":    true,
	".sql":  true,
	".yaml": true,
	".yml":  true,
	".toml": true,
	".ini":  true,
	".log":  true,
	".pdf":  true,
	".docx": true,
}

const (
	MaxFileSize    = 10 * 1024 * 1024 // 10MB
	MaxTextLength  = 50000            // Max characters to send to AI
)

type FileProcessor struct{}

type ProcessedFile struct {
	FileName    string
	FileType    string
	Content     string
	Truncated   bool
	OriginalLen int
}

func NewFileProcessor() *FileProcessor {
	return &FileProcessor{}
}

// ProcessFile extracts text content from an uploaded file
func (fp *FileProcessor) ProcessFile(file multipart.File, header *multipart.FileHeader) (*ProcessedFile, error) {
	// Check file size
	if header.Size > MaxFileSize {
		return nil, ErrFileTooLarge
	}

	if header.Size == 0 {
		return nil, ErrEmptyFile
	}

	// Get file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !supportedExtensions[ext] {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFileType, ext)
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	var textContent string

	switch ext {
	case ".pdf":
		textContent, err = fp.extractPDF(content)
	case ".docx":
		textContent, err = fp.extractDOCX(content)
	case ".csv":
		textContent, err = fp.extractCSV(content)
	case ".json":
		textContent, err = fp.extractJSON(content)
	default:
		// Plain text files
		textContent = string(content)
	}

	if err != nil {
		return nil, fmt.Errorf("error processing %s: %w", ext, err)
	}

	// Clean and prepare content
	textContent = strings.TrimSpace(textContent)
	if textContent == "" {
		return nil, ErrEmptyFile
	}

	result := &ProcessedFile{
		FileName:    header.Filename,
		FileType:    ext,
		OriginalLen: len(textContent),
		Truncated:   false,
	}

	// Truncate if too long
	if len(textContent) > MaxTextLength {
		textContent = textContent[:MaxTextLength] + "\n\n[... contenido truncado ...]"
		result.Truncated = true
	}

	result.Content = textContent
	return result, nil
}

// extractPDF extracts text from PDF files
func (fp *FileProcessor) extractPDF(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	numPages := pdfReader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		textBuilder.WriteString(text)
		textBuilder.WriteString("\n\n")
	}

	return textBuilder.String(), nil
}

// extractDOCX extracts text from Word documents
func (fp *FileProcessor) extractDOCX(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder

	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			xmlContent, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			// Simple XML text extraction (removes tags)
			text := fp.stripXMLTags(string(xmlContent))
			textBuilder.WriteString(text)
			break
		}
	}

	return textBuilder.String(), nil
}

// extractCSV formats CSV content for better readability
func (fp *FileProcessor) extractCSV(content []byte) (string, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	for i, record := range records {
		if i == 0 {
			textBuilder.WriteString("Headers: ")
		} else {
			textBuilder.WriteString(fmt.Sprintf("Row %d: ", i))
		}
		textBuilder.WriteString(strings.Join(record, " | "))
		textBuilder.WriteString("\n")
	}

	return textBuilder.String(), nil
}

// extractJSON formats JSON content for better readability
func (fp *FileProcessor) extractJSON(content []byte) (string, error) {
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		// If invalid JSON, return as plain text
		return string(content), nil
	}

	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return string(content), nil
	}

	return string(formatted), nil
}

// stripXMLTags removes XML tags from content
func (fp *FileProcessor) stripXMLTags(xml string) string {
	var result strings.Builder
	inTag := false

	for _, r := range xml {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}

	// Clean up multiple spaces
	text := result.String()
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return strings.TrimSpace(text)
}

// GetSupportedExtensions returns list of supported file types
func (fp *FileProcessor) GetSupportedExtensions() []string {
	exts := make([]string, 0, len(supportedExtensions))
	for ext := range supportedExtensions {
		exts = append(exts, ext)
	}
	return exts
}

// FormatFileContext creates a formatted context string for the AI
func (fp *FileProcessor) FormatFileContext(processed *ProcessedFile) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("\n\n--- ARCHIVO ADJUNTO: %s ---\n", processed.FileName))
	builder.WriteString(fmt.Sprintf("Tipo: %s\n", processed.FileType))
	if processed.Truncated {
		builder.WriteString(fmt.Sprintf("Nota: Contenido truncado (original: %d caracteres)\n", processed.OriginalLen))
	}
	builder.WriteString("\nContenido:\n")
	builder.WriteString(processed.Content)
	builder.WriteString("\n--- FIN ARCHIVO ---\n")

	return builder.String()
}
