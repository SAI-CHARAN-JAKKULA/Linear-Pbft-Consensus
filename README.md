Lab - 2 Distributed Systems
# Linear PBFT Consensus Protocol

A distributed consensus system implementing the **Practical Byzantine Fault Tolerance (PBFT)** protocol with linear ordering of transactions. This project demonstrates a fault-tolerant distributed system that can reach consensus even in the presence of Byzantine (arbitrarily faulty) servers.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [PBFT Protocol Phases](#pbft-protocol-phases)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Usage](#usage)
- [How It Works](#how-it-works)
- [Test Cases](#test-cases)
- [Performance Metrics](#performance-metrics)
- [View Change Mechanism](#view-change-mechanism)
- [Security Features](#security-features)
- [Troubleshooting](#troubleshooting)

## Overview

This implementation provides a distributed consensus protocol that ensures all non-faulty servers agree on the order and execution of transactions, even when up to `f` servers are Byzantine (where `n = 3f + 1` and `n` is the total number of servers). The system maintains a distributed ledger with account balances and processes transactions in a linear, ordered fashion.

### Key Capabilities

- **Byzantine Fault Tolerance**: Handles up to `f` Byzantine servers in a system of `3f+1` servers
- **Linear Ordering**: Ensures all servers execute transactions in the same order
- **View Changes**: Automatically handles primary server failures
- **Cryptographic Security**: Uses RSA signatures and threshold signatures for message authentication
- **Performance Monitoring**: Tracks throughput and latency metrics

## Architecture

The system consists of three main components:

### 1. **Servers (Replicas)**
- **7 servers** (S1-S7) that participate in consensus
- Each server maintains:
  - Transaction log
  - Account balances (for accounts A through J)
  - View number (for primary election)
  - View change messages
- Servers communicate via RPC over TCP

### 2. **Clients**
- **10 clients** (A-J) that can initiate transactions
- Each client has an initial balance of 10 units
- Clients sign transactions using RSA cryptography

### 3. **Primary Server**
- Rotates based on view number: `(viewNumber - 1) % 7`
- Assigns sequence numbers to transactions
- Coordinates the consensus process

## Features

- **Byzantine Fault Tolerance**:** Up to `f` Byzantine servers in a `3f+1` system
- **Linear Ordering**: All servers execute transactions in the same sequence
- **View Change Protocol**: Automatic recovery from primary failures
- **Threshold Signatures**: Uses Shamir's Secret Sharing for efficient consensus
- **Optimistic Fast Path**: Reduces latency when 3f+1 prepare messages are received
- **Transaction Signing**: RSA-based digital signatures for authentication
- **Performance Metrics**: Throughput and latency tracking
- **Interactive CLI**: Menu-driven interface for system inspection

## PBFT Protocol Phases

The protocol operates in four main phases:

### 1. **Pre-Prepare Phase**
- Client sends transaction to primary server (S1 initially)
- Primary assigns a sequence number and view number
- Primary signs the transaction and broadcasts it to all backup servers
- Status: `client-request` → `pre-prepare`

### 2. **Prepare Phase**
- Backup servers receive pre-prepare message
- Each backup verifies the signature and broadcasts a prepare message
- Servers collect prepare messages from other replicas
- When `2f+1` prepare messages are received, transaction moves to "prepared" state
- Status: `pre-prepare` → `prepare` → `prepared`

### 3. **Commit Phase**
- Primary combines prepare signatures using threshold signature scheme
- Primary broadcasts "prepared" message to all servers
- Servers send commit messages
- When `2f+1` commit messages are received, transaction is "committed"
- Status: `prepared` → `commit` → `committed`

### 4. **Execute Phase**
- Committed transactions are executed in sequence number order
- Balances are updated atomically
- Status: `committed` → `executed`

## Project Structure

```
linear_pbft_project/
├── main.go                    # Entry point, orchestrates system
├── go.mod                     # Go module dependencies
├── go.sum                     # Dependency checksums
├── lab2_test_cases.csv       # Test case definitions
│
├── client/
│   └── client.go             # Client implementation with RPC calls
│
├── server/
│   └── server.go             # Server implementation with PBFT logic
│
├── csvparser/
│   └── csv_parser.go         # CSV test case parser
│
└── shared/
    ├── transaction.go        # Transaction data structure
    ├── viewchangemessage.go  # View change message structure
    ├── serveraddress.go      # Server address mappings
    ├── servernamefromview.go # Primary election logic
    └── isthisbyzantine.go    # Byzantine server detection
```

## Prerequisites

- **Go 1.23.2** or later
- **Git** (for cloning the repository)

## Installation

1. **Clone the repository** (if applicable):
   ```bash
   git clone <repository-url>
   cd pbft-saicharanjakkula
   ```

2. **Navigate to the project directory**:
   ```bash
   cd linear_pbft_project
   ```

3. **Install dependencies**:
   ```bash
   go mod download
   ```

   The project uses the following dependencies:
   - `github.com/google/uuid` - For generating unique transaction IDs
   - `github.com/hashicorp/vault/shamir` - For threshold signature schemes

## Usage

### Running the System

1. **Start the program**:
   ```bash
   go run main.go
   ```

2. **Select a test set**:
   - The program will prompt you to enter a set number (1-10)
   - Each set contains different transactions and Byzantine server configurations
   - Example: Enter `1` to run test set 1

3. **Monitor transaction processing**:
   - The system will process transactions and display status updates
   - Wait for "All transactions from Set X have been processed" message

4. **Use the interactive menu**:
   After processing completes, you'll see a menu with options:
   ```
   Select an option:
   1. PrintLog
   2. PrintDB
   3. PrintStatus
   4. PrintView
   5. PrintPerformance
   6. Exit
   ```

### Menu Options Explained

#### 1. PrintLog
Displays the complete transaction log for all servers, showing:
- Transaction ID
- Source and destination accounts
- Amount
- Current status (pre-prepare, prepared, committed, executed)
- Sequence number
- View number

#### 2. PrintDB
Shows the current account balances (datastore) for all servers, sorted alphabetically by account name (A through J).

#### 3. PrintStatus
- Prompts for a sequence number
- Shows the status of that transaction at each server
- Status codes:
  - `PP` = Pre-prepare
  - `P` = Prepared
  - `E` = Executed/Committed
  - `X` = No status (transaction not found)

#### 4. PrintView
Displays all view-change and new-view messages exchanged during the test case, including:
- View number
- Replica ID
- List of pre-prepared transactions
- Message status (view-change or new-view)

#### 5. PrintPerformance
Shows performance metrics:
- Total number of transactions processed
- Total execution time
- Throughput (transactions per second)
- Average latency per transaction

#### 6. Exit
Terminates the program.

## How It Works

### Transaction Flow

1. **Client Request**:
   - Client creates a transaction (source, destination, amount)
   - Client signs the transaction with its RSA private key
   - Client sends transaction to primary server (S1)

2. **Pre-Prepare**:
   - Primary verifies client signature
   - Primary assigns sequence number (increments `MaxSequenceNumber`)
   - Primary signs and broadcasts to all backup servers

3. **Prepare**:
   - Backup servers verify primary's signature
   - Each backup signs and sends prepare message to primary
   - Primary collects prepare messages
   - When `2f+1` prepares received, transaction is "prepared"

4. **Commit**:
   - Primary combines signatures using Shamir's Secret Sharing
   - Primary broadcasts "prepared" message with threshold signature
   - Servers send commit messages
   - When `2f+1` commits received, transaction is "committed"

5. **Execute**:
   - Committed transactions are executed in sequence number order
   - Balances updated: `source -= amount`, `destination += amount`
   - Transaction status becomes "executed"

### Byzantine Fault Handling

- Byzantine servers are identified in the test case configuration
- Non-Byzantine servers ignore messages from Byzantine servers
- The system requires `2f+1` honest servers to reach consensus
- Byzantine servers may send incorrect messages but cannot break consensus

### Sequence Number Assignment

- Primary server assigns sequence numbers sequentially
- Each transaction gets a unique sequence number
- Transactions are executed in sequence number order
- This ensures linear ordering across all servers

## Test Cases

The system includes 10 test sets defined in `lab2_test_cases.csv`. Each set contains:

- **Set Number**: Identifier (1-10)
- **Transactions**: List of (Source, Destination, Amount) tuples
- **Active Server List**: Servers participating in consensus (e.g., [S1, S2, S3, S4, S5, S6, S7])
- **Byzantine Server List**: Servers that will behave maliciously (e.g., [S4, S6])

### Test Set Examples

- **Set 1**: Basic transaction with all servers active, no Byzantine servers
- **Set 2**: Multiple transactions with 2 Byzantine servers (S4, S6)
- **Set 6**: Transactions with 3 Byzantine servers (S3, S5, S7) - maximum fault tolerance test
- **Set 10**: Large transaction set (30 transactions) with 1 Byzantine server

## Performance Metrics

The system tracks:

- **Throughput**: Transactions processed per second
- **Latency**: Average time from transaction submission to execution
- **Total Time**: End-to-end processing time for all transactions

These metrics help evaluate the system's performance under different Byzantine fault scenarios.

## View Change Mechanism

When a primary server fails or becomes unresponsive:

1. **Trigger**: Client doesn't receive `f+1` executed responses within timeout
2. **View Change Initiation**: Client resends transaction to other servers
3. **View Change Messages**: Non-Byzantine servers send view-change messages
4. **New View**: New primary (based on view number) sends new-view message
5. **Recovery**: Pending transactions are re-proposed with new view number

### View Number Rotation

Primary server selection rotates based on view number:
- View 1: S1
- View 2: S2
- View 3: S3
- ...
- View 7: S7
- View 8: S1 (wraps around)

Formula: `primary = serverNames[(viewNumber - 1) % 7]`

## Security Features

### 1. **Digital Signatures**
- All transactions are signed using RSA-2048
- Signatures are verified at each phase
- Prevents message tampering

### 2. **Threshold Signatures**
- Uses Shamir's Secret Sharing for efficient consensus
- Combines multiple signatures into a single threshold signature
- Reduces message complexity while maintaining security

### 3. **Signature Verification**
- Each server verifies signatures before processing messages
- Invalid signatures are rejected
- Ensures message authenticity

### 4. **Byzantine Detection**
- System identifies and isolates Byzantine servers
- Non-Byzantine servers ignore messages from Byzantine servers
- Maintains consensus despite malicious behavior

## Troubleshooting

### Common Issues

1. **Port Already in Use**:
   - Error: `Error starting server S1: bind: address already in use`
   - Solution: Ensure no other instance is running, or change ports in `shared/serveraddress.go`

2. **Connection Refused**:
   - Error: `Error connecting to server S1`
   - Solution: Ensure all servers have started before clients send transactions

3. **Transaction Not Executing**:
   - Check if enough non-Byzantine servers are active (need at least `2f+1`)
   - Verify Byzantine server list configuration
   - Check server logs for error messages

4. **View Change Not Completing**:
   - Ensure enough servers are non-Byzantine
   - Check if new primary is Byzantine (may cause issues)
   - Verify network connectivity between servers

### Debugging Tips

- Enable debug prints by uncommenting `fmt.Printf` statements in `server/server.go`
- Check transaction logs using menu option 1 (PrintLog)
- Verify server status using menu option 3 (PrintStatus)
- Monitor view changes using menu option 4 (PrintView)

## Technical Details

### Quorum Requirements

- **Pre-Prepare**: 1 message (from primary)
- **Prepare**: `2f+1` messages (including primary)
- **Commit**: `2f+1` messages
- **Execute**: `f+1` executed responses (for client confirmation)

### Optimistic Fast Path

When `3f+1` prepare messages are received (instead of just `2f+1`), the system can skip the commit phase and directly commit the transaction, reducing latency.

### Transaction Ordering

- Transactions are executed strictly in sequence number order
- A transaction with sequence number `n` cannot execute until all transactions with sequence numbers `< n` have executed
- This ensures deterministic state across all servers

## Future Enhancements

Potential improvements:
- Persistent storage for transaction logs
- Network partition handling
- Dynamic server addition/removal
- Enhanced performance optimizations
- Web-based monitoring dashboard

## License

[Specify license if applicable]

## Authors

[Add author information]

## Acknowledgments

- Based on the Practical Byzantine Fault Tolerance (PBFT) protocol by Castro and Liskov
- Uses Shamir's Secret Sharing from HashiCorp Vault
- RSA cryptography for digital signatures
