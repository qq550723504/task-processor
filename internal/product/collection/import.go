package collection

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

var ImportHeaders = []string{"title", "description", "brand", "images", "sku", "currency", "price", "stock"}

type ImportPreview struct {
	Products []OwnProduct `json:"products"`
}
type ImportTemplate struct {
	Content string `json:"content"`
}

func (s *Service) PreviewImport(ctx context.Context, raw []byte) (ImportPreview, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, PermissionManage)
	if err != nil {
		return ImportPreview{}, err
	}
	products, err := ParseImport(ctx, raw)
	if err != nil {
		return ImportPreview{}, err
	}
	fresh, err := s.authorize(ctx, PermissionManage)
	if err != nil || fresh != scope {
		return ImportPreview{}, ErrForbidden
	}
	return ImportPreview{Products: products}, nil
}
func (s *Service) ImportTemplate(ctx context.Context) (ImportTemplate, error) {
	if _, err := s.authorize(ctx, PermissionRead); err != nil {
		return ImportTemplate{}, err
	}
	f := excelize.NewFile()
	defer f.Close()
	for i, name := range ImportHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue("Sheet1", cell, name); err != nil {
			return ImportTemplate{}, ErrUnavailable
		}
	}
	b, err := f.WriteToBuffer()
	if err != nil {
		return ImportTemplate{}, ErrUnavailable
	}
	return ImportTemplate{Content: base64.StdEncoding.EncodeToString(b.Bytes())}, nil
}

// The ZIP/XML scan is a resource and content gate. Excelize remains the workbook parser.
func validateImportArchive(ctx context.Context, raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxPayloadBytes {
		return ErrInvalid
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(z.File) > 128 {
		return ErrInvalid
	}
	total, sheets := int64(0), 0
	seen := map[string]bool{}
	for _, entry := range z.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.ToLower(entry.Name)
		if seen[name] || strings.Contains(name, "..") || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") {
			return ErrInvalid
		}
		seen[name] = true
		if strings.Contains(name, "vbaproject") || strings.Contains(name, "externallinks") || strings.Contains(name, "embeddings/") || strings.Contains(name, "xl/media/") {
			return ErrInvalid
		}
		if strings.HasPrefix(name, "xl/worksheets/") && strings.HasSuffix(name, ".xml") {
			sheets++
		}
		r, e := entry.Open()
		if e != nil {
			return ErrInvalid
		}
		b, e := io.ReadAll(io.LimitReader(r, (2<<20)+1))
		r.Close()
		total += int64(len(b))
		if e != nil || len(b) > 2<<20 || total > 8<<20 {
			return ErrInvalid
		}
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			return ErrInvalid
		}
		decoder := xml.NewDecoder(bytes.NewReader(b))
		for {
			tok, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return ErrInvalid
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			switch t := tok.(type) {
			case xml.Directive:
				return ErrInvalid
			case xml.StartElement:
				if t.Name.Local == "f" {
					return ErrInvalid
				}
				for _, a := range t.Attr {
					if a.Name.Local == "TargetMode" && strings.EqualFold(a.Value, "External") {
						return ErrInvalid
					}
					if strings.HasPrefix(name, "xl/worksheets/") && a.Name.Local == "r" {
						if t.Name.Local == "row" {
							n, e := strconv.Atoi(a.Value)
							if e != nil || n < 1 || n > 201 {
								return ErrInvalid
							}
						}
						if t.Name.Local == "c" {
							col, row, e := excelize.CellNameToCoordinates(a.Value)
							if e != nil || col < 1 || col > 8 || row < 1 || row > 201 {
								return ErrInvalid
							}
						}
					}
				}
			}
		}
	}
	if sheets != 1 {
		return ErrInvalid
	}
	return nil
}
func ParseImport(ctx context.Context, raw []byte) ([]OwnProduct, error) {
	if err := validateImportArchive(ctx, raw); err != nil {
		return nil, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw), excelize.Options{UnzipSizeLimit: 8 << 20, UnzipXMLSizeLimit: 2 << 20, RawCellValue: true})
	if err != nil {
		return nil, ErrInvalid
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 1 {
		return nil, ErrInvalid
	}
	rows, err := f.GetRows(sheets[0], excelize.Options{RawCellValue: true})
	if err != nil || len(rows) < 2 || len(rows) > 201 || len(rows[0]) != 8 {
		return nil, ErrInvalid
	}
	for i, h := range ImportHeaders {
		if rows[0][i] != h {
			return nil, ErrInvalid
		}
	}
	products := []OwnProduct{}
	for i, row := range rows[1:] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		values := make([]string, 8)
		if len(row) > 8 {
			return nil, ErrInvalid
		}
		copy(values, row)
		if strings.Join(values, "") == "" {
			continue
		}
		p := OwnProduct{Title: values[0], Description: values[1], Brand: values[2], Images: []string{}}
		if values[3] != "" {
			for _, url := range strings.FieldsFunc(values[3], func(r rune) bool { return r == '\n' || r == '|' }) {
				p.Images = append(p.Images, strings.TrimSpace(url))
			}
		}
		if values[4] != "" {
			price, e := strconv.ParseFloat(values[6], 64)
			stock, se := strconv.Atoi(values[7])
			if e != nil || se != nil || price < 0 || stock < 0 || len(values[5]) != 3 || strings.ToUpper(values[5]) != values[5] {
				return nil, ErrInvalid
			}
			p.Variants = []OwnVariant{{SourceID: "row-" + strconv.Itoa(i+2), Title: p.Title, SKU: values[4], Currency: values[5], Price: price, Stock: stock, Attributes: map[string]string{}}}
		} else if values[5] != "" || values[6] != "" || values[7] != "" {
			return nil, ErrInvalid
		}
		if _, err := OwnEnvelope(StableID("preview", strconv.Itoa(i)), p); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	encoded, err := json.Marshal(Mutation{Action: "import_products", Name: strings.Repeat("x", 200), Products: products})
	if err != nil || len(encoded) > MaxPayloadBytes || len(products) == 0 || len(products) > 200 {
		return nil, ErrInvalid
	}
	return products, nil
}
