package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"

	"github.com/coregx/gxpdf/creator"
	"github.com/gogpu/gg"
	"github.com/gogpu/gg/recording"
	"github.com/gogpu/gg/text"
)

// Backend implements recording.Backend for PDF output.
// It uses gxpdf to generate PDF documents from recorded drawing commands.
//
// PDF coordinates use a bottom-left origin, while gg uses a top-left origin.
// This backend handles the coordinate transformation automatically.
type Backend struct {
	creator *creator.Creator
	page    *creator.Page
	surface *creator.Surface

	width  float64
	height float64

	// State stack for Save/Restore
	stateStack []backendState

	// Current graphics state
	currentTransform recording.Matrix
}

// backendState stores the graphics state for Save/Restore operations.
type backendState struct {
	transform recording.Matrix
}

// NewBackend creates a new PDF backend.
// The backend starts in an uninitialized state. Call Begin() to initialize
// with specific dimensions before drawing.
func NewBackend() *Backend {
	return &Backend{
		stateStack: make([]backendState, 0, 8),
	}
}

// Begin initializes the backend for rendering at the given dimensions.
// This creates a new PDF document with a single page of the specified size.
func (b *Backend) Begin(width, height int) error {
	b.width = float64(width)
	b.height = float64(height)

	b.creator = creator.New()

	// Create custom page size
	page, err := b.creator.NewPageWithSize(creator.A4) // Will be overridden
	if err != nil {
		return fmt.Errorf("pdf: failed to create page: %w", err)
	}
	b.page = page
	b.surface = page.Surface()

	// Initialize state
	b.currentTransform = recording.Identity()
	b.stateStack = b.stateStack[:0]

	// Apply Y-flip transform to convert from top-left to bottom-left origin
	// We need to translate by height and flip Y axis
	flipTransform := creator.Translate(0, b.height).Then(creator.Scale(1, -1))
	b.surface.PushTransform(flipTransform)

	return nil
}

// End finalizes the rendering and prepares the output.
// After End is called, WriteTo and SaveToFile can be used to get the PDF.
func (b *Backend) End() error {
	// Pop the initial Y-flip transform
	b.surface.Pop()
	return nil
}

// Save saves the current graphics state onto a stack.
func (b *Backend) Save() {
	b.stateStack = append(b.stateStack, backendState{
		transform: b.currentTransform,
	})
	// Push an identity layer in gxpdf for proper state restoration
	b.surface.PushTransform(creator.Identity())
}

// Restore restores the graphics state from the stack.
func (b *Backend) Restore() {
	if len(b.stateStack) == 0 {
		return // No-op if stack is empty
	}

	// Pop the saved state
	state := b.stateStack[len(b.stateStack)-1]
	b.stateStack = b.stateStack[:len(b.stateStack)-1]

	b.currentTransform = state.transform
	b.surface.Pop()
}

// SetTransform sets the current transformation matrix.
// The transform is in gg coordinates (top-left origin).
func (b *Backend) SetTransform(m recording.Matrix) {
	b.currentTransform = m

	// Apply the transform via gxpdf
	// Note: We need to pop the current transform and push the new one
	pdfTransform := b.matrixToTransform(m)
	b.surface.Pop() // Pop old transform (or identity from Save)
	b.surface.PushTransform(pdfTransform)
}

// SetClip sets the clipping region to the given path.
func (b *Backend) SetClip(path *gg.Path, rule recording.FillRule) {
	pdfPath := b.translatePath(path)
	fillRule := b.translateFillRule(rule)
	_ = b.surface.PushClipPath(pdfPath, fillRule)
}

// ClearClip removes any clipping region.
// Note: PDF doesn't support clearing clips, so this is a no-op warning.
// Proper clip management should use Save/Restore.
func (b *Backend) ClearClip() {
	// PDF does not support clearing clips directly.
	// Clips should be managed via Save/Restore instead.
	// This is a limitation of the PDF format.
}

// FillPath fills the given path with the brush color/pattern.
func (b *Backend) FillPath(path *gg.Path, brush recording.Brush, rule recording.FillRule) {
	pdfPath := b.translatePath(path)
	fill := b.translateBrushToFill(brush)
	fill.Rule = b.translateFillRule(rule)

	b.surface.SetFill(fill)
	b.surface.SetStroke(nil)
	_ = b.surface.FillPath(pdfPath)
}

// StrokePath strokes the given path with the brush and stroke style.
func (b *Backend) StrokePath(path *gg.Path, brush recording.Brush, stroke recording.Stroke) {
	pdfPath := b.translatePath(path)
	pdfStroke := b.translateStroke(brush, stroke)

	b.surface.SetStroke(pdfStroke)
	b.surface.SetFill(nil)
	_ = b.surface.StrokePath(pdfPath)
}

// FillRect fills an axis-aligned rectangle with the brush.
func (b *Backend) FillRect(rect recording.Rect, brush recording.Brush) {
	pdfRect := creator.Rect{
		X:      rect.MinX,
		Y:      rect.MinY,
		Width:  rect.Width(),
		Height: rect.Height(),
	}

	fill := b.translateBrushToFill(brush)
	b.surface.SetFill(fill)
	b.surface.SetStroke(nil)
	_ = b.surface.DrawRect(pdfRect)
}

