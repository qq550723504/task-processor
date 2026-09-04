package shein

import "encoding/json"

// ClonePackageForPersistence copies a SHEIN package using its persisted JSON
// contract while preserving owner-private resolution state. Semantic aliases
// are normalized on a shallow copy so cloning never mutates the source.
func ClonePackageForPersistence(pkg *Package) (*Package, error) {
	if pkg == nil {
		return nil, nil
	}

	normalized := *pkg
	NormalizePackageSemanticFields(&normalized)
	encoded, err := json.Marshal(&normalized)
	if err != nil {
		return nil, err
	}
	var cloned Package
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil, err
	}
	if pkg.SaleAttributeResolution != nil {
		if cloned.SaleAttributeResolution == nil {
			cloned.SaleAttributeResolution = &SaleAttributeResolution{}
		}
		cloned.SaleAttributeResolution.skcAssignments = cloneResolvedSaleAttributeMap(pkg.SaleAttributeResolution.skcAssignments)
		cloned.SaleAttributeResolution.skuAssignments = cloneResolvedSaleAttributeSliceMap(pkg.SaleAttributeResolution.skuAssignments)
		cloned.SaleAttributeResolution.skcValueAssignments = cloneResolvedSaleAttributeMap(pkg.SaleAttributeResolution.skcValueAssignments)
		cloned.SaleAttributeResolution.skuValueAssignments = cloneResolvedSaleAttributeMap(pkg.SaleAttributeResolution.skuValueAssignments)
	}
	return &cloned, nil
}
