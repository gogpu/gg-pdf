package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/coregx/gxpdf/creator"
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

func TestColorTranslationPreservesNormalizedComponents(t *testing.T) {
	backend := NewBackend()
	input := gg.RGBA2(0.2, 0.4, 0.8, 0.35)
	brush := recording.NewSolidBrush(input)
	want := creator.Color{R: input.R, G: input.G, B: input.B}

	fill := backend.translateBrushToFill(brush)
	if got, ok := fill.Paint.(creator.Color); !ok {
		t.Fatalf("solid fill paint has type %T, want creator.Color", fill.Paint)
	} else if got != want {
		t.Errorf("solid fill color = %+v, want %+v", got, want)
	}
	if fill.Opacity != input.A {
		t.Errorf("solid fill opacity = %v, want %v", fill.Opacity, input.A)
	}

	if got := backend.brushToColor(brush); got != want {
		t.Errorf("brush color = %+v, want %+v", got, want)
	}

	stroke := backend.translateStroke(brush, recording.DefaultStroke())
	if got, ok := stroke.Paint.(creator.Color); !ok {
		t.Fatalf("stroke paint has type %T, want creator.Color", stroke.Paint)
	} else if got != want {
		t.Errorf("stroke color = %+v, want %+v", got, want)
	}
}

func TestGradientTranslationPreservesNormalizedComponents(t *testing.T) {
	backend := NewBackend()
	stops := []gg.RGBA{
		gg.RGB(0.1, 0.25, 0.5),
		gg.RGB(0.75, 0.5, 0.2),
	}

	linear := recording.NewLinearGradientBrush(0, 0, 100, 100).
		AddColorStop(0, stops[0]).
		AddColorStop(1, stops[1])
	linearFill := backend.translateBrushToFill(linear)
	assertGradientColors(t, linearFill, stops)

	radial := recording.NewRadialGradientBrush(50, 50, 0, 50).
		AddColorStop(0, stops[0]).
		AddColorStop(1, stops[1])
	radialFill := backend.translateBrushToFill(radial)
	assertGradientColors(t, radialFill, stops)

	sweep := recording.NewSweepGradientBrush(50, 50, 0).
		AddColorStop(0, stops[0]).
		AddColorStop(1, stops[1])
	sweepFill := backend.translateBrushToFill(sweep)
	if got, ok := sweepFill.Paint.(creator.Color); !ok {
		t.Fatalf("sweep fallback paint has type %T, want creator.Color", sweepFill.Paint)
	} else if want := (creator.Color{R: stops[0].R, G: stops[0].G, B: stops[0].B}); got != want {
		t.Errorf("sweep fallback color = %+v, want %+v", got, want)
	}
}

func TestGradientStrokeUsesNormalizedFirstStopColor(t *testing.T) {
	backend := NewBackend()
	if err := backend.Begin(100, 100); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	t.Cleanup(func() {
		if err := backend.End(); err != nil {
			t.Errorf("End failed: %v", err)
		}
	})

	want := gg.RGB(0.2, 0.4, 0.8)
	path := gg.NewPath()
	path.MoveTo(10, 10)
	path.LineTo(90, 90)

	tests := []struct {
		name  string
		brush recording.Brush
	}{
		{
			name: "linear",
			brush: recording.NewLinearGradientBrush(0, 0, 100, 100).
				AddColorStop(0, want).
				AddColorStop(1, gg.RGB(1, 0, 0)),
		},
		{
			name: "radial",
			brush: recording.NewRadialGradientBrush(50, 50, 0, 50).
				AddColorStop(0, want).
				AddColorStop(1, gg.RGB(1, 0, 0)),
		},
		{
			name: "sweep",
			brush: recording.NewSweepGradientBrush(50, 50, 0).
				AddColorStop(0, want).
				AddColorStop(1, gg.RGB(1, 0, 0)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend.StrokePath(path, tt.brush, recording.DefaultStroke())

			stroke := backend.surface.CurrentStroke()
			if stroke == nil {
				t.Fatal("StrokePath did not configure a stroke")
			}
			got, ok := stroke.Paint.(creator.Color)
			if !ok {
				t.Fatalf("stroke paint has type %T, want creator.Color", stroke.Paint)
			}
			if expected := (creator.Color{R: want.R, G: want.G, B: want.B}); got != expected {
				t.Errorf("stroke color = %+v, want first gradient stop %+v", got, expected)
			}
		})
	}
}

