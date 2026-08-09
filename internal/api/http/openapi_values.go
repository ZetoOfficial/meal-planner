package httpapi

func float64Pointer(value *float32) *float64 {
	if value == nil {
		return nil
	}
	converted := float64(*value)
	return &converted
}

func float32Pointer(value *float64) *float32 {
	if value == nil {
		return nil
	}
	converted := float32(*value)
	return &converted
}

func boolPointer(value bool) *bool       { return &value }
func stringPointer(value string) *string { return &value }
