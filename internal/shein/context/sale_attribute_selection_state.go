package context

// SaleAttributeSelectionState is the narrow runtime handoff DTO stored on
// TaskContext. It keeps only the sale-attribute selection fields downstream
// SHEIN pipeline steps need, without importing publishing packages.
type SaleAttributeSelectionState struct {
	Source                   string `json:"source,omitempty"`
	PrimaryAttributeID       int    `json:"primary_attribute_id,omitempty"`
	SecondaryAttributeID     int    `json:"secondary_attribute_id,omitempty"`
	PrimarySourceDimension   string `json:"primary_source_dimension,omitempty"`
	SecondarySourceDimension string `json:"secondary_source_dimension,omitempty"`
}
