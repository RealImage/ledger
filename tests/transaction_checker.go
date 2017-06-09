package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"time"

	ledgerContext "github.com/RealImage/QLedger/context"
	"github.com/RealImage/QLedger/controllers"
	"github.com/RealImage/QLedger/middlewares"
	"github.com/RealImage/QLedger/models"
	_ "github.com/lib/pq"
)

// CSV tests runner
func main() {
	var endpoint, filename string
	var load int
	flag.StringVar(&endpoint, "endpoint", "", "API endpoint")
	flag.StringVar(&filename, "filename", "transactions.csv", "Transactions CSV file")
	flag.IntVar(&load, "load", 10, "Load count for repeating the tests")
	flag.Parse()

	if len(endpoint) == 0 {
		log.Println("Connecting to the test database")
		db, err := sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
		if err != nil {
			log.Panic("Unable to connect to Database:", err)
		}
		log.Println("Successfully established connection to database.")
		log.Println("Starting test enpoints...")
		appContext := &ledgerContext.AppContext{DB: db}
		accountServer := httptest.NewServer(middlewares.ContextMiddleware(controllers.GetAccountInfo, appContext))
		transactionsServer := httptest.NewServer(middlewares.ContextMiddleware(controllers.MakeTransaction, appContext))
		defer accountServer.Close()
		defer transactionsServer.Close()
		log.Println("Running tests from endpoints:", accountServer.URL, transactionsServer.URL)
		RunCSVTests(accountServer.URL, transactionsServer.URL, filename, load)
	} else {
		log.Println("Running tests from endpoint:", endpoint)
		RunCSVTests(endpoint, endpoint, filename, load)
	}
}

func RunCSVTests(accountsEndpoint string, transactionsEndpoint string, filename string, load int) {
	// Timestamp to avoid conflict IDs
	timestamp := time.Now().UTC().Format("20060102150405")

	log.Println("Importing data from CSV:", filename)
	transactions, accounts := ImportTransactionCSV(filename)

	// test sequential transactions
	log.Println("Testing sequential transactions...")
	PrepareExpectedBalance(accountsEndpoint, accounts, load)
	for _, transaction := range transactions {
		for i := 1; i <= load; i++ {
			tag := fmt.Sprintf("sequential_%v_%v", i, timestamp)
			t := CloneTransaction(transaction, tag)
			status := PostTransaction(transactionsEndpoint, t)
			if status != http.StatusCreated {
				log.Fatalf("Sequential transaction:%v failed with status code:%v", t["id"], status)
			}
		}
	}
	VerifyExpectedBalance(accountsEndpoint, accounts)
	log.Println("Successful sequential transactions")

	// test parallel transactions
	log.Println("Testing parallel transactions...")
	PrepareExpectedBalance(accountsEndpoint, accounts, load)
	var pwg sync.WaitGroup
	pwg.Add(len(transactions) * load)
	for _, transaction := range transactions {
		for i := 1; i <= load; i++ {
			tag := fmt.Sprintf("parallel_%v_%v", i, timestamp)
			t := CloneTransaction(transaction, tag)
			go func() {
				status := PostTransaction(transactionsEndpoint, t)
				if status != http.StatusCreated {
					log.Fatalf("Parallel transaction:%v failed with status code:%v", t["id"], status)
				}
				pwg.Done()
			}()
		}
	}
	pwg.Wait()
	VerifyExpectedBalance(accountsEndpoint, accounts)
	log.Println("Successful parallel transactions")

	// test repeated parallel transactions
	log.Println("Testing repeated parallel transactions...")
	PrepareExpectedBalance(accountsEndpoint, accounts, load)
	var rwg sync.WaitGroup
	rwg.Add(len(transactions) * load * 2)
	for _, transaction := range transactions {
		for i := 1; i <= load; i++ {
			tag := fmt.Sprintf("repeated_%v_%v", i, timestamp)
			t := CloneTransaction(transaction, tag)
			var localwg sync.WaitGroup
			localwg.Add(2)
			var status1, status2 int
			go func() {
				status1 = PostTransaction(transactionsEndpoint, t)
				rwg.Done()
				localwg.Done()
			}()
			go func() {
				status2 = PostTransaction(transactionsEndpoint, t)
				rwg.Done()
				localwg.Done()
			}()
			localwg.Wait()
			if status1 == http.StatusCreated && status2 == http.StatusCreated {
				log.Fatalf("Parallel repeated transactions with same ID %v are accepted", t["id"])
			} else if status1 >= 400 && status2 >= 400 {
				log.Fatalf("Both parallel repeated transactions with same ID %v are failed", t["id"])
			}
		}
	}
	rwg.Wait()
	VerifyExpectedBalance(accountsEndpoint, accounts)
	log.Println("Successful repeated parallel transactions")
}

