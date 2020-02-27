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
	AccountSearchAPI = "/v1/accounts"
)

type AccountsSearchSuite struct {
	suite.Suite
	handler Service
}

func (as *AccountsSearchSuite) SetupTest() {
	t := as.T()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	assert.NotEmpty(t, databaseURL)
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Panic("Unable to connect to Database:", err)
	}
	log.Println("Successfully established connection to database.")
	searchEngine, appErr := models.NewSearchEngine(db, models.SearchNamespaceAccounts)
	if appErr != nil {
		t.Fatal(appErr)
	}
	accountsDB := models.NewAccountDB(db)
	transactionsDB := models.NewTransactionDB(db)
	ctrl := controller.NewController(searchEngine, &accountsDB, &transactionsDB)
	as.handler = Service{Ctrl: ctrl}

	// Create test accounts
	accDB := models.NewAccountDB(db)
	acc1 := &models.Account{
		ID: "acc1",
		Data: map[string]interface{}{
			"customer_id": "C1",
			"status":      "active",
			"created":     "2017-01-01",
		},
	}
	err = accDB.CreateAccount(acc1)
	assert.Equal(t, nil, err, "Error creating test account")
	acc2 := &models.Account{
		ID: "acc2",
		Data: map[string]interface{}{
			"customer_id": "C2",
			"status":      "inactive",
			"created":     "2017-06-30",
		},
	}
	err = accDB.CreateAccount(acc2)
	assert.Equal(t, nil, err, "Error creating test account")
}

func (as *AccountsSearchSuite) TestAccountsSearch() {
	t := as.T()

	// Prepare search query
	payload := `{
        "query": {
            "must": {
                "fields": [
                    {"id": {"eq": "acc1"}}
                ],
                "terms": [
                    {"status": "active"}
                ]
            },
            "should": {
                "terms": [
                    {"customer_id": "C1"}
                ],
                "ranges": [
                    {"created": {"gte": "2018-01-01", "lte": "2018-01-30"}}
                ]
            }
        }
    }`
	req, err := http.NewRequest("GET", AccountSearchAPI, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	as.handler.GetAccounts(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "Invalid response code")

	var accounts []models.AccountResult
	err = json.Unmarshal(rr.Body.Bytes(), &accounts)
	if err != nil {
		t.Errorf("Invalid json response: %v", rr.Body.String())
	}
	assert.Equal(t, 1, len(accounts), "Accounts count doesn't match")
	assert.Equal(t, "acc1", accounts[0].ID, "Account ID doesn't match")
}

func TestAccountsSuite(t *testing.T) {
	suite.Run(t, new(AccountsSearchSuite))
}
