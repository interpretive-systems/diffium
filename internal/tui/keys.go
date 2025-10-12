package tui

// contains checks if a key string is in a slice of key bindings
func contains(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
