// Package customdata parses bounded specialist uploads. XLSX safety and Product
// validation are owned by the existing Collection parser and envelope contract.
package customdata

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"task-processor/internal/dataservice"
	"task-processor/internal/httproute"
	"task-processor/internal/product/collection"
)

func ParseDelivery(ctx context.Context, format string, raw []byte) ([]collection.OwnProduct, error) {
	if len(raw) == 0 || len(raw) > collection.MaxPayloadBytes {
		return nil, dataservice.ErrInvalid
	}
	var rows []collection.OwnProduct
	switch format {
	case "xlsx":
		var err error
		rows, err = collection.ParseImport(ctx, raw)
		if err != nil {
			return nil, dataservice.ErrInvalid
		}
	case "json":
		if httproute.DecodeJSON(raw, &rows, collection.MaxPayloadBytes, true) != nil {
			return nil, dataservice.ErrInvalid
		}
	case "csv":
		reader := csv.NewReader(bytes.NewReader(raw))
		reader.FieldsPerRecord = len(collection.ImportHeaders)
		header, err := reader.Read()
		if err != nil {
			return nil, dataservice.ErrInvalid
		}
		for i, h := range header {
			if h != collection.ImportHeaders[i] {
				return nil, dataservice.ErrInvalid
			}
		}
		for line := 0; line <= 200; line++ {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil || line == 200 {
				return nil, dataservice.ErrInvalid
			}
			product := collection.OwnProduct{Title: record[0], Description: record[1], Brand: record[2], Images: []string{}}
			for _, image := range strings.FieldsFunc(record[3], func(r rune) bool { return r == '|' || r == '\n' }) {
				product.Images = append(product.Images, strings.TrimSpace(image))
			}
			if record[4] != "" {
				price, err := strconv.ParseFloat(record[6], 64)
				stock, stockErr := strconv.Atoi(record[7])
				if err != nil || stockErr != nil {
					return nil, dataservice.ErrInvalid
				}
				product.Variants = []collection.OwnVariant{{SourceID: "row-" + strconv.Itoa(line+2), Title: product.Title, SKU: record[4], Currency: record[5], Price: price, Stock: stock, Attributes: map[string]string{}}}
			} else if record[5] != "" || record[6] != "" || record[7] != "" {
				return nil, dataservice.ErrInvalid
			}
			rows = append(rows, product)
		}
	default:
		return nil, dataservice.ErrInvalid
	}
	if len(rows) < 1 || len(rows) > 200 {
		return nil, dataservice.ErrInvalid
	}
	for i, p := range rows {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, err := collection.OwnEnvelope(collection.StableID("custom-preview", strconv.Itoa(i)), p); err != nil {
			return nil, dataservice.ErrInvalid
		}
	}
	encoded, err := json.Marshal(rows)
	if err != nil || len(encoded) > collection.MaxPayloadBytes {
		return nil, dataservice.ErrInvalid
	}
	if err := dataservice.ValidateCustomProducts(rows); err != nil {
		return nil, err
	}
	return rows, nil
}