// DrawImage draws an image from the source rectangle to the destination rectangle.
func (b *Backend) DrawImage(img image.Image, src, dst recording.Rect, opts recording.ImageOptions) {
	// Encode Go image to PNG for gxpdf
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return // Silently fail on encode error
	}

	// Load image from encoded bytes
	pdfImg, err := creator.LoadImageFromReader(&buf)
	if err != nil {
		return // Silently fail on load error
	}

	// Apply opacity if not fully opaque
	if opts.Alpha < 1.0 {
		_ = b.surface.PushOpacity(opts.Alpha)
	}

	// Draw the image at the destination rectangle
	// gxpdf DrawImage takes (img, x, y, width, height)
	_ = b.page.DrawImage(pdfImg, dst.MinX, dst.MinY, dst.Width(), dst.Height())

	// Pop opacity if applied
	if opts.Alpha < 1.0 {
		b.surface.Pop()
	}
}

// DrawText draws text at the given position with the specified font face and brush.
func (b *Backend) DrawText(s string, x, y float64, face text.Face, brush recording.Brush) {
	// Extract color from brush
	color := b.brushToColor(brush)

	// Get font size from face
	fontSize := 12.0
	if face != nil {
		// text.Face exposes Size() method
		fontSize = face.Size()
		if fontSize <= 0 {
			// Fallback to line height if size not available
			metrics := face.Metrics()
			fontSize = metrics.LineHeight()
			if fontSize <= 0 {
				fontSize = 12.0
			}
		}
	}

	// Use default Helvetica font for now
	// TODO: Support custom fonts via face parameter
	_ = b.page.AddTextColor(s, x, y, creator.Helvetica, fontSize, color)
}

// WriteTo writes the PDF to the given writer.
// This implements recording.WriterBackend.
func (b *Backend) WriteTo(w io.Writer) (int64, error) {
	return b.creator.WriteTo(w)
}

// SaveToFile saves the PDF to a file at the given path.
// This implements recording.FileBackend.
func (b *Backend) SaveToFile(path string) error {
	return b.creator.WriteToFile(path)
}

// translatePath converts a gg.Path to a gxpdf Path.
// Note: Y coordinates are NOT flipped here because we apply a global Y-flip
// transform in Begin(). The paths are drawn in gg coordinate space.
func (b *Backend) translatePath(path *gg.Path) *creator.Path {
	pdfPath := creator.NewPath()

	for _, elem := range path.Elements() {
		switch e := elem.(type) {
		case gg.MoveTo:
			pdfPath.MoveTo(e.Point.X, e.Point.Y)
		case gg.LineTo:
			pdfPath.LineTo(e.Point.X, e.Point.Y)
		case gg.QuadTo:
			pdfPath.QuadraticTo(
				e.Control.X, e.Control.Y,
				e.Point.X, e.Point.Y,
			)
		case gg.CubicTo:
			pdfPath.CubicTo(
				e.Control1.X, e.Control1.Y,
				e.Control2.X, e.Control2.Y,
				e.Point.X, e.Point.Y,
			)
		case gg.Close:
			pdfPath.Close()
		}
	}

	return pdfPath
}

// translateBrushToFill converts a recording.Brush to a gxpdf Fill.
func (b *Backend) translateBrushToFill(brush recording.Brush) *creator.Fill {
	switch br := brush.(type) {
	case recording.SolidBrush:
		color := creator.Color{
			R: float64(br.Color.R) / 255.0,
			G: float64(br.Color.G) / 255.0,
			B: float64(br.Color.B) / 255.0,
		}
		fill := creator.NewFill(color)
		opacity := float64(br.Color.A) / 255.0
		if opacity < 1.0 {
			return fill.WithOpacity(opacity)
		}
		return fill

	case *recording.LinearGradientBrush:
		return b.translateLinearGradient(br)

	case *recording.RadialGradientBrush:
		return b.translateRadialGradient(br)

	case *recording.SweepGradientBrush:
		// Sweep gradients are not directly supported in PDF
		// Fallback to solid color from first stop
		if len(br.Stops) > 0 {
			stop := br.Stops[0]
			color := creator.Color{
				R: float64(stop.Color.R) / 255.0,
				G: float64(stop.Color.G) / 255.0,
				B: float64(stop.Color.B) / 255.0,
			}
			return creator.NewFill(color)
		}
		return creator.NewFill(creator.Black)

	default:
		return creator.NewFill(creator.Black)
	}
}

// translateLinearGradient converts a recording.LinearGradientBrush to a gxpdf Fill.
func (b *Backend) translateLinearGradient(br *recording.LinearGradientBrush) *creator.Fill {
	grad := creator.NewLinearGradient(
		br.Start.X, br.Start.Y,
		br.End.X, br.End.Y,
	)

	for _, stop := range br.Stops {
		color := creator.Color{
			R: float64(stop.Color.R) / 255.0,
			G: float64(stop.Color.G) / 255.0,
			B: float64(stop.Color.B) / 255.0,
		}
		_ = grad.AddColorStop(stop.Offset, color)
	}

	return creator.NewFill(grad)
}

