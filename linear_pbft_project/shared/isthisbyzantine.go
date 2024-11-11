package shared

// IsByzantine checks if a server is in the list of Byzantine (faulty) servers
// Parameters:
//   - byzantineServerList: slice of server names known to be Byzantine
//   - serverName: name of the server to check
//
// Returns:
//   - true if the server is Byzantine, false otherwise
func IsByzantine(byzantineServerList []string, serverName string) bool {
	// Iterate through the list of Byzantine servers
	for _, byzantineServer := range byzantineServerList {
		// If we find a match, this server is Byzantine
		if byzantineServer == serverName {
			return true
		}
	}
	// Server was not found in Byzantine list
	return false
}
