package types

// NewStructPtr instantiates a struct of the passed types and returns a pointer to it.
// Intended use is to instantiate anonymous structs more easily
func NewStructPtr[T any, P *T](_ P) P {
	result := new(T)
	return P(result)
}
