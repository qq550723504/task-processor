package collection

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"testing"
)

func importWorkbook(t *testing.T, change func(*excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	for i, h := range ImportHeaders {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		require.NoError(t, f.SetCellValue("Sheet1", c, h))
	}
	require.NoError(t, f.SetCellValue("Sheet1", "A2", "原始商品"))
	if change != nil {
		change(f)
	}
	b, err := f.WriteToBuffer()
	require.NoError(t, err)
	return b.Bytes()
}
func TestImportReadsEntireSheetAndRejectsDangerousContent(t *testing.T) {
	p, err := ParseImport(context.Background(), importWorkbook(t, nil))
	require.NoError(t, err)
	require.Len(t, p, 1)
	require.Empty(t, p[0].Variants)
	cases := map[string]func(*excelize.File){
		"hidden formula": func(f *excelize.File) {
			require.NoError(t, f.SetCellFormula("Sheet1", "B3", "1+1"))
			require.NoError(t, f.SetRowVisible("Sheet1", 3, false))
		},
		"sparse row":             func(f *excelize.File) { require.NoError(t, f.SetCellValue("Sheet1", "A1000000", "超限")) },
		"sparse column":          func(f *excelize.File) { require.NoError(t, f.SetCellValue("Sheet1", "XFD3", "超限")) },
		"missing declared stock": func(f *excelize.File) { require.NoError(t, f.SetCellValue("Sheet1", "E2", "SKU-1")) },
		"extra sheet":            func(f *excelize.File) { _, err := f.NewSheet("hidden"); require.NoError(t, err) },
		"unknown header":         func(f *excelize.File) { require.NoError(t, f.SetCellValue("Sheet1", "H1", "未知字段")) },
		"external link": func(f *excelize.File) {
			require.NoError(t, f.SetCellHyperLink("Sheet1", "B3", "https://external.example", "External"))
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseImport(context.Background(), importWorkbook(t, change))
			require.ErrorIs(t, err, ErrInvalid)
		})
	}
	p, err = ParseImport(context.Background(), importWorkbook(t, func(f *excelize.File) {
		for c, v := range map[string]any{"E2": "SKU-1", "F2": "USD", "G2": 0, "H2": 0} {
			require.NoError(t, f.SetCellValue("Sheet1", c, v))
		}
	}))
	require.NoError(t, err)
	require.Equal(t, 0, p[0].Variants[0].Stock)
}
