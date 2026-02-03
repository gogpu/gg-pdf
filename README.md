# gg-pdf

PDF export backend for [gg](https://github.com/gogpu/gg)'s recording system.

Part of the [GoGPU](https://github.com/gogpu) ecosystem.

## Installation

```bash
go get github.com/gogpu/gg-pdf
```

## Usage

Import with a blank identifier to register the PDF backend:

```go
import (
    "github.com/gogpu/gg/recording"
    _ "github.com/gogpu/gg-pdf" // Register PDF backend
)

func main() {
    // Create a recorder
    rec := recording.NewRecorder(800, 600)

    // Draw something
    rec.SetFillRGBA(1, 0, 0, 1) // Red
    rec.DrawRectangle(100, 100, 200, 150)
    rec.Fill()

    // Finish recording
    r := rec.FinishRecording()

    // Create PDF backend
    backend, err := recording.NewBackend("pdf")
    if err != nil {
        log.Fatal(err)
    }

    // Playback to PDF
    r.Playback(backend)

    // Save to file
    if fb, ok := backend.(recording.FileBackend); ok {
        fb.SaveToFile("output.pdf")
    }
}
```

## Multi-Page Documents

For multi-page documents, use the `Document` type directly:

```go
import "github.com/gogpu/gg-pdf"

func main() {
    doc := pdf.NewDocument()
    doc.SetTitle("My Document")
    doc.SetAuthor("Author Name")

    // Page 1
    p1 := doc.NewPage(800, 600)
    // ... draw on p1 ...

    // Page 2
    p2 := doc.NewPage(600, 800)
    // ... draw on p2 ...

    // Save
    doc.SaveToFile("document.pdf")
}
```

## Features

- Solid color fills and strokes
- Linear and radial gradients
- Path operations (fill, stroke, clip)
- Transformations
- Stroke styles (width, cap, join, dash patterns)
- State management (Save/Restore)
- Multi-page documents
- Document metadata (title, author, subject, keywords)

## Limitations

- Sweep gradients fallback to first stop color (PDF limitation)
- Text uses Helvetica font only (custom font support planned)
- Clipping cannot be cleared (use Save/Restore instead)

## License

MIT License - see [LICENSE](LICENSE) for details.
