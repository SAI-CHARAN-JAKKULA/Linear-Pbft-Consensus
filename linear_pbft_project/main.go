package main

import (
	"fmt"
	"linear_pbft_project/client"
	"linear_pbft_project/csvparser"
	"linear_pbft_project/server"
	"linear_pbft_project/shared"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TransactionPerformance struct {
	TransactionID string
	StartTime     time.Time
	EndTime       time.Time
}

func main() {
	// Start RPC servers and store server instances
	s1 := server.StartServerRPC("S1", shared.ServerAddresses["S1"])
	s2 := server.StartServerRPC("S2", shared.ServerAddresses["S2"])
	s3 := server.StartServerRPC("S3", shared.ServerAddresses["S3"])
	s4 := server.StartServerRPC("S4", shared.ServerAddresses["S4"])
	s5 := server.StartServerRPC("S5", shared.ServerAddresses["S5"])
	s6 := server.StartServerRPC("S6", shared.ServerAddresses["S6"])
	s7 := server.StartServerRPC("S7", shared.ServerAddresses["S7"])

	fmt.Println("Starting Linear PBFT Consensus Protocol...")
	sets, err := csvparser.ParseCSV("lab2_test_cases.csv")
	if err != nil {
		log.Fatalf("Failed to parse CSV: %v", err)
	}

	// Ask user for set number
	var setNumber int
	fmt.Print("Enter the set number to process (1-10): ")
	fmt.Scan(&setNumber)

	if setNumber < 1 || setNumber > 10 {
		log.Fatalf("Invalid set number: %d. Please enter a value between 1 and 10.", setNumber)
	}

	// Fetch transactions from the selected set
	if len(sets) < setNumber {
		log.Fatalf("Set %d not found in the CSV data", setNumber)
	}
	set := sets[setNumber-1]
	transactions := []shared.Transaction{}

	// Convert csvparser transactions to shared transactions
	for _, tx := range set.Transactions {
		transaction := shared.Transaction{
			TransactionID:       uuid.New().String(),
			Source:              tx.Source,
			Destination:         tx.Destination,
			Amount:              tx.Amount,
			Status:              "client-request", // Default status
			ActiveServerList:    set.ActiveServerList,
			ByzantineServerList: set.ByzantineServerList,
			SequenceNumber:      0, // Initialize sequence number to zero
			ViewNumber:          1, // Initialize view number to 1
		}
		transactions = append(transactions, transaction)
	}

	// Order transactions by Source
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Source < transactions[j].Source
	})

	// Send transactions to servers, maintaining order for same Source
	var wg sync.WaitGroup
	sourceGroups := make(map[string][]shared.Transaction)

	// Group transactions by Source
	for _, tx := range transactions {
		sourceGroups[tx.Source] = append(sourceGroups[tx.Source], tx)
	}

	var performanceData []TransactionPerformance
	var perfMutex sync.Mutex

	startTime := time.Now() // Start time for performance measurement

	// Send transactions concurrently by group, ensuring transactions within each group are sent sequentially
	for _, txGroup := range sourceGroups {
		wg.Add(1)
		go func(group []shared.Transaction) {
			defer wg.Done()
			for _, tx := range group {
				// Record start time
				txStartTime := time.Now()

				// Send transaction to Server S1
				client.SendTransaction("S1", tx)
				time.Sleep(500 * time.Millisecond) // Wait for 0.5 seconds before checking responses again
				responses := make(map[string]string)
				for _, serverName := range tx.ActiveServerList {
					status, response := client.GetTransactionStatus(serverName, tx.TransactionID)
					responses[serverName] = status
					if response == "failed" {
						fmt.Printf("Failed to receive response from server %s for transaction %s\n", serverName, tx.TransactionID)
					}
				}
				successCount := 0
				for _, status := range responses {
					if status == "executed" {
						successCount++
						if successCount >= 3 { // f+1 replies
							fmt.Printf("At client tx %s->%s:%d received %d responses, with status executed\n", tx.Source, tx.Destination, tx.Amount, successCount)
							break
						}
					}
				}
				if successCount < 3 {
					// Retry sending to all active servers except S1
					fmt.Printf("At client tx %s->%s:%d received %d responses, with status executed\n", tx.Source, tx.Destination, tx.Amount, successCount)
					tx.Status = "resent-client-request"
					for _, serverName := range tx.ActiveServerList {
						if serverName != "S1" {
							isInViewChange, _ := client.IsInViewChange(serverName)
							if !isInViewChange {
								go client.SendTransaction(serverName, tx)
							}
						}
					}
					time.Sleep(1100 * time.Millisecond) // Wait for 1 second before checking responses again
					resentResponses := make(map[string]string)
					for _, serverName := range tx.ActiveServerList {
						status, response := client.GetTransactionStatus(serverName, tx.TransactionID)
						resentResponses[serverName] = status
						if response == "failed" {
							fmt.Printf("Failed to receive response from server %s for transaction %s\n", serverName, tx.TransactionID)
						}
					}
					resentsuccessCount := 0
					for _, status := range resentResponses {
						if status == "executed" {
							resentsuccessCount++
							if resentsuccessCount >= 3 { // f+1 replies
								fmt.Printf("At client tx %s->%s:%d received %d responses, with status executed after view change\n", tx.Source, tx.Destination, tx.Amount, resentsuccessCount)
								break
							}
						}
					}
					// TODO: If quorum is not reached, do not send transactions and do not send view changes
				}
				if len(set.ActiveServerList)-len(set.ByzantineServerList) < 5 {
					break
				}

				// Record end time
				txEndTime := time.Now()

				// Store performance data
				perfMutex.Lock()
				performanceData = append(performanceData, TransactionPerformance{
					TransactionID: tx.TransactionID,
					StartTime:     txStartTime,
					EndTime:       txEndTime,
				})
				perfMutex.Unlock()
			}
		}(txGroup)
	}

	wg.Wait()
	endTime := time.Now() // End time for performance measurement
	fmt.Printf("All transactions from Set %d have been processed.\n", setNumber)

	// Main menu for user interaction
	servers := []*server.Server{s1, s2, s3, s4, s5, s6, s7}
	for {
		fmt.Println("Select an option:")
		fmt.Println("1. PrintLog")
		fmt.Println("2. PrintDB")
		fmt.Println("3. PrintStatus")
		fmt.Println("4. PrintView")
		fmt.Println("5. PrintPerformance")
		fmt.Println("6. Exit")
		fmt.Print("Enter your choice: ")
		var choice int
		fmt.Scan(&choice)
		switch choice {
		case 1:
			printLog(servers)
		case 2:
			printDB(servers)
		case 3:
			fmt.Print("Enter sequence number: ")
			var seqNum int
			fmt.Scan(&seqNum)
			printStatus(servers, seqNum)
		case 4:
			printView(servers)
		case 5:
			perfMutex.Lock()
			printPerformance(performanceData, startTime, endTime)
			perfMutex.Unlock()
		case 6:
			fmt.Println("Exiting program.")
			return
		default:
			fmt.Println("Invalid choice. Please select an option between 1 and 6.")
		}
	}
}

