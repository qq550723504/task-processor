package recordpersistence

// RuntimeTableNames names this adapter's durable record and operation tables.
// A composition can verify a narrow database role without taking ownership of
// the adapter's schema.
func RuntimeTableNames() []string {
	return []string{"listing_shein_records", "listing_shein_record_operations"}
}
