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

// Return true if the given list of strings (haystack) contains the given string (needle)
func ListContains(needle string, haystack []string) bool {
	for _, str := range haystack {
		if needle == str {
			return true
		}
	}

	return false
}