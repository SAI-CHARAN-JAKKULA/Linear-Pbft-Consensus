package shared

// Transaction represents a transaction between two servers
type Transaction struct {
	TransactionID       string   // Unique transaction ID
	Source              string   // The source server (e.g., S1, S2, etc.)
	Destination         string   // The destination server
	Amount              int      // The amount of the transaction
	Status              string   // Status of the transaction (e.g., "pending", "committed","local","failed")
	ActiveServerList    []string // List of active servers involved in the transaction
	ByzantineServerList []string // List of byzantine servers involved in the transaction
	SequenceNumber      int      // Sequence number for the transaction
	ViewNumber          int
	Signature           []byte // Digital signature of the transaction
}
