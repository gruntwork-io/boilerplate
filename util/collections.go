package util

// Merge all the maps into one. Sadly, Go has no generics, so this is only defined for string maps.
func MergeMaps(maps ... map[string]string) map[string]string {
	out := map[string]string{}

	for _, currMap := range maps {
		for key, value := range currMap {
			out[key] = value
		}
	}

	return out
}