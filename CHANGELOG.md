# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-02-03

### Added

- **PDF Backend** for gg's recording system
  - `Backend` — implements `recording.Backend`, `recording.WriterBackend`, `recording.FileBackend`
  - Auto-registration via blank import (`import _ "github.com/gogpu/gg-pdf"`)
  - Solid color fills and strokes
  - Linear and radial gradients
  - Path operations (fill, stroke, clip)
  - Transformations (matrix)
  - Stroke styles (width, cap, join, dash patterns)
  - State management (Save/Restore)

- **Multi-page Document Support**
  - `Document` — multi-page PDF document
  - `NewPage(width, height)` — add pages with custom dimensions
  - `SetTitle`, `SetAuthor`, `SetSubject`, `SetKeywords` — document metadata

- **Project Infrastructure**
  - LICENSE (MIT)
  - CONTRIBUTING.md
  - CODE_OF_CONDUCT.md
  - SECURITY.md
  - GitHub Actions CI (build, test, lint on Linux/macOS/Windows)
  - golangci-lint configuration

### Notes

- Uses [gxpdf](https://github.com/nicholasq/gxpdf) for PDF generation
- Sweep gradients fallback to first stop color (PDF limitation)
- Text uses Helvetica font only (custom font support planned)
- Clipping cannot be cleared (use Save/Restore instead)
- Part of the [gogpu](https://github.com/gogpu) ecosystem

[0.1.0]: https://github.com/gogpu/gg-pdf/releases/tag/v0.1.0
