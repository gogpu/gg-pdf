package pdf

import (
	"fmt"
	"io"

	"github.com/coregx/gxpdf/creator"
	"github.com/gogpu/gg/recording"
)

// Document provides multi-page PDF document support.
// Unlike Backend which creates a single-page PDF, Document allows
// creating multiple pages with different sizes.
//
// Example:
//
//	doc := pdf.NewDocument()
//
//	// First page
//	backend := doc.NewPage(800, 600)
//	// ... draw on first page ...
//
//	// Second page
//	backend = doc.NewPage(600, 800)
//	// ... draw on second page ...
//
//	doc.SaveToFile("output.pdf")
type Document struct {
	creator  *creator.Creator
	pages    []*pageBackend
	finished bool
}

// pageBackend is a Backend that shares the creator with Document.
type pageBackend struct {
	*Backend
	doc *Document
}

// NewDocument creates a new multi-page PDF document.
func NewDocument() *Document {
	return &Document{
		creator: creator.New(),
		pages:   make([]*pageBackend, 0, 4),
	}
}

// NewPage creates a new page with the given dimensions and returns a backend for it.
// Each call to NewPage adds a new page to the document.
// The returned backend can be used with recording.Playback() to draw on the page.
func (d *Document) NewPage(width, height int) recording.Backend {
	// Create new backend using the document's creator
	pb := &pageBackend{
		Backend: &Backend{
			width:      float64(width),
			height:     float64(height),
			stateStack: make([]backendState, 0, 8),
		},
		doc: d,
	}

	// Store the creator reference
	pb.creator = d.creator

	// Create page with custom size (using A4 as base, will be overridden by surface)
	page, err := d.creator.NewPageWithSize(creator.A4)
	if err != nil {
		// Return backend anyway, it will fail on drawing
		return pb
	}

	pb.page = page
	pb.surface = page.Surface()
	pb.currentTransform = recording.Identity()

	// Apply Y-flip transform
	flipTransform := creator.Translate(0, float64(height)).Then(creator.Scale(1, -1))
	pb.surface.PushTransform(flipTransform)

	d.pages = append(d.pages, pb)
	return pb
}

// Playback replays a recording to a new page with the recording's dimensions.
// This is a convenience method that creates a page and plays the recording to it.
func (d *Document) Playback(rec *recording.Recording) error {
	backend := d.NewPage(rec.Width(), rec.Height())
	return rec.Playback(backend)
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() int {
	return len(d.pages)
}

// Finish finalizes all pages in the document.
// This must be called before WriteTo or SaveToFile.
func (d *Document) Finish() error {
	if d.finished {
		return nil
	}

	// Pop the Y-flip transform from each page
	for _, pb := range d.pages {
		if pb.surface != nil {
			pb.surface.Pop()
		}
	}

	d.finished = true
	return nil
}

// WriteTo writes the PDF to the given writer.
// Finish() is called automatically if not already called.
func (d *Document) WriteTo(w io.Writer) (int64, error) {
	if err := d.Finish(); err != nil {
		return 0, fmt.Errorf("pdf: failed to finish document: %w", err)
	}
	return d.creator.WriteTo(w)
}

// SaveToFile saves the PDF to a file at the given path.
// Finish() is called automatically if not already called.
func (d *Document) SaveToFile(path string) error {
	if err := d.Finish(); err != nil {
		return fmt.Errorf("pdf: failed to finish document: %w", err)
	}
	return d.creator.WriteToFile(path)
}

// SetTitle sets the document title metadata.
func (d *Document) SetTitle(title string) {
	d.creator.SetTitle(title)
}

// SetAuthor sets the document author metadata.
func (d *Document) SetAuthor(author string) {
	d.creator.SetAuthor(author)
}

// SetSubject sets the document subject metadata.
func (d *Document) SetSubject(subject string) {
	d.creator.SetSubject(subject)
}

// SetKeywords sets the document keywords metadata.
func (d *Document) SetKeywords(keywords string) {
	d.creator.SetKeywords(keywords)
}
