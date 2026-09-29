package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

func NormalizeName(name string) (string, error) {
	if !utf8.ValidString(name) || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", ErrInvalid
	}
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", ErrInvalid
	}
	return name, nil
}

func DetectDocument(filename string, data []byte) (string, error) {
	if !utf8.ValidString(filename) || filename != strings.TrimSpace(filename) || len(filename) > 255 || filename == "" || strings.ContainsAny(filename, "/\\") || strings.IndexFunc(filename, unicode.IsControl) >= 0 || len(data) == 0 || len(data) > MaxUploadBytes {
		return "", ErrInvalid
	}
	ext := strings.ToLower(path.Ext(filename))
	pdf := bytes.HasPrefix(data, []byte("%PDF-"))
	zipFile := bytes.HasPrefix(data, []byte("PK\x03\x04"))
	switch ext {
	case ".txt", ".md", ".markdown":
		if pdf || zipFile || !utf8.Valid(data) || bytes.IndexFunc(data, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) >= 0 {
			return "", ErrInvalid
		}
		if ext != ".txt" {
			return "text/markdown", nil
		}
		return "text/plain", nil
	case ".pdf":
		if pdf {
			return "application/pdf", nil
		}
	case ".docx":
		if !zipFile {
			return "", ErrInvalid
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil || len(zr.File) > 256 {
			return "", ErrInvalid
		}
		var total uint64
		var document, contentType bool
		seen := map[string]bool{}
		for _, f := range zr.File {
			if seen[f.Name] || strings.HasPrefix(f.Name, "/") || strings.Contains(f.Name, "..") {
				return "", ErrInvalid
			}
			seen[f.Name] = true
			total += f.UncompressedSize64
			if total > 64<<20 {
				return "", ErrInvalid
			}
			if f.Name == "word/document.xml" {
				document = true
			}
			if f.Name == "[Content_Types].xml" {
				if f.UncompressedSize64 > 64<<10 {
					return "", ErrInvalid
				}
				reader, e := f.Open()
				if e != nil {
					return "", ErrInvalid
				}
				var types struct {
					Overrides []struct {
						PartName    string `xml:"PartName,attr"`
						ContentType string `xml:"ContentType,attr"`
					} `xml:"Override"`
				}
				e = xml.NewDecoder(io.LimitReader(reader, 64<<10)).Decode(&types)
				_ = reader.Close()
				if e != nil {
					return "", ErrInvalid
				}
				for _, entry := range types.Overrides {
					if entry.PartName == "/word/document.xml" && entry.ContentType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml" {
						contentType = true
					}
				}
			}
		}
		if document && contentType {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document", nil
		}
	}
	return "", ErrInvalid
}
