package utils

// ToPtr takes anything and returns a pointer to it.
func ToPtr[T any](v T) *T {
	return &v
}
