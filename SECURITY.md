# Security Policy

## Supported Versions

gogpu/gg-pdf is currently in early development (v0.x.x).

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1.0 | :x:                |

## Reporting a Vulnerability

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security issues via:

1. **Private Security Advisory** (preferred):
   https://github.com/gogpu/gg-pdf/security/advisories/new

2. **GitHub Discussions** (for less critical issues):
   https://github.com/gogpu/gg-pdf/discussions

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

### Response Timeline

- **Initial Response**: Within 72 hours
- **Fix & Disclosure**: Coordinated with reporter

## Security Considerations

gogpu/gg-pdf is a PDF generation library. Security considerations:

1. **File System** — SaveToFile writes to specified paths
2. **Memory** — Large documents allocate significant memory
3. **Input Validation** — Font/image data should be validated

## Security Contact

- **GitHub Security Advisory**: https://github.com/gogpu/gg-pdf/security/advisories/new
- **Public Issues**: https://github.com/gogpu/gg-pdf/issues

---

**Thank you for helping keep gogpu/gg-pdf secure!**