// printLog prints the transaction logs of all servers
func printLog(servers []*server.Server) {
	for _, s := range servers {
		fmt.Printf("Server %s Transactions Log:\n", s.Name)
		s.Mutex.Lock()
		for _, tx := range s.Transactions {
			fmt.Printf("Transaction ID: %s, Source: %s, Destination: %s, Amount: %d, Status: %s, SeqNo: %d, ViewNo: %d\n",
				tx.TransactionID, tx.Source, tx.Destination, tx.Amount, tx.Status, tx.SequenceNumber, tx.ViewNumber)
		}
		s.Mutex.Unlock()
		fmt.Println()
	}
}

// printDB prints the current datastore (balances) of all servers
func printDB(servers []*server.Server) {
	for _, s := range servers {
		fmt.Printf("Server %s Datastore (Balances):\n", s.Name)
		s.Mutex.Lock()
		// Collect the balances into a slice and sort them
		type AccountBalance struct {
			Account string
			Balance int
		}
		balances := []AccountBalance{}
		for account, balance := range s.Balances {
			balances = append(balances, AccountBalance{Account: account, Balance: balance})
		}
		// Sort balances by account name (A to J)
		sort.Slice(balances, func(i, j int) bool {
			return balances[i].Account < balances[j].Account
		})
		for _, ab := range balances {
			fmt.Printf("Account: %s, Balance: %d\n", ab.Account, ab.Balance)
		}
		s.Mutex.Unlock()
		fmt.Println()
	}
}

