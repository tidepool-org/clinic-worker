package types

// NewSlice is a helper to instantiate slices of anonymous types easily
func NewSlice[T any](_ []T, l int) []T {
	result := make([]T, l)
	return result
}

// NewSliceItem is a helper to instantiate an item of anonymous slices easily
func NewSliceItem[T any](_ []T) (result T) {
	return
}
