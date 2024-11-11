package server

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"linear_pbft_project/shared"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/hashicorp/vault/shamir"
)

type Server struct {
	Name                  string
	Address               string
	Transactions          []shared.Transaction
	CommittedTransactions []shared.Transaction
	Mutex                 sync.Mutex
	Balances              map[string]int // Key is the source (A to J), value is the balance
	MaxSequenceNumber     int            // Maximum sequence number, initialized to zero
	MaxExecutedSno        int            // Maximum executed sequence number
	PrivateKey            *rsa.PrivateKey
	PublicKey             *rsa.PublicKey
	ViewNumber            int // View number, initialized to zero
	ViewChangeList        []shared.ViewChangeMessage
	IsItInViewChange      bool
	timer                 *time.Timer
}

const (
	viewChangeTimeout = 1 * time.Second // Adjust this as needed
)

func StartServerRPC(name string, address string) *Server {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error generating private key for server %s: %v\n", name, err)
		return nil
	}
	publicKey := &privateKey.PublicKey

	server := &Server{
		Name:                  name,
		Address:               address,
		Transactions:          []shared.Transaction{},
		CommittedTransactions: []shared.Transaction{},
		Balances:              map[string]int{"A": 10, "B": 10, "C": 10, "D": 10, "E": 10, "F": 10, "G": 10, "H": 10, "I": 10, "J": 10},
		MaxSequenceNumber:     0,
		MaxExecutedSno:        0,
		PrivateKey:            privateKey,
		PublicKey:             publicKey,
		ViewNumber:            1,
		ViewChangeList:        []shared.ViewChangeMessage{},
		IsItInViewChange:      false,
		timer:                 nil,
	}
	err = rpc.RegisterName(fmt.Sprintf("Server.%s", name), server)
	if err != nil {
		fmt.Printf("Error registering server %s: %v\n", name, err)
		return nil
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Printf("Error starting server %s: %v\n", name, err)
		return nil
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Printf("Error accepting connection on server %s: %v\n", name, err)
				continue
			}
			go rpc.ServeConn(conn)
		}
	}()
	fmt.Printf("Server %s started at %s\n", name, address)
	return server
}

func (s *Server) signTransaction(tx shared.Transaction) ([]byte, error) {
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d", tx.Source, tx.Destination, tx.Amount)))
	signedTx, err := rsa.SignPKCS1v15(rand.Reader, s.PrivateKey, crypto.SHA256, txHash[:])
	if err != nil {
		fmt.Printf("Error signing the transaction on server %s: %v\n", s.Name, err)
		return nil, err
	}
	return signedTx, nil
}

