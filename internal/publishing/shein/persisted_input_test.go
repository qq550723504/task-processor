package shein

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDecodePersistedPackageStrict(t *testing.T) {
	for name, raw := range map[string]string{
		"unknown": `{"unrecognized":1}`, "nested unknown": `{"pricing":{"surprise":1}}`,
		"sdk unknown": `{"preview_payload":{"surprise":1}}`, "duplicate": `{"spu_name":"a","spu_name":"b"}`,
		"escaped duplicate": `{"spu_name":"a","spu_\u006eame":"b"}`, "map duplicate": `{"metadata":{"x":"a","x":"b"}}`,
		"case": `{"Spu_Name":"a"}`, "null": `null`, "array": `[]`, "trailing": `{} {}`,
		"surrogate": `{"spu_name":"\ud800"}`, "low surrogate": `{"spu_name":"\udc00"}`,
		"utf8": "{\"spu_name\":\"\xff\"}", "overflow": `{"pricing":{"rule_snapshot":{"exchange_rate":1e999}}}`,
		"alias conflict":  `{"preview_payload":{"spu_name":"a"},"preview_product":{"spu_name":"b"}}`,
		"float underflow": `{"pricing":{"rule_snapshot":{"exchange_rate":1e-999}}}`,
		"depth":           `{"metadata":` + strings.Repeat(`[`, 64) + `0` + strings.Repeat(`]`, 64) + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			p, err := DecodePersistedPackageStrict([]byte(raw))
			if err == nil || p != nil {
				t.Fatalf("accepted %s: %+v %v", name, p, err)
			}
		})
	}
	if p, err := DecodePersistedPackageStrict(bytes.Repeat([]byte(" "), MaxPersistedPackageBytes+1)); err == nil || p != nil {
		t.Fatal("accepted oversized")
	}
	p, err := DecodePersistedPackageStrict([]byte(`{"spu_name":"\ud83d\ude00","metadata":{"slash":"\/","literal":"\\ud800"}}`))
	if err != nil || p.SpuName != "😀" {
		t.Fatalf("valid Unicode rejected: %v", err)
	}
}

func TestPersistedPackageBoundaryLimitsAndPrivateState(t *testing.T) {
	raw := []byte(`{"metadata":{"x":"` + strings.Repeat("x", MaxPersistedPackageBytes-len(`{"metadata":{"x":""}}`)) + `"}}`)
	if len(raw) != MaxPersistedPackageBytes {
		t.Fatal("fixture size")
	}
	if _, err := DecodePersistedPackageStrict(raw); err != nil {
		t.Fatal("boundary package rejected", err)
	}
	if _, err := DecodePersistedPackageStrict(append(raw, ' ')); err == nil {
		t.Fatal("oversize accepted")
	}
	for _, depth := range []int{64, 65} {
		raw := []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
		if validPersistedSyntaxBounds(raw) != (depth == 64) {
			t.Fatal("depth limit", depth)
		}
	}
	p, err := DecodePersistedPackageStrict([]byte(`{"sale_attribute_resolution":{"skc_value_assignments":{"a":{"value":"v"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	r := p.SaleAttributeResolution
	if len(r.SKCValueAssignments) != 1 || r.skcAssignments != nil || r.skuAssignments != nil || r.skcValueAssignments != nil || r.skuValueAssignments != nil {
		t.Fatal("private resolver state injected")
	}
}

// A new nested custom decoder could silently bypass the strict library. Keep
// reachable custom decoding explicit instead of claiming DisallowUnknownFields
// automatically penetrates UnmarshalJSON methods.
func TestPersistedPackageCustomDecodersAreAudited(t *testing.T) {
	unmarshaler := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	seen := map[reflect.Type]bool{}
	var inspect func(reflect.Type)
	inspect = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		if typ.Kind() != reflect.Pointer && reflect.PointerTo(typ).Implements(unmarshaler) {
			if typ == reflect.TypeOf(time.Time{}) {
				return
			}
			if typ != reflect.TypeOf(Package{}) {
				t.Errorf("unaudited custom decoder: %s", typ)
			}
		}
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			inspect(typ.Elem())
		case reflect.Map:
			inspect(typ.Key())
			inspect(typ.Elem())
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if f.IsExported() {
					inspect(f.Type)
				}
			}
		}
	}
	inspect(reflect.TypeOf(Package{}))
}