func ImportTransactionCSV(filename string) ([]map[string]interface{}, []map[string]interface{}) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalln("Error opening CSV:", err)
	}
	rdr := csv.NewReader(bufio.NewReader(file))
	rdr.FieldsPerRecord = 3 //transaction_id,account_id,delta
	rows, err := rdr.ReadAll()
	if err != nil {
		log.Fatalln("Error reading CSV:", err)
	}

	transactions := make(map[string]interface{})
	accounts := make(map[string]interface{})
	for _, row := range rows[1:] { // skip row 0
		transactionID, accountID, deltaVal := row[0], row[1], row[2]
		delta, err := strconv.Atoi(deltaVal)
		if err != nil {
			log.Fatalf("Invalid delta: %v (%v)", deltaVal, err)
		}
		// track the transactions
		if _, ok := transactions[transactionID]; !ok {
			transactions[transactionID] = map[string]interface{}{
				"_id": transactionID,
				"lines": []map[string]interface{}{
					{
						"account": accountID,
						"delta":   delta,
					},
				},
			}
		} else {
			txn, _ := transactions[transactionID].(map[string]interface{})
			lines, _ := txn["lines"].([]map[string]interface{})
			lines = append(lines, map[string]interface{}{
				"account": accountID,
				"delta":   delta,
			})
			txn["lines"] = lines
		}
		// track the accounts
		if _, ok := accounts[accountID]; !ok {
			accounts[accountID] = map[string]interface{}{
				"id":        accountID,
				"delta_sum": delta,
			}
		} else {
			acc, _ := accounts[accountID].(map[string]interface{})
			deltaSum, _ := acc["delta_sum"].(int)
			acc["delta_sum"] = deltaSum + delta
		}
	}

	// convert to slices
	var transactionsList []map[string]interface{}
	for _, txn := range transactions {
		t, _ := txn.(map[string]interface{})
		transactionsList = append(transactionsList, t)
	}
	var accountsList []map[string]interface{}
	for _, acc := range accounts {
		a, _ := acc.(map[string]interface{})
		accountsList = append(accountsList, a)
	}
	return transactionsList, accountsList
}

func GetAccountBalance(endpoint string, accountID interface{}) int {
	accountsURL := fmt.Sprintf("%v/v1/accounts?id=%v", endpoint, accountID)
	res, err := http.Get(accountsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	account := models.Account{}

	err = json.NewDecoder(res.Body).Decode(&account)
	if err != nil {
		log.Fatal("Invalid json response:", err)
	}
	return account.Balance
}

func PostTransaction(endpoint string, transaction map[string]interface{}) int {
	log.Printf("Posting transaction: %v", transaction["id"])
	payload, err := json.Marshal(transaction)
	if err != nil {
		log.Fatalf("Invalid transaction data: %v (%v)", transaction, err)
	}
	transactionsURL := endpoint + "/v1/transactions"
	res, err := http.Post(transactionsURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("Error in transaction:%v (%v)", transaction["id"], err)
	}
	log.Printf("Completed transaction:%v with status:%v", transaction["id"], res.StatusCode)
	return res.StatusCode
}

func CloneTransaction(transaction map[string]interface{}, tag string) map[string]interface{} {
	t := make(map[string]interface{})
	t["id"] = fmt.Sprintf("%v_%v", tag, transaction["_id"])
	t["lines"] = transaction["lines"]
	return t
}

func PrepareExpectedBalance(endpoint string, accounts []map[string]interface{}, load int) {
	log.Println("Preparing expected balances...")
	for _, acc := range accounts {
		currentBalance := GetAccountBalance(endpoint, acc["id"])
		deltaSum, _ := acc["delta_sum"].(int)
		acc["expected_balance"] = currentBalance + (deltaSum * load)
		log.Printf("Expected balance of account:%v = %v", acc["id"], acc["expected_balance"])
	}
}

func VerifyExpectedBalance(endpoint string, accounts []map[string]interface{}) {
	log.Println("Verifying expected balances...")
	for _, acc := range accounts {
		currentBalance := GetAccountBalance(endpoint, acc["id"])
		log.Printf("Current balance of account:%v = %v", acc["id"], currentBalance)
		if currentBalance != acc["expected_balance"] {
			panic("Incorrect balance")
		}
	}
}
