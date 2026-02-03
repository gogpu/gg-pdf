package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/recording"
)

func TestBackendRegistration(t *testing.T) {
	// Test that the PDF backend is registered
	if !recording.IsRegistered("pdf") {
		t.Error("PDF backend should be registered")
	}

	// Test that we can create a backend
	backend, err := recording.NewBackend("pdf")
	if err != nil {
		t.Fatalf("Failed to create PDF backend: %v", err)
	}

	if backend == nil {
		t.Error("Backend should not be nil")
	}
}

func TestBackendInterfaces(t *testing.T) {
	backend := NewBackend()

	// Test Backend interface
	var _ recording.Backend = backend

	// Test WriterBackend interface
	var _ recording.WriterBackend = backend

	// Test FileBackend interface
	var _ recording.FileBackend = backend
}

func TestBackendLifecycle(t *testing.T) {
	backend := NewBackend()

	// Test Begin
	err := backend.Begin(800, 600)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Test End
	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendSaveRestore(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(800, 600)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Save state
	backend.Save()

	// Set transform
	backend.SetTransform(recording.Translate(100, 100))

	// Restore should work without error
	backend.Restore()

	// Multiple saves and restores
	backend.Save()
	backend.Save()
	backend.Restore()
	backend.Restore()

	// Restore with empty stack should be no-op
	backend.Restore() // Should not panic

	_ = backend.End()
}

func TestBackendFillPath(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Create a simple rectangle path
	path := gg.NewPath()
	path.Rectangle(50, 50, 100, 80)

	// Create a solid brush
	brush := recording.NewSolidBrush(gg.RGBA{R: 255, G: 0, B: 0, A: 255})

	// Fill the path
	backend.FillPath(path, brush, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Write to buffer to verify no errors
	var buf bytes.Buffer
	_, err = backend.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("PDF output should not be empty")
	}
}

func TestBackendStrokePath(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Create a triangle path
	path := gg.NewPath()
	path.MoveTo(100, 50)
	path.LineTo(150, 150)
	path.LineTo(50, 150)
	path.Close()

	// Create brush and stroke
	brush := recording.NewSolidBrush(gg.RGBA{R: 0, G: 0, B: 255, A: 255})
	stroke := recording.Stroke{
		Width:      2.0,
		Cap:        recording.LineCapRound,
		Join:       recording.LineJoinRound,
		MiterLimit: 4.0,
	}

	backend.StrokePath(path, brush, stroke)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	var buf bytes.Buffer
	_, err = backend.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

func TestBackendFillRect(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	rect := recording.NewRect(20, 20, 160, 120)
	brush := recording.NewSolidBrush(gg.RGBA{R: 0, G: 255, B: 0, A: 200})

	backend.FillRect(rect, brush)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendLinearGradient(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	path := gg.NewPath()
	path.Rectangle(50, 50, 200, 150)

	grad := recording.NewLinearGradientBrush(50, 50, 250, 200).
		AddColorStop(0, gg.RGBA{R: 255, G: 0, B: 0, A: 255}).
		AddColorStop(0.5, gg.RGBA{R: 0, G: 255, B: 0, A: 255}).
		AddColorStop(1, gg.RGBA{R: 0, G: 0, B: 255, A: 255})

	backend.FillPath(path, grad, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendRadialGradient(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	path := gg.NewPath()
	path.Circle(200, 150, 100)

	grad := recording.NewRadialGradientBrush(200, 150, 0, 100).
		AddColorStop(0, gg.RGBA{R: 255, G: 255, B: 0, A: 255}).
		AddColorStop(1, gg.RGBA{R: 255, G: 0, B: 0, A: 255})

	backend.FillPath(path, grad, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendDashedStroke(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	path := gg.NewPath()
	path.MoveTo(50, 150)
	path.LineTo(350, 150)

	brush := recording.NewSolidBrush(gg.RGBA{R: 0, G: 0, B: 0, A: 255})
	stroke := recording.Stroke{
		Width:       3.0,
		Cap:         recording.LineCapButt,
		Join:        recording.LineJoinMiter,
		MiterLimit:  4.0,
		DashPattern: []float64{10, 5, 3, 5},
		DashOffset:  0,
	}

	backend.StrokePath(path, brush, stroke)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendClip(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Create clip path (circle)
	clipPath := gg.NewPath()
	clipPath.Circle(200, 150, 80)

	// Set clip
	backend.SetClip(clipPath, recording.FillRuleNonZero)

	// Draw rectangle (should be clipped to circle)
	rect := gg.NewPath()
	rect.Rectangle(100, 50, 200, 200)

	brush := recording.NewSolidBrush(gg.RGBA{R: 255, G: 100, B: 100, A: 255})
	backend.FillPath(rect, brush, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendTransform(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Test various transforms
	transforms := []recording.Matrix{
		recording.Translate(100, 50),
		recording.Scale(2, 2),
		recording.Rotate(0.5), // ~28.6 degrees
		recording.Identity(),
	}

	for _, transform := range transforms {
		backend.Save()
		backend.SetTransform(transform)

		path := gg.NewPath()
		path.Rectangle(10, 10, 30, 30)

		brush := recording.NewSolidBrush(gg.RGBA{R: 100, G: 100, B: 255, A: 255})
		backend.FillPath(path, brush, recording.FillRuleNonZero)

		backend.Restore()
	}

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestBackendSaveToFile(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Draw something
	path := gg.NewPath()
	path.Rectangle(50, 50, 300, 200)
	brush := recording.NewSolidBrush(gg.RGBA{R: 100, G: 150, B: 200, A: 255})
	backend.FillPath(path, brush, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	// Save to temp file
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "test.pdf")

	err = backend.SaveToFile(pdfPath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file exists and has content
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("PDF file should not be empty")
	}

	// Verify PDF header
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Error("Output file should start with PDF header")
	}
}

func TestDocument(t *testing.T) {
	doc := NewDocument()

	// Create first page
	p1 := doc.NewPage(400, 300)
	// Note: Begin is called internally by NewPage, calling it again would
	// reset state which we don't want. Skip the explicit Begin call.

	path := gg.NewPath()
	path.Rectangle(50, 50, 100, 80)
	brush := recording.NewSolidBrush(gg.RGBA{R: 255, G: 0, B: 0, A: 255})
	p1.FillPath(path, brush, recording.FillRuleNonZero)

	// Create second page
	p2 := doc.NewPage(300, 400)
	path2 := gg.NewPath()
	path2.Circle(150, 200, 50)
	brush2 := recording.NewSolidBrush(gg.RGBA{R: 0, G: 0, B: 255, A: 255})
	p2.FillPath(path2, brush2, recording.FillRuleNonZero)

	// Verify page count
	if doc.PageCount() != 2 {
		t.Errorf("Expected 2 pages, got %d", doc.PageCount())
	}

	// Save document
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "multi_page.pdf")

	err := doc.SaveToFile(pdfPath)
	if err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	// Verify file
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("PDF file should not be empty")
	}
}

func TestDocumentMetadata(t *testing.T) {
	doc := NewDocument()
	doc.SetTitle("Test Document")
	doc.SetAuthor("Test Author")
	doc.SetSubject("Test Subject")
	doc.SetKeywords("test, pdf, gg")

	// Create a page
	_ = doc.NewPage(400, 300)

	// Write to buffer
	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Metadata should be in the PDF (as objects), but we can't easily verify
	// Just check that the PDF is valid
	if !bytes.Contains(buf.Bytes(), []byte("%PDF-")) {
		t.Error("Output should be a valid PDF")
	}
}

func TestFillRuleTranslation(t *testing.T) {
	backend := NewBackend()

	// Test NonZero rule
	rule := backend.translateFillRule(recording.FillRuleNonZero)
	if rule != 0 { // FillRuleNonZero = 0 in gxpdf
		t.Errorf("Expected NonZero fill rule")
	}

	// Test EvenOdd rule
	rule = backend.translateFillRule(recording.FillRuleEvenOdd)
	if rule != 1 { // FillRuleEvenOdd = 1 in gxpdf
		t.Errorf("Expected EvenOdd fill rule")
	}
}

func TestLineCapTranslation(t *testing.T) {
	backend := NewBackend()

	tests := []struct {
		input    recording.LineCap
		expected int
	}{
		{recording.LineCapButt, 0},
		{recording.LineCapRound, 1},
		{recording.LineCapSquare, 2},
	}

	for _, tt := range tests {
		result := backend.translateLineCap(tt.input)
		if int(result) != tt.expected {
			t.Errorf("translateLineCap(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestLineJoinTranslation(t *testing.T) {
	backend := NewBackend()

	tests := []struct {
		input    recording.LineJoin
		expected int
	}{
		{recording.LineJoinMiter, 0},
		{recording.LineJoinRound, 1},
		{recording.LineJoinBevel, 2},
	}

	for _, tt := range tests {
		result := backend.translateLineJoin(tt.input)
		if int(result) != tt.expected {
			t.Errorf("translateLineJoin(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestMatrixTranslation(t *testing.T) {
	backend := NewBackend()

	// Test identity matrix
	identity := recording.Identity()
	transform := backend.matrixToTransform(identity)

	if transform.A != 1 || transform.D != 1 {
		t.Error("Identity matrix should have A=1, D=1")
	}
	if transform.E != 0 || transform.F != 0 {
		t.Error("Identity matrix should have E=0, F=0")
	}
}

func TestSweepGradientFallback(t *testing.T) {
	backend := NewBackend()
	err := backend.Begin(400, 300)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	path := gg.NewPath()
	path.Circle(200, 150, 100)

	// Sweep gradients are not supported in PDF, should fallback to first stop color
	grad := recording.NewSweepGradientBrush(200, 150, 0).
		AddColorStop(0, gg.RGBA{R: 255, G: 0, B: 0, A: 255}).
		AddColorStop(1, gg.RGBA{R: 0, G: 255, B: 0, A: 255})

	backend.FillPath(path, grad, recording.FillRuleNonZero)

	err = backend.End()
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func BenchmarkBackendFillPath(b *testing.B) {
	backend := NewBackend()
	_ = backend.Begin(800, 600)

	path := gg.NewPath()
	path.Rectangle(50, 50, 100, 80)
	brush := recording.NewSolidBrush(gg.RGBA{R: 255, G: 0, B: 0, A: 255})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.FillPath(path, brush, recording.FillRuleNonZero)
	}
}

func BenchmarkBackendStrokePath(b *testing.B) {
	backend := NewBackend()
	_ = backend.Begin(800, 600)

	path := gg.NewPath()
	path.MoveTo(0, 0)
	path.LineTo(100, 100)
	path.LineTo(200, 0)

	brush := recording.NewSolidBrush(gg.RGBA{R: 0, G: 0, B: 0, A: 255})
	stroke := recording.DefaultStroke()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.StrokePath(path, brush, stroke)
	}
}
