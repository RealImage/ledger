package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RealImage/QLedger/controller"
	"github.com/RealImage/QLedger/models"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	TransactionsSearchAPI = "/v1/transactions"
)

type TransactionSearchSuite struct {
	suite.Suite
	handler Service
}

func (as *TransactionSearchSuite) SetupTest() {
	t := as.T()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	assert.NotEmpty(t, databaseURL)
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Panic("Unable to connect to Database:", err)
	}
	log.Println("Successfully established connection to database.")
	searchEngine, appErr := models.NewSearchEngine(db, models.SearchNamespaceTransactions)
	if appErr != nil {
		t.Fatal(appErr)
	}
	accountsDB := models.NewAccountDB(db)
	transactionsDB := models.NewTransactionDB(db)
	ctrl := controller.NewController(searchEngine, &accountsDB, &transactionsDB)
	as.handler = Service{Ctrl: ctrl}
	// Create test transactions
	txnDB := models.NewTransactionDB(db)
	txn1 := &models.Transaction{
		ID: "txn1",
		Lines: []*models.TransactionLine{
			&models.TransactionLine{
				AccountID: "acc1",
				Delta:     1000,
			},
			&models.TransactionLine{
				AccountID: "acc2",
				Delta:     -1000,
			},
		},
		Data: map[string]interface{}{
			"action": "setcredit",
			"expiry": "2018-01-01",
			"months": []string{"jan", "feb", "mar"},
		},
	}
	ok := txnDB.Transact(txn1)
	assert.Equal(t, true, ok, "Error creating test transaction")
	txn2 := &models.Transaction{
		ID: "txn2",
		Lines: []*models.TransactionLine{
			&models.TransactionLine{
				AccountID: "acc1",
				Delta:     100,
			},
			&models.TransactionLine{
				AccountID: "acc2",
				Delta:     -100,
			},
		},
		Data: map[string]interface{}{
			"action": "setcredit",
			"expiry": "2018-01-15",
			"months": []string{"apr", "may", "jun"},
		},
	}
	ok = txnDB.Transact(txn2)
	assert.Equal(t, true, ok, "Error creating test transaction")
	txn3 := &models.Transaction{
		ID: "txn3",
		Lines: []*models.TransactionLine{
			&models.TransactionLine{
				AccountID: "acc1",
				Delta:     400,
			},
			&models.TransactionLine{
				AccountID: "acc2",
				Delta:     -400,
			},
		},
		Data: map[string]interface{}{
			"action": "setcredit",
			"expiry": "2018-01-30",
			"months": []string{"jul", "aug", "sep"},
		},
	}
	ok = txnDB.Transact(txn3)
	assert.Equal(t, true, ok, "Error creating test transaction")
}

func (as *TransactionSearchSuite) TestTransactionsSearch() {
	t := as.T()

	// Prepare search query
	payload := `{
        "query": {
            "must": {
                "fields": [
                    {"id": {"eq": "txn1"}}
                ],
                "terms": [
                    {"action": "setcredit"}
                ]
            },
            "should": {
                "terms": [
                    {"months": ["jan", "feb", "mar"]},
                    {"months": ["apr", "may", "jun"]},
                    {"months": ["jul", "aug", "sep"]}
                ],
                "ranges": [
                    {"expiry": {"gte": "2018-01-01", "lte": "2018-01-30"}}
                ]
            }
        }
    }`

	req, err := http.NewRequest("GET", TransactionsSearchAPI, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	as.handler.GetTransactions(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "Invalid response code")

	var transactions []models.TransactionResult
	err = json.Unmarshal(rr.Body.Bytes(), &transactions)
	if err != nil {
		t.Errorf("Invalid json response: %v", rr.Body.String())
	}
	assert.Equal(t, 1, len(transactions), "Transactions count doesn't match")
	assert.Equal(t, "txn1", transactions[0].ID, "Transaction ID doesn't match")
}

func TestTransactionSearchSuite(t *testing.T) {
	suite.Run(t, new(TransactionSearchSuite))
}
