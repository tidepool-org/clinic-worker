package types

func NewSlice[T any](_ []T, l int) []T {
	result := make([]T, l)
	return result
}

func NewSlicePtr[T any](_ *[]T, l int) *[]T {
	result := make([]T, l)
	return &result
}

func NewItemForSlice[T any](_ []T) (result T) {
	return
}
func NewItemPtrForSlice[T any, P *T](_ []P) P {
	return P(new(T))
}