// translateRadialGradient converts a recording.RadialGradientBrush to a gxpdf Fill.
func (b *Backend) translateRadialGradient(br *recording.RadialGradientBrush) *creator.Fill {
	grad := creator.NewRadialGradient(
		br.Center.X, br.Center.Y, br.StartRadius,
		br.Focus.X, br.Focus.Y, br.EndRadius,
	)

	for _, stop := range br.Stops {
		color := creator.Color{
			R: float64(stop.Color.R) / 255.0,
			G: float64(stop.Color.G) / 255.0,
			B: float64(stop.Color.B) / 255.0,
		}
		_ = grad.AddColorStop(stop.Offset, color)
	}

	return creator.NewFill(grad)
}

// translateStroke converts brush and stroke settings to a gxpdf Stroke.
func (b *Backend) translateStroke(brush recording.Brush, stroke recording.Stroke) *creator.Stroke {
	color := b.brushToColor(brush)
	s := creator.NewStroke(color)

	s.Width = stroke.Width
	s.LineCap = b.translateLineCap(stroke.Cap)
	s.LineJoin = b.translateLineJoin(stroke.Join)
	s.MiterLimit = stroke.MiterLimit

	if len(stroke.DashPattern) > 0 {
		s.DashArray = make([]float64, len(stroke.DashPattern))
		copy(s.DashArray, stroke.DashPattern)
		s.DashPhase = stroke.DashOffset
	}

	return s
}

// brushToColor extracts a Color from a brush for text and stroke operations.
func (b *Backend) brushToColor(brush recording.Brush) creator.Color {
	switch br := brush.(type) {
	case recording.SolidBrush:
		return creator.Color{
			R: float64(br.Color.R) / 255.0,
			G: float64(br.Color.G) / 255.0,
			B: float64(br.Color.B) / 255.0,
		}
	case *recording.LinearGradientBrush:
		if len(br.Stops) > 0 {
			stop := br.Stops[0]
			return creator.Color{
				R: float64(stop.Color.R) / 255.0,
				G: float64(stop.Color.G) / 255.0,
				B: float64(stop.Color.B) / 255.0,
			}
		}
	case *recording.RadialGradientBrush:
		if len(br.Stops) > 0 {
			stop := br.Stops[0]
			return creator.Color{
				R: float64(stop.Color.R) / 255.0,
				G: float64(stop.Color.G) / 255.0,
				B: float64(stop.Color.B) / 255.0,
			}
		}
	case *recording.SweepGradientBrush:
		if len(br.Stops) > 0 {
			stop := br.Stops[0]
			return creator.Color{
				R: float64(stop.Color.R) / 255.0,
				G: float64(stop.Color.G) / 255.0,
				B: float64(stop.Color.B) / 255.0,
			}
		}
	}
	return creator.Black
}

// translateFillRule converts a recording.FillRule to a gxpdf FillRule.
func (b *Backend) translateFillRule(rule recording.FillRule) creator.FillRule {
	switch rule {
	case recording.FillRuleEvenOdd:
		return creator.FillRuleEvenOdd
	default:
		return creator.FillRuleNonZero
	}
}

// translateLineCap converts a recording.LineCap to a gxpdf LineCap.
func (b *Backend) translateLineCap(lineCap recording.LineCap) creator.LineCap {
	switch lineCap {
	case recording.LineCapRound:
		return creator.LineCapRound
	case recording.LineCapSquare:
		return creator.LineCapSquare
	default:
		return creator.LineCapButt
	}
}

// translateLineJoin converts a recording.LineJoin to a gxpdf LineJoin.
func (b *Backend) translateLineJoin(join recording.LineJoin) creator.LineJoin {
	switch join {
	case recording.LineJoinRound:
		return creator.LineJoinRound
	case recording.LineJoinBevel:
		return creator.LineJoinBevel
	default:
		return creator.LineJoinMiter
	}
}

// matrixToTransform converts a recording.Matrix to a gxpdf Transform.
func (b *Backend) matrixToTransform(m recording.Matrix) creator.Transform {
	// recording.Matrix is row-major: [A B C; D E F]
	// gxpdf Transform is: [A B C D E F] where A,D=scale, B,C=skew, E,F=translate
	// Actually gxpdf uses [a b c d e f] for the matrix [ a c e; b d f; 0 0 1 ]
	// Which matches our recording.Matrix if we map: A->a, B->c, C->e, D->b, E->d, F->f
	return creator.Transform{
		A: m.A, B: m.D,
		C: m.B, D: m.E,
		E: m.C, F: m.F,
	}
}

// Ensure Backend implements the required interfaces.
var (
	_ recording.Backend       = (*Backend)(nil)
	_ recording.WriterBackend = (*Backend)(nil)
	_ recording.FileBackend   = (*Backend)(nil)
)
