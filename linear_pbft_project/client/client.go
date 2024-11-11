package client

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"linear_pbft_project/shared"
	"net/rpc"
)

var privateKey *rsa.PrivateKey
var publicKey *rsa.PublicKey

func init() {
	var err error
	// Generate an RSA private key for signing transactions
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Error generating RSA private key:", err)
	}
	// Extract the public key from the private key
	publicKey = &privateKey.PublicKey
}

func ConnectToServer(serverName string) *rpc.Client {
	serverAddress := shared.ServerAddresses[serverName]
	client, err := rpc.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Printf("Error connecting to server %s at %s: %v\n", serverName, serverAddress, err)
		return nil
	}
	return client
}

func SendTransaction(serverName string, tx shared.Transaction) {
	// Create a hash of the transaction to sign
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d", tx.Source, tx.Destination, tx.Amount)))

	// Sign the hash using the RSA private key
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, txHash[:])
	if err != nil {
		fmt.Println("Error signing the transaction:", err)
		return
	}
	tx.Signature = signature

	client := ConnectToServer(serverName)
	if client == nil {
		fmt.Println("Failed to connect to server, transaction aborted.")
		return
	}
	defer client.Close()

	// Send the transaction along with the public key
	var reply string
	err = client.Call(fmt.Sprintf("Server.%s.HandleTransaction", serverName), struct {
		Transaction shared.Transaction
		PublicKey   *rsa.PublicKey
	}{tx, publicKey}, &reply)
	if err != nil {
		fmt.Println("Transaction error:", err)
	}
}

func GetTransactionStatus(serverName string, txID string) (string, string) {
	client := ConnectToServer(serverName)
	if client == nil {
		fmt.Println("Failed to connect to server, unable to wait for response.")
		return "", "failed"
	}
	defer client.Close()

	var status string
	err := client.Call(fmt.Sprintf("Server.%s.GetTransactionStatus", serverName), txID, &status)
	if err != nil {
		fmt.Println("Error waiting for response from server:", err)
		return "", "failed"
	}

	return status, "success"
}

func IsInViewChange(serverName string) (bool, string) {
	client := ConnectToServer(serverName)
	if client == nil {
		fmt.Println("Failed to connect to server, unable to get view change status.")
		return false, "failed"
	}
	defer client.Close()

	var status bool
	err := client.Call(fmt.Sprintf("Server.%s.IsInViewChange", serverName), serverName, &status)
	if err != nil {
		fmt.Println("Error getting view change status from server:", err)
		return false, "failed"
	}

	return status, "success"
}