// HandleTransaction processes the incoming transaction and stores it
func (s *Server) HandleTransaction(args *struct {
	Transaction shared.Transaction
	PublicKey   *rsa.PublicKey
}, reply *string) error {
	s.Mutex.Lock()
	tx := args.Transaction
	publicKey := args.PublicKey

	// If signature is valid, proceed
	if tx.Status == "client-request" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.Mutex.Unlock()
			fmt.Printf("returning nill at client-request verify")
			return nil
		}
		tx.Status = "pre-prepare"
		tx.ViewNumber = s.ViewNumber
		s.MaxSequenceNumber++
		tx.SequenceNumber = s.MaxSequenceNumber
		// fmt.Printf("received client-request at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
		// appending to the list
		s.Transactions = append(s.Transactions, tx)
		// sign before calling
		signedTx, err := s.signTransaction(tx)
		if err != nil {
			*reply = "signing failed"
			s.Mutex.Unlock()
			fmt.Printf("returning nill at client-request sign")
			return nil
		}
		tx.Signature = signedTx
		// Call Pre-Prepare
		s.Mutex.Unlock()
		go s.PrePrepare(tx)
	} else if tx.Status == "pre-prepare" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.Mutex.Unlock()
			fmt.Printf("returning nill at pre-prepare verify")
			return nil
		}
		s.Transactions = append(s.Transactions, tx)
		// fmt.Printf("received pre-prepare at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			// for _, server := range tx.ByzantineServerList {
			// 	if server == s.Name {
			// 		s.Mutex.Unlock()
			// 		// fmt.Printf("returning nill as server is in ByzantineServerList")
			// 		return nil
			// 	}
			// }
			tx.Status = "prepare"
			// sign before calling
			signedTx, err := s.signTransaction(tx)
			if err != nil {
				*reply = "signing failed"
				s.Mutex.Unlock()
				fmt.Printf("returning nill at pre-prepare sign")
				return nil
			}
			tx.Signature = signedTx
			s.Mutex.Unlock()
			go s.sendFromBackupsToPrimary(tx)
		} else {
			s.Mutex.Unlock()
		}
	} else if tx.Status == "prepare" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.Mutex.Unlock()
			fmt.Printf("returning nill")
			return nil
		}
		s.Transactions = append(s.Transactions, tx)
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			// for _, server := range tx.ByzantineServerList {
			// 	if server == s.Name {
			// 		s.Mutex.Unlock()
			// 		// fmt.Printf("returning nill as server is in ByzantineServerList")
			// 		return nil
			// 	}
			// }
			// fmt.Printf("received prepare at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			// Create a list of prepare transactions
			prepareTransactions := []shared.Transaction{}
			for _, transaction := range s.Transactions {
				if transaction.TransactionID == tx.TransactionID && transaction.Status == "prepare" {
					prepareTransactions = append(prepareTransactions, transaction)
				}
			}

			// Check if enough prepare messages have been received
			if len(prepareTransactions) >= len(tx.ActiveServerList)-len(tx.ByzantineServerList)-1 && len(prepareTransactions) >= 4 {
				// fmt.Printf("received %d prepares at %s tx: %s->%s:%d\n", len(prepareTransactions), s.Name, tx.Source, tx.Destination, tx.Amount)
				// Implement Shamir's Secret Sharing to create a threshold signature from prepare transactions
				signatures := [][]byte{}
				for _, transaction := range prepareTransactions {
					signatures = append(signatures, transaction.Signature)
				}

				thresholdSignature, err := shamir.Combine(signatures) // TSS Combined Signature
				newTx := tx
				newTx.Status = "prepared"
				newTx.Signature = thresholdSignature
				s.Transactions = append(s.Transactions, newTx)
				if len(prepareTransactions) == 6 { // optimistic phase reduction // 3f+1 replies
					newTx.Status = "committed"
					s.Transactions = append(s.Transactions, newTx)
				}
				go s.sendFromPrimaryWithThresholdKey(newTx, s.PublicKey)

				if err != nil {
				}
			}
		}
		s.Mutex.Unlock()
	} else if tx.Status == "prepared" {
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.Mutex.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			// fmt.Printf("received prepared at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			tx.Status = "commit"
			// sign before calling
			signedTx, err := s.signTransaction(tx)
			if err != nil {
				*reply = "signing failed"
				s.Mutex.Unlock()
				return nil
			}
			tx.Signature = signedTx
			s.Mutex.Unlock()
			go s.sendFromBackupsToPrimary(tx)
		} else {
			s.Mutex.Unlock()
		}
	} else if tx.Status == "commit" {
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.Mutex.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			// fmt.Printf("received commit at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			// Create a list of commit transactions
			commitTransactions := []shared.Transaction{}
			for _, transaction := range s.Transactions {
				if transaction.TransactionID == tx.TransactionID && transaction.Status == "commit" {
					commitTransactions = append(commitTransactions, transaction)
				}
			}

			// Check if enough commit messages have been received
			if len(commitTransactions) >= len(tx.ActiveServerList)-len(tx.ByzantineServerList)-1 && len(commitTransactions) >= 4 {
				// Implement Shamir's Secret Sharing to create a threshold signature from commit transactions
				// fmt.Printf("received %d commits at %s tx: %s->%s:%d\n", len(commitTransactions), s.Name, tx.Source, tx.Destination, tx.Amount)
				signatures := [][]byte{}
				for _, transaction := range commitTransactions {
					signatures = append(signatures, transaction.Signature)
				}
				thresholdSignature, err := shamir.Combine(signatures)
				newTx := tx
				newTx.Status = "committed"
				newTx.Signature = thresholdSignature
				// Add committed transaction
				s.Transactions = append(s.Transactions, newTx)
				s.CommittedTransactions = append(s.CommittedTransactions, newTx)
				go s.sendFromPrimaryWithThresholdKey(newTx, s.PublicKey)
				// Execute all committed transactions in order
				count := s.MaxExecutedSno
				for {
					found := false
					for i, transaction := range s.CommittedTransactions {
						if transaction.SequenceNumber == count+1 {
							// Update balances
							if s.Balances[transaction.Source]-transaction.Amount >= 0 {
								s.Balances[transaction.Source] -= transaction.Amount
								s.Balances[transaction.Destination] += transaction.Amount
							}
							s.CommittedTransactions[i].Status = "executed"
							s.Transactions = append(s.Transactions, s.CommittedTransactions[i])
							// etx := s.CommittedTransactions[i]
							// fmt.Printf("Executed at %s tx: %s->%s:%d sqno: %d status : %s\n", s.Name, etx.Source, etx.Destination, etx.Amount, etx.SequenceNumber, etx.Status)
							// Print all balances
							// for account, balance := range s.Balances {
							// 	fmt.Printf("Account: %s, Balance: %d\n", account, balance)
							// }
							count++
							found = true
							break
						}
					}
					if !found {
						break
					}
				}
				s.MaxExecutedSno = count
				if err != nil {
				}
			}
		}
		s.Mutex.Unlock()
	} else if tx.Status == "committed" {
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			if !s.verifyPublicKey(tx, publicKey, reply) {
				s.Mutex.Unlock()
				fmt.Printf("returning nill")
				return nil
			}
			s.Transactions = append(s.Transactions, tx)
			// fmt.Printf("received committed at %s tx: %s->%s:%d sqno: %d\n", s.Name, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			s.CommittedTransactions = append(s.CommittedTransactions, tx)

			// Execute all committed transactions in order
			count := s.MaxExecutedSno
			for {
				found := false
				for i, transaction := range s.CommittedTransactions {
					if transaction.SequenceNumber == count+1 {
						// Update balances
						if s.Balances[transaction.Source]-transaction.Amount >= 0 {
							s.Balances[transaction.Source] -= transaction.Amount
							s.Balances[transaction.Destination] += transaction.Amount
						}
						s.CommittedTransactions[i].Status = "executed"
						s.Transactions = append(s.Transactions, s.CommittedTransactions[i])
						// etx := s.CommittedTransactions[i]
						// fmt.Printf("Executed at %s tx: %s->%s:%d sqno: %d status : %s\n", s.Name, etx.Source, etx.Destination, etx.Amount, etx.SequenceNumber, etx.Status)
						// // Print all balances
						// // for account, balance := range s.Balances {
						// // 	fmt.Printf("Account: %s, Balance: %d\n", account, balance)
						// // }
						count++
						found = true
						break
					}
				}
				if !found {
					break
				}
			}
			s.MaxExecutedSno = count
		}
		s.Mutex.Unlock()
	} else if tx.Status == "resent-client-request" {
		if !s.verifyPublicKey(tx, publicKey, reply) {
			s.Mutex.Unlock()
			fmt.Printf("returning nill at client-request verify")
			return nil
		}
		// fmt.Printf("server %s got resent-client-request\n", s.Name)
		s.IsItInViewChange = true
		s.ViewNumber++
		if !shared.IsByzantine(tx.ByzantineServerList, s.Name) {
			// fmt.Printf("server %s is  sending view-change becaue it is not byzantine\n", s.Name)
			s.Mutex.Unlock()
			go s.sendViewChange(tx.Signature, tx.ActiveServerList, tx.ByzantineServerList, "view-change")
		} else {
			s.Mutex.Unlock()
			// fmt.Printf("server %s is not sending view-change becaue it is byzantine\n", s.Name)
		}
	} else {
		s.Mutex.Unlock()
	}
	*reply = tx.Status
	return nil
}
func (s *Server) sendViewChange(Signature []byte, ActiveServerList []string, ByzantineServerList []string, status string) {
	for _, serverName := range ActiveServerList {
		if serverName != s.Name {
			go func(serverName string) {
				client, err := rpc.Dial("tcp", shared.ServerAddresses[serverName])
				if err != nil {
					fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
					return
				}
				defer client.Close()

				vc := shared.ViewChangeMessage{
					Transactions:        s.Transactions,
					ViewNumber:          s.ViewNumber,
					Signature:           Signature,
					ReplicaID:           s.Name,
					ActiveServerList:    ActiveServerList,
					ByzantineServerList: ByzantineServerList,
					Status:              status,
				}

				err = client.Call(fmt.Sprintf("Server.%s.HandleViewChange", serverName), &struct {
					ViewChangeMessage shared.ViewChangeMessage
					PublicKey         *rsa.PublicKey
				}{ViewChangeMessage: vc, PublicKey: s.PublicKey}, nil)
				if err != nil {
					fmt.Printf("Error sending viewchange message to server %s: %v\n", serverName, err)
				}
			}(serverName)
		}
	}
}

