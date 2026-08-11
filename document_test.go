package pdf

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/coregx/gxpdf/creator"
	"github.com/gogpu/gg/recording"
)

func testRecording(width, height int) *recording.Recording {
	recorder := recording.NewRecorder(width, height)
	recorder.SetFillRGBA(1, 0, 0, 1)
	recorder.FillRectangle(10, 10, 20, 20)
	return recorder.FinishRecording()
}

func TestDocumentPlaybackPreservesPageOwnership(t *testing.T) {
	doc := NewDocument()
	docCreator := doc.creator
	backend := doc.NewPage(200, 100)
	page := backend.(*pageBackend)
	docPage := page.page

	if err := testRecording(200, 100).Playback(backend); err != nil {
		t.Fatalf("Playback failed: %v", err)
	}

	if page.creator != docCreator {
		t.Fatal("Playback replaced the document-owned creator")
	}
	if page.page != docPage {
		t.Fatal("Playback replaced the document-owned page")
	}
	if doc.creator.PageCount() != 1 {
		t.Fatalf("document creator has %d pages, want 1", doc.creator.PageCount())
	}
}

func TestDocumentPlaybackWritesPDF(t *testing.T) {
	doc := NewDocument()
	if err := doc.Playback(testRecording(200, 100)); err != nil {
		t.Fatalf("Playback failed: %v", err)
	}

	var output bytes.Buffer
	if _, err := doc.WriteTo(&output); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if !bytes.HasPrefix(output.Bytes(), []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
}

func TestDocumentPlaybackTwoPages(t *testing.T) {
	doc := NewDocument()
	for _, size := range [][2]int{{200, 100}, {100, 200}} {
		if err := doc.Playback(testRecording(size[0], size[1])); err != nil {
			t.Fatalf("Playback(%dx%d) failed: %v", size[0], size[1], err)
		}
	}

	if doc.PageCount() != 2 {
		t.Fatalf("PageCount = %d, want 2", doc.PageCount())
	}
	if doc.creator.PageCount() != 2 {
		t.Fatalf("document creator has %d pages, want 2", doc.creator.PageCount())
	}

	var output bytes.Buffer
	if _, err := doc.WriteTo(&output); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

func TestDocumentPageRejectsMismatchedPlaybackDimensions(t *testing.T) {
	doc := NewDocument()
	backend := doc.NewPage(200, 100)

	err := testRecording(201, 100).Playback(backend)
	if err == nil {
		t.Fatal("Playback succeeded with mismatched dimensions")
	}
	if !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("Playback error = %q, want a dimension mismatch", err)
	}

	var output bytes.Buffer
	if _, err := doc.WriteTo(&output); err != nil {
		t.Fatalf("WriteTo after rejected playback failed: %v", err)
	}
}

func TestDocumentPageLifecycleValidation(t *testing.T) {
	t.Run("creation error", func(t *testing.T) {
		creationErr := errors.New("page creation failed")
		page := &pageBackend{initErr: creationErr}

		if err := page.Begin(200, 100); !errors.Is(err, creationErr) {
			t.Fatalf("Begin error = %v, want %v", err, creationErr)
		}
		if err := page.End(); !errors.Is(err, creationErr) {
			t.Fatalf("End error = %v, want %v", err, creationErr)
		}
	})

	t.Run("uninitialized", func(t *testing.T) {
		page := &pageBackend{}
		if err := page.Begin(200, 100); err == nil || !strings.Contains(err.Error(), "not initialized") {
			t.Fatalf("Begin error = %v, want an initialization error", err)
		}
		if err := page.End(); err == nil || !strings.Contains(err.Error(), "not initialized") {
			t.Fatalf("End error = %v, want an initialization error", err)
		}
	})

	t.Run("wrong creator", func(t *testing.T) {
		doc := NewDocument()
		page := doc.NewPage(200, 100).(*pageBackend)
		page.creator = NewDocument().creator

		if err := page.Begin(200, 100); err == nil || !strings.Contains(err.Error(), "invalid creator") {
			t.Fatalf("Begin error = %v, want a creator ownership error", err)
		}
		if err := page.End(); err == nil || !strings.Contains(err.Error(), "invalid creator") {
			t.Fatalf("End error = %v, want a creator ownership error", err)
		}
	})

	t.Run("finalized", func(t *testing.T) {
		doc := NewDocument()
		page := doc.NewPage(200, 100)
		if err := page.End(); err != nil {
			t.Fatalf("End failed: %v", err)
		}
		if err := page.Begin(200, 100); err == nil || !strings.Contains(err.Error(), "finalized") {
			t.Fatalf("Begin error = %v, want a finalized-page error", err)
		}
	})
}

func TestDocumentPropagatesPageCreationFailure(t *testing.T) {
	doc := NewDocument()
	creationErr := errors.New("injected page creation failure")
	doc.newPage = func() (*creator.Page, error) {
		return nil, creationErr
	}

	page := doc.NewPage(200, 100)
	if got := doc.PageCount(); got != 1 {
		t.Fatalf("PageCount = %d, want failed page to remain tracked", got)
	}
	if err := page.Begin(200, 100); !errors.Is(err, creationErr) {
		t.Fatalf("Begin error = %v, want %v", err, creationErr)
	}

	err := doc.Finish()
	if !errors.Is(err, creationErr) {
		t.Fatalf("Finish error = %v, want wrapped %v", err, creationErr)
	}
	if !strings.Contains(err.Error(), "failed to finish page") {
		t.Fatalf("Finish error = %q, want page-finalization context", err)
	}
}

func TestDocumentFinishAfterPageEndIsIdempotent(t *testing.T) {
	doc := NewDocument()
	page := doc.NewPage(200, 100)

	if err := page.End(); err != nil {
		t.Fatalf("End failed: %v", err)
	}
	if err := page.End(); err != nil {
		t.Fatalf("second End failed: %v", err)
	}
	if err := doc.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if err := doc.Finish(); err != nil {
		t.Fatalf("second Finish failed: %v", err)
	}

	var output bytes.Buffer
	if _, err := doc.WriteTo(&output); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

func TestDocumentNewPageAfterFinishReturnsLifecycleError(t *testing.T) {
	doc := NewDocument()
	_ = doc.NewPage(200, 100)
	if err := doc.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	wantPages := doc.PageCount()
	wantCreatorPages := doc.creator.PageCount()

	latePage := doc.NewPage(100, 50)
	if err := latePage.Begin(100, 50); err == nil || !strings.Contains(err.Error(), "finished document") {
		t.Fatalf("Begin error = %v, want a finished-document error", err)
	}
	if err := latePage.End(); err == nil || !strings.Contains(err.Error(), "finished document") {
		t.Fatalf("End error = %v, want a finished-document error", err)
	}
	if doc.PageCount() != wantPages {
		t.Fatalf("PageCount = %d after rejected NewPage, want %d", doc.PageCount(), wantPages)
	}
	if doc.creator.PageCount() != wantCreatorPages {
		t.Fatalf("document creator has %d pages after rejected NewPage, want %d", doc.creator.PageCount(), wantCreatorPages)
	}
}

func TestDocumentPlaybackAfterFinishReturnsLifecycleError(t *testing.T) {
	doc := NewDocument()
	if err := doc.Finish(); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}

	err := doc.Playback(testRecording(200, 100))
	if err == nil || !strings.Contains(err.Error(), "finished document") {
		t.Fatalf("Playback error = %v, want a finished-document error", err)
	}
	if doc.PageCount() != 0 {
		t.Fatalf("PageCount = %d after rejected Playback, want 0", doc.PageCount())
	}
	if doc.creator.PageCount() != 0 {
		t.Fatalf("document creator has %d pages after rejected Playback, want 0", doc.creator.PageCount())
	}
}
