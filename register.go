// Package pdf provides a PDF export backend for gg's recording system.
//
// This package registers a "pdf" backend that can be used to export
// recorded drawing operations to PDF format using the gxpdf library.
//
// # Usage
//
// Import this package with a blank identifier to register the PDF backend:
//
//	import _ "github.com/gogpu/gg-pdf"
//
// Then use recording.NewBackend("pdf") to create a PDF backend:
//
//	backend, err := recording.NewBackend("pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Record drawing operations
//	rec := recording.NewRecorder(800, 600)
//	// ... draw ...
//	r := rec.Finish()
//
//	// Playback to PDF backend
//	r.Playback(backend)
//
//	// Save to file
//	if fb, ok := backend.(recording.FileBackend); ok {
//	    fb.SaveToFile("output.pdf")
//	}
package pdf

import "github.com/gogpu/gg/recording"

func init() {
	recording.Register("pdf", func() recording.Backend {
		return NewBackend()
	})
}
