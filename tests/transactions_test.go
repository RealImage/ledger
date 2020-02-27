package tests

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/RealImage/QLedger/controller"
	"github.com/RealImage/QLedger/handler"
	"github.com/RealImage/QLedger/models"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
)

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
			if (status1 != http.StatusCreated && status1 != http.StatusAccepted) || (status2 != http.StatusCreated && status2 != http.StatusAccepted) {
				log.Fatalf("Parallel repeated transactions with same ID %v are not accepted", t["id"])
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
	payload := []byte(fmt.Sprintf(`{
	  "query": {
	    "must": {"fields": [{"id": {"eq": "%s"}}]}
	  }
	}`, accountID))
	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Panic("Unable to get account balance:", err)
	}
	defer resp.Body.Close()

	var accounts []models.AccountResult
	body, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &accounts)
	if err != nil {
		log.Panic("Error parsing accounts result:", err)
	}
	if len(accounts) == 0 {
		return 0
	}

	return accounts[0].Balance
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

type CSVSuite struct {
	suite.Suite
	accountServer      *httptest.Server
	transactionsServer *httptest.Server
	db                 *sql.DB
}

type testHandler struct {
	handler func(w http.ResponseWriter, r *http.Request)
}

func (t *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.handler(w, r)
}

func (cs *CSVSuite) SetupTest() {
	log.Println("Connecting to the test database")
	db, err := sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		log.Panic("Unable to connect to Database:", err)
	}
	log.Println("Successfully established connection to database.")
	log.Println("Starting test endpoints...")
	cs.db = db
	searchEngine, appErr := models.NewSearchEngine(db, models.SearchNamespaceTransactions)
	if appErr != nil {
		log.Panic(appErr)
	}
	accountsDB := models.NewAccountDB(db)
	transactionsDB := models.NewTransactionDB(db)
	ctrl := controller.NewController(searchEngine, &accountsDB, &transactionsDB)
	h := handler.Service{Ctrl: ctrl}
	cs.accountServer = httptest.NewServer(&testHandler{handler: h.GetAccounts})
	cs.transactionsServer = httptest.NewServer(&testHandler{handler: h.MakeTransaction})
}

func (cs *CSVSuite) TestTransactionsLoad() {
	log.Println("Running tests from endpoints:", cs.accountServer.URL, cs.transactionsServer.URL)
	RunCSVTests(cs.accountServer.URL, cs.transactionsServer.URL, "transactions.csv", 3)
}

func (cs *CSVSuite) TearDownTest() {
	log.Println("Closing test endpoints...")
	defer cs.accountServer.Close()
	defer cs.transactionsServer.Close()

	log.Println("Cleaning up the test database")
	t := cs.T()
	_, err := cs.db.Exec(`DELETE FROM lines`)
	if err != nil {
		t.Fatal("Error deleting lines:", err)
	}
	_, err = cs.db.Exec(`DELETE FROM transactions`)
	if err != nil {
		t.Fatal("Error deleting transactions:", err)
	}
	_, err = cs.db.Exec(`DELETE FROM accounts`)
	if err != nil {
		t.Fatal("Error deleting accounts:", err)
	}
}

func TestCSVSuite(t *testing.T) {
	suite.Run(t, new(CSVSuite))
}
