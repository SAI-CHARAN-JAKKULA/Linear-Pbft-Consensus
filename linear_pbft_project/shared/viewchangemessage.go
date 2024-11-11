package shared

type ViewChangeMessage struct {
	Transactions        []Transaction
	ViewNumber          int
	Signature           []byte
	ReplicaID           string
	ActiveServerList    []string
	ByzantineServerList []string
	Status              string
}