func assertGradientColors(t *testing.T, fill *creator.Fill, stops []gg.RGBA) {
	t.Helper()

	gradient, ok := fill.Paint.(*creator.Gradient)
	if !ok {
		t.Fatalf("gradient fill paint has type %T, want *creator.Gradient", fill.Paint)
	}
	if len(gradient.ColorStops) != len(stops) {
		t.Fatalf("gradient has %d color stops, want %d", len(gradient.ColorStops), len(stops))
	}
	for i, stop := range stops {
		want := creator.Color{R: stop.R, G: stop.G, B: stop.B}
		if got := gradient.ColorStops[i].Color; got != want {
			t.Errorf("gradient color stop %d = %+v, want %+v", i, got, want)
		}
	}
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

func TestBackendYFlipTransform(t *testing.T) {
	const height = 240

	backend := NewBackend()
	if err := backend.Begin(320, height); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	want := creator.Scale(1, -1).Then(creator.Translate(0, height))
	if got := backend.surface.CurrentTransform(); got != want {
		t.Fatalf("Y-flip transform = %#v, want %#v", got, want)
	}

	if err := backend.End(); err != nil {
		t.Fatalf("End failed: %v", err)
	}
}

func TestDocumentYFlipTransform(t *testing.T) {
	const height = 240

	doc := NewDocument()
	backend, ok := doc.NewPage(320, height).(*pageBackend)
	if !ok {
		t.Fatal("Document.NewPage returned an unexpected backend type")
	}

	want := creator.Scale(1, -1).Then(creator.Translate(0, height))
	if got := backend.surface.CurrentTransform(); got != want {
		t.Fatalf("Y-flip transform = %#v, want %#v", got, want)
	}

	if err := doc.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
}

func TestBackendPageDimensions(t *testing.T) {
	backend := NewBackend()
	if err := backend.Begin(123, 456); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := backend.End(); err != nil {
		t.Fatalf("End failed: %v", err)
	}

	var buf bytes.Buffer
	if _, err := backend.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Keep this assertion tied to the serialized MediaBox rather than only
	// Backend.width/height so it catches regressions in page creation.
	const mediaBox = "/MediaBox [0.00 0.00 123.00 456.00]"
	if !bytes.Contains(buf.Bytes(), []byte(mediaBox)) {
		t.Fatalf("PDF does not contain requested page dimensions %q:\n%s", mediaBox, buf.Bytes())
	}
	if bytes.Contains(buf.Bytes(), []byte("/MediaBox [0.00 0.00 595.00 842.00]")) {
		t.Fatal("PDF unexpectedly uses the A4 page dimensions")
	}
}

func TestBackendBeginRejectsNonPositiveDimensions(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
	}{
		{name: "zero width", width: 0, height: 456},
		{name: "zero height", width: 123, height: 0},
		{name: "negative width", width: -123, height: 456},
		{name: "negative height", width: 123, height: -456},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewBackend().Begin(tt.width, tt.height); err == nil {
				t.Fatalf("Begin(%d, %d) succeeded, want an error", tt.width, tt.height)
			}
		})
	}
}

func TestBackendBeginFailurePreservesPriorState(t *testing.T) {
	backend := NewBackend()
	if err := backend.Begin(123, 456); err != nil {
		t.Fatalf("initial Begin failed: %v", err)
	}

	if err := backend.Begin(0, 456); err == nil {
		t.Fatal("invalid Begin succeeded, want an error")
	}

	// The failed Begin must leave the previous page usable. End should pop its
	// original Y-flip transform, and serialization should still use its page.
	if err := backend.End(); err != nil {
		t.Fatalf("End after failed Begin failed: %v", err)
	}
	var buf bytes.Buffer
	if _, err := backend.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo after failed Begin failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/MediaBox [0.00 0.00 123.00 456.00]")) {
		t.Fatalf("failed Begin corrupted the previous page:\n%s", buf.Bytes())
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
	brush := recording.NewSolidBrush(gg.RGB(1, 0, 0))

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
	brush := recording.NewSolidBrush(gg.RGB(0, 0, 1))
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
	brush := recording.NewSolidBrush(gg.RGBA2(0, 1, 0, 200.0/255.0))

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
		AddColorStop(0, gg.RGB(1, 0, 0)).
		AddColorStop(0.5, gg.RGB(0, 1, 0)).
		AddColorStop(1, gg.RGB(0, 0, 1))

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
		AddColorStop(0, gg.RGB(1, 1, 0)).
		AddColorStop(1, gg.RGB(1, 0, 0))

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

	brush := recording.NewSolidBrush(gg.RGB(0, 0, 0))
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

	brush := recording.NewSolidBrush(gg.RGB(1, 100.0/255.0, 100.0/255.0))
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

		brush := recording.NewSolidBrush(gg.RGB(100.0/255.0, 100.0/255.0, 1))
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
	brush := recording.NewSolidBrush(gg.RGB(100.0/255.0, 150.0/255.0, 200.0/255.0))
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
	brush := recording.NewSolidBrush(gg.RGB(1, 0, 0))
	p1.FillPath(path, brush, recording.FillRuleNonZero)

	// Create second page
	p2 := doc.NewPage(300, 400)
	path2 := gg.NewPath()
	path2.Circle(150, 200, 50)
	brush2 := recording.NewSolidBrush(gg.RGB(0, 0, 1))
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

func TestDocumentPageDimensions(t *testing.T) {
	doc := NewDocument()
	doc.NewPage(123, 456)
	doc.NewPage(789, 321)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.Bytes()
	for _, mediaBox := range []string{
		"/MediaBox [0.00 0.00 123.00 456.00]",
		"/MediaBox [0.00 0.00 789.00 321.00]",
	} {
		if !bytes.Contains(pdf, []byte(mediaBox)) {
			t.Errorf("PDF does not contain requested page dimensions %q:\n%s", mediaBox, pdf)
		}
	}
	if bytes.Contains(pdf, []byte("/MediaBox [0.00 0.00 595.00 842.00]")) {
		t.Fatal("PDF unexpectedly uses the A4 page dimensions")
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
		AddColorStop(0, gg.RGB(1, 0, 0)).
		AddColorStop(1, gg.RGB(0, 1, 0))

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
	brush := recording.NewSolidBrush(gg.RGB(1, 0, 0))

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

	brush := recording.NewSolidBrush(gg.RGB(0, 0, 0))
	stroke := recording.DefaultStroke()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.StrokePath(path, brush, stroke)
	}
}
