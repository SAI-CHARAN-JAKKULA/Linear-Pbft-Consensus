package shared

// GetServerNameByViewNumber returns the server name (e.g., S1, S2, ..., S7) based on the given view number.
func GetServerNameWithViewNumber(viewNumber int) string {
	serverNames := []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7"}
	index := (viewNumber - 1) % len(serverNames)
	return serverNames[index]
}