func (s *Server) HandleViewChange(args *struct {
	ViewChangeMessage shared.ViewChangeMessage
	PublicKey         *rsa.PublicKey
}, reply *string) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.ViewChangeList = append(s.ViewChangeList, args.ViewChangeMessage)
	if !shared.IsByzantine(args.ViewChangeMessage.ByzantineServerList, s.Name) {
		count := 0
		for _, message := range s.ViewChangeList {
			if message.ViewNumber == args.ViewChangeMessage.ViewNumber && message.Status == "view-change" {
				count++
			}
		}
		if count == len(args.ViewChangeMessage.ActiveServerList)-len(args.ViewChangeMessage.ByzantineServerList)-1 && len(args.ViewChangeMessage.ActiveServerList)-len(args.ViewChangeMessage.ByzantineServerList)-1 >= 4 {
			// fmt.Printf("received %d view change messages at %s\n", count, s.Name)
			if s.Name == shared.GetServerNameWithViewNumber(s.ViewNumber) {
				s.sendNewView(args.ViewChangeMessage.Signature, args.ViewChangeMessage.ActiveServerList, args.ViewChangeMessage.ByzantineServerList, "new-view")
				// time.Sleep(50 * time.Millisecond)
				for _, tx := range args.ViewChangeMessage.Transactions {
					tx.Status = "pre-prepare"
					tx.ViewNumber = s.ViewNumber
					s.Transactions = append(s.Transactions, tx)
					// sign before calling
					signedTx, err := s.signTransaction(tx)
					if err != nil {
						*reply = "signing failed"
						fmt.Printf("returning nill at client-request sign")
						return nil
					}
					tx.Signature = signedTx
					// Call Pre-Prepare
					go s.PrePrepare(tx)
				}
			} else {
				if s.timer != nil {
					s.timer.Stop()
				}
				// Capture necessary variables
				activeServerList := args.ViewChangeMessage.ActiveServerList
				byzantineServerList := args.ViewChangeMessage.ByzantineServerList
				signature := args.ViewChangeMessage.Signature

				s.timer = time.AfterFunc(viewChangeTimeout, func() {
					s.Mutex.Lock()
					defer s.Mutex.Unlock()
					if !s.receivedNewViewForCurrentViewNumber() {
						// Increment view number
						s.ViewNumber++
						fmt.Printf("Server %s: Timer expired, incrementing view to %d and sending view-change\n", s.Name, s.ViewNumber)
						// Clear ViewChangeList for the old view number
						// s.ViewChangeList = []shared.ViewChangeMessage{}
						s.IsItInViewChange = true
						// Send new view-change message
						go s.sendViewChange(signature, activeServerList, byzantineServerList, "view-change")
					}
				})
			}
		}
	} else {
		// fmt.Printf("not handling view change as this server %s is byzantine, but stored the logs\n", s.Name)
	}
	*reply = "success"
	return nil
}

