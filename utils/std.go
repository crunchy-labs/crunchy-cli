package utils

func ElementInSlice[T comparable](check T, toCheck []T) bool {
	for _, item := range toCheck {
		if check == item {
			return true
		}
	}
	return false
}
