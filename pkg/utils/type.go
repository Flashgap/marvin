package utils

// KV is a shortcut for map[string]interface{}
type KV map[string]any

// Ptr returns a pointer to the type in parameter
func Ptr[T any](t T) *T {
	return &t
}

// SafeVal returns the value of the pointer or an empty T if the pointer is nil
func SafeVal[T any](s *T) T {
	if s == nil {
		return *new(T)
	}
	return *s
}

// NilIfZero returns val pointer or nil if the value is equal to type zero value
func NilIfZero[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}
	return &val
}
