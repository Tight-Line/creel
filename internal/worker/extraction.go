package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ledongthuc/pdf"
	"golang.org/x/net/html"

	"github.com/Tight-Line/creel/internal/store"
)

// ExtractionWorker extracts text from uploaded document content.
type ExtractionWorker struct {
	docStore *store.DocumentStore
	jobStore *store.JobStore
}

// NewExtractionWorker creates a new extraction worker.
func NewExtractionWorker(docStore *store.DocumentStore, jobStore *store.JobStore) *ExtractionWorker {
	return &ExtractionWorker{docStore: docStore, jobStore: jobStore}
}

// Type returns the job type this worker handles.
func (w *ExtractionWorker) Type() string {
	return "extraction"
}

// Process extracts text from the document's raw content.
func (w *ExtractionWorker) Process(ctx context.Context, job *store.ProcessingJob) error {
	if err := w.docStore.UpdateStatus(ctx, job.DocumentID, "processing"); err != nil {
		return fmt.Errorf("setting document status to processing: %w", err)
	}

	content, err := w.docStore.GetContent(ctx, job.DocumentID)
	if err != nil {
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after content error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("getting document content: %w", err)
	}

	text, err := ExtractText(content.RawContent, content.ContentType)
	if err != nil {
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after extraction error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("extracting text: %w", err)
	}

	if err := w.docStore.SaveExtractedText(ctx, job.DocumentID, text); err != nil {
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after save error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("saving extracted text: %w", err)
	}

	// Create the next pipeline job: chunking.
	if _, err := w.jobStore.Create(ctx, job.DocumentID, "chunking"); err != nil {
		// coverage:ignore - requires DB failure after successful claim
		if setErr := w.docStore.UpdateStatus(ctx, job.DocumentID, "failed"); setErr != nil {
			return fmt.Errorf("setting document status to failed after job creation error: %w (original: %v)", setErr, err)
		}
		return fmt.Errorf("creating chunking job: %w", err)
	}

	return nil
}

// ExtractText extracts text content from raw bytes based on the content type.
// Supported types: text/plain (passthrough), text/html (tag stripping).
// If contentType is empty, the type is detected from the data using
// http.DetectContentType.
func ExtractText(data []byte, contentType string) (string, error) {
	// Normalize content type by taking only the media type portion.
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}

	// Auto-detect when no content type is provided.
	if ct == "" {
		detected := http.DetectContentType(data)
		ct = strings.ToLower(detected)
		if idx := strings.Index(ct, ";"); idx >= 0 {
			ct = strings.TrimSpace(ct[:idx])
		}
	}

	switch ct {
	case "text/plain":
		return string(data), nil
	case "text/html", "application/xhtml+xml":
		return extractHTML(data)
	case "application/pdf":
		return extractPDF(data)
	default:
		return "", fmt.Errorf("unsupported content type: %s", ct)
	}
}

// extractPDF extracts text content from PDF bytes.
func extractPDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening PDF: %w", err)
	}

	plainText, err := reader.GetPlainText()
	// coverage:ignore - requires malformed internal PDF structure
	if err != nil {
		return "", fmt.Errorf("extracting PDF text: %w", err)
	}

	text, err := io.ReadAll(plainText)
	// coverage:ignore - requires IO failure on in-memory reader
	if err != nil {
		return "", fmt.Errorf("reading PDF text: %w", err)
	}

	return strings.TrimSpace(string(text)), nil
}

// extractHTML parses HTML and extracts visible text content.
func extractHTML(data []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(data))
	// coverage:ignore - Go's html.Parse is extremely lenient and essentially never errors
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	var buf strings.Builder
	extractHTMLText(doc, &buf)
	return strings.TrimSpace(buf.String()), nil
}

// extractHTMLText recursively walks the HTML node tree and writes text content.
func extractHTMLText(n *html.Node, w io.Writer) {
	// Skip script and style elements.
	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
		return
	}
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			_, _ = fmt.Fprintf(w, "%s ", text)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractHTMLText(c, w)
	}
}