func (s *Server) receivedNewViewForCurrentViewNumber() bool {
	for _, message := range s.ViewChangeList {
		if message.ViewNumber == s.ViewNumber && message.Status == "new-view" {
			return true
		}
	}
	return false
}

func (s *Server) sendNewView(Signature []byte, ActiveServerList []string, ByzantineServerList []string, status string) {
	for _, serverName := range ActiveServerList {
		if serverName != s.Name {
			go func(serverName string) {
				client, err := rpc.Dial("tcp", shared.ServerAddresses[serverName])
				if err != nil {
					fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
					return
				}
				defer client.Close()

				vc := shared.ViewChangeMessage{
					Transactions:        s.Transactions,
					ViewNumber:          s.ViewNumber,
					Signature:           Signature,
					ReplicaID:           s.Name,
					ActiveServerList:    ActiveServerList,
					ByzantineServerList: ByzantineServerList,
					Status:              status,
				}

				err = client.Call(fmt.Sprintf("Server.%s.HandleViewChange", serverName), &struct {
					ViewChangeMessage shared.ViewChangeMessage
					PublicKey         *rsa.PublicKey
				}{ViewChangeMessage: vc, PublicKey: s.PublicKey}, nil)
				if err != nil {
					fmt.Printf("Error sending viewchange message to server %s: %v\n", serverName, err)
				}
			}(serverName)
		}
	}

}

func (s *Server) HandleNewView(args *struct {
	ViewChangeMessage shared.ViewChangeMessage
	PublicKey         *rsa.PublicKey
}, reply *string) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	// Iterate through each transaction and update its ViewNumber
	for i := range args.ViewChangeMessage.Transactions {
		args.ViewChangeMessage.Transactions[i].ViewNumber = args.ViewChangeMessage.ViewNumber
	}
	s.ViewChangeList = append(s.ViewChangeList, args.ViewChangeMessage)
	//stop the timer
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	*reply = "success"
	return nil
}

