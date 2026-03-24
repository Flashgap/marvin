package option

// Package option contains helpers for Golang Functional Options pattern.

// Option represents a generic option.
type Option[T any] func(o *T)

// New reads all options and returns an initialized option object.
func New[T any](opts []Option[T]) T {
	var o T
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
