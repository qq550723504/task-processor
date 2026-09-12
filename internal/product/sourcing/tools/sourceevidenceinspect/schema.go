package sourceevidenceinspect

import "encoding/json"

var inputSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "product_key",
    "catalog_version"
  ],
  "properties": {
    "product_key": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
    },
    "catalog_version": {
      "type": "string",
      "pattern": "^[1-9][0-9]{0,18}$"
    }
  },
  "$schema": "https://json-schema.org/draft/2020-12/schema"
}`)
var outputSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "product_key",
    "catalog_version",
    "publication_id",
    "catalog_publication_id",
    "producer",
    "published_at",
    "source_identity",
    "lineage",
    "capture",
    "warnings",
    "missing_facts",
    "disclosure"
  ],
  "properties": {
    "product_key": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
    },
    "catalog_version": {
      "type": "string",
      "pattern": "^[1-9][0-9]{0,18}$"
    },
    "publication_id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
    },
    "catalog_publication_id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
    },
    "producer": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "kind",
        "version"
      ],
      "properties": {
        "kind": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128,
          "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
        },
        "version": {
          "type": "string",
          "minLength": 1,
          "maxLength": 128,
          "pattern": "^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"
        }
      }
    },
    "published_at": {
      "type": "string",
      "format": "date-time",
      "maxLength": 40
    },
    "source_identity": {
      "type": "object",
      "additionalProperties": false,
      "required": [],
      "properties": {
        "source_type": {
          "const": "crawler"
        },
        "source_platform": {
          "const": "1688"
        },
        "source_id": {
          "type": "string",
          "pattern": "^[1-9][0-9]{0,31}$"
        },
        "source_fingerprint": {
          "type": "string",
          "pattern": "^[a-f0-9]{64}$"
        }
      }
    },
    "lineage": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "input_hash",
        "envelope_hash",
        "snapshot_hash"
      ],
      "properties": {
        "input_hash": {
          "type": "string",
          "pattern": "^[a-f0-9]{64}$"
        },
        "envelope_hash": {
          "type": "string",
          "pattern": "^[a-f0-9]{64}$"
        },
        "snapshot_hash": {
          "type": "string",
          "pattern": "^[a-f0-9]{64}$"
        }
      }
    },
    "capture": {
      "type": "object",
      "additionalProperties": false,
      "required": [],
      "properties": {
        "captured_at": {
          "type": "string",
          "format": "date-time",
          "maxLength": 40
        },
        "checksum": {
          "type": "string",
          "pattern": "^sha256:[a-f0-9]{64}$"
        }
      }
    },
    "warnings": {
      "type": "array",
      "maxItems": 256,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "kind"
        ],
        "properties": {
          "kind": {
            "const": "source_warning"
          },
          "field": {
            "type": "string",
            "enum": [
              "title",
              "images",
              "brand",
              "description",
              "category_path",
              "attributes",
              "variants",
              "price",
              "stock",
              "currency"
            ]
          }
        }
      }
    },
    "missing_facts": {
      "type": "array",
      "maxItems": 256,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "field"
        ],
        "properties": {
          "field": {
            "type": "string",
            "enum": [
              "title",
              "images",
              "brand",
              "description",
              "category_path",
              "attributes",
              "variants",
              "price",
              "stock",
              "currency"
            ]
          }
        }
      }
    },
    "disclosure": {
      "type": "object",
      "additionalProperties": false,
      "required": [
        "raw_text_returned",
        "source_url_provided",
        "diagnostics_partial",
        "warning_codes_omitted",
        "warnings_total",
        "missing_facts_total",
        "warning_fields_omitted",
        "missing_facts_omitted",
        "identity_fields_omitted",
        "capture_fields_omitted"
      ],
      "properties": {
        "raw_text_returned": {
          "const": false
        },
        "source_url_provided": {
          "const": false
        },
        "diagnostics_partial": {
          "type": "boolean"
        },
        "warning_codes_omitted": {
          "type": "integer",
          "minimum": 0,
          "maximum": 256
        },
        "warnings_total": {
          "type": "integer",
          "minimum": 0,
          "maximum": 256
        },
        "missing_facts_total": {
          "type": "integer",
          "minimum": 0,
          "maximum": 256
        },
        "warning_fields_omitted": {
          "type": "integer",
          "minimum": 0,
          "maximum": 256
        },
        "missing_facts_omitted": {
          "type": "integer",
          "minimum": 0,
          "maximum": 256
        },
        "identity_fields_omitted": {
          "type": "array",
          "maxItems": 4,
          "items": {
            "enum": [
              "source_type",
              "source_platform",
              "source_id",
              "source_fingerprint"
            ]
          }
        },
        "capture_fields_omitted": {
          "type": "array",
          "maxItems": 1,
          "items": {
            "enum": [
              "checksum"
            ]
          }
        }
      }
    }
  },
  "$schema": "https://json-schema.org/draft/2020-12/schema"
}`)

func InputSchema() json.RawMessage  { return append(json.RawMessage(nil), inputSchema...) }
func OutputSchema() json.RawMessage { return append(json.RawMessage(nil), outputSchema...) }