// func (s *Server) containsNewViewMessage() bool {
// 	for _, message := range s.ViewChangeList {
// 		if message.Status == "new-view" {
// 			return true
// 		}
// 	}
// 	return false
// }

// sendPrepareToPrimary sends the transaction to the primary server during the prepare phase
func (s *Server) sendFromBackupsToPrimary(tx shared.Transaction) {
	client, err := rpc.Dial("tcp", shared.ServerAddresses[shared.GetServerNameWithViewNumber(s.ViewNumber)])
	if err != nil {
		fmt.Printf("Error connecting to primary server %s: %v\n", shared.GetServerNameWithViewNumber(s.ViewNumber), err)
		return
	}
	defer client.Close()

	var prepareReply string
	err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", shared.GetServerNameWithViewNumber(s.ViewNumber)), &struct {
		Transaction shared.Transaction
		PublicKey   *rsa.PublicKey
	}{Transaction: tx, PublicKey: s.PublicKey}, &prepareReply)
	if err != nil {
		fmt.Printf("Error sending prepare transaction to primary server %s: %v\n", shared.GetServerNameWithViewNumber(s.ViewNumber), err)
		return
	}
}

// GetTransactionStatus returns the status of a specific transaction
func (s *Server) GetTransactionStatus(txID string, reply *string) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	for _, tx := range s.Transactions {
		if tx.TransactionID == txID && tx.Status == "executed" {
			*reply = tx.Status
			return nil
		}
	}

	*reply = "failed"
	return nil
}

// / IsInViewChange returns true if there are any view-change messages in the list
func (s *Server) IsInViewChange(sname string, reply *bool) error {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	// fmt.Println("%s processed %s IsInViewChange", sname, s.Name)
	if !s.IsItInViewChange {
		s.IsItInViewChange = true
		*reply = false
	} else {
		*reply = true
	}
	return nil
}

// PrePrepare sends the transaction to all the remaining servers in the active servers list
func (s *Server) PrePrepare(tx shared.Transaction) {
	for _, serverName := range tx.ActiveServerList {
		if serverName != s.Name {
			client, err := rpc.Dial("tcp", shared.ServerAddresses[serverName])
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
				continue
			}
			defer client.Close()

			var reply string
			err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), &struct {
				Transaction shared.Transaction
				PublicKey   *rsa.PublicKey
			}{Transaction: tx, PublicKey: s.PublicKey}, &reply)
			if err != nil {
				fmt.Printf("Error sending transaction to server %s: %v\n", serverName, err)
			} else {
				// fmt.Printf("Sending Pre-Prepare from %s to %s tx: %s->%s:%d sqno: %d\n", s.Name, serverName, tx.Source, tx.Destination, tx.Amount, tx.SequenceNumber)
			}
		}
	}
}
func (s *Server) sendFromPrimaryWithThresholdKey(tx shared.Transaction, thresholdPublicKey *rsa.PublicKey) {
	for _, serverName := range tx.ActiveServerList {
		if serverName != s.Name {
			client, err := rpc.Dial("tcp", shared.ServerAddresses[serverName])
			if err != nil {
				fmt.Printf("Error connecting to server %s: %v\n", serverName, err)
				fmt.Printf("Threshold signature: %x\n", thresholdPublicKey) // Added for debugging
				continue
			}
			defer client.Close()
			var reply string
			err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), &struct {
				Transaction shared.Transaction
				PublicKey   *rsa.PublicKey
			}{Transaction: tx, PublicKey: s.PublicKey}, &reply)
			if err != nil {
				fmt.Printf("Error sending prepare transaction to server %s: %v\n", serverName, err)
			} else {
				// fmt.Printf("Prepare transaction sent to server %s, reply: %s\n", serverName, reply)
			}
		}
	}
}

// verifyPublicKey verifies the signature of a given transaction
func (s *Server) verifyPublicKey(tx shared.Transaction, publicKey *rsa.PublicKey, reply *string) bool {
	// Create a hash of the transaction to verify
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d", tx.Source, tx.Destination, tx.Amount)))

	// Verify the signature using the public key
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, txHash[:], tx.Signature)
	if err != nil {
		*reply = "invalid signature"
		return true
	}
	return true
}