// printStatus prints the status of a transaction with a given sequence number at each server
func printStatus(servers []*server.Server, sequenceNumber int) {
	fmt.Printf("Status of transaction with Sequence Number %d at each server:\n", sequenceNumber)
	for _, s := range servers {
		s.Mutex.Lock()

		// Initialize to the lowest status label "X" (No Status)
		statusLabel := "X"
		latestStatusOrder := -1 // Track the most recent status in terms of progression

		// Status progression order
		statusOrder := map[string]int{
			"pre-prepare": 1,
			"prepared":    2,
			"committed":   3,
			"executed":    4,
		}

		// Iterate over all transactions and find the most recent status for the given sequence number
		for _, tx := range s.Transactions {
			if tx.SequenceNumber == sequenceNumber {
				currentOrder := statusOrder[tx.Status]

				// Update if we find a later status
				if currentOrder > latestStatusOrder {
					latestStatusOrder = currentOrder
					switch tx.Status {
					case "pre-prepare":
						statusLabel = "PP"
					case "prepared":
						statusLabel = "P"
					case "committed":
						statusLabel = "E"
					case "executed":
						statusLabel = "E"
					}
				}
			}
		}

		s.Mutex.Unlock()
		fmt.Printf("Server %s: %s\n", s.Name, statusLabel)
	}
	fmt.Println()
}

// printView prints all view-change and new-view messages exchanged since the start of the test case
func printView(servers []*server.Server) {
	fmt.Println("View-change and New-view messages exchanged since the start of the test case:")
	for _, s := range servers {
		s.Mutex.Lock()
		if len(s.ViewChangeList) > 0 {
			fmt.Printf("Server %s ViewChangeList:\n", s.Name)
			fmt.Println()
			for _, vc := range s.ViewChangeList {
				if vc.Status == "new-view" || vc.Status == "view-change" {
					fmt.Printf("Status: %s, ViewNumber: %d, ReplicaID: %s,\n",
						vc.Status, vc.ViewNumber, vc.ReplicaID)
					fmt.Printf("List of pre-prepared transactions:\n")
					for _, tx := range vc.Transactions {
						if vc.Status == "new-view" {
							fmt.Printf("TX-ID: %s, TX: %s->%s:%d, Status: %s, SeqNo: %d, ViewNo: %d\n",
								tx.TransactionID, tx.Source, tx.Destination, tx.Amount, tx.Status, tx.SequenceNumber, vc.ViewNumber)
						} else {
							fmt.Printf("TX-ID: %s, TX: %s->%s:%d, Status: %s, SeqNo: %d, ViewNo: %d\n",
								tx.TransactionID, tx.Source, tx.Destination, tx.Amount, tx.Status, tx.SequenceNumber, tx.ViewNumber)
						}
					}
					fmt.Println()
				}
			}
			fmt.Println()
			fmt.Println()
		}
		s.Mutex.Unlock()
	}
}

// printPerformance prints throughput and latency based on performance data
func printPerformance(performanceData []TransactionPerformance, startTime, endTime time.Time) {
	totalTransactions := len(performanceData)
	if totalTransactions == 0 {
		fmt.Println("No transactions to calculate performance.")
		return
	}
	totalTime := endTime.Sub(startTime).Seconds()
	throughput := float64(totalTransactions) / totalTime

	// Calculate average latency
	var totalLatency float64
	for _, p := range performanceData {
		latency := p.EndTime.Sub(p.StartTime).Seconds()
		totalLatency += latency
	}
	averageLatency := totalLatency / float64(totalTransactions)

	fmt.Printf("Performance Metrics:\n")
	fmt.Printf("Total Transactions: %d\n", totalTransactions)
	fmt.Printf("Total Time: %.2f seconds\n", totalTime)
	fmt.Printf("Throughput: %.2f transactions/second\n", throughput)
	fmt.Printf("Average Latency: %.2f seconds\n", averageLatency)
}
