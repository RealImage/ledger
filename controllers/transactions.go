package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"

	ledgerContext "github.com/RealImage/QLedger/context"
	"github.com/RealImage/QLedger/models"
)

func unmarshalToTransaction(r *http.Request, txn *models.Transaction) error {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, txn)
	if err != nil {
		return err
	}
	var validKey = regexp.MustCompile(`^[a-z_A-Z0-9]+$`)
	for key := range txn.Data {
		if !validKey.MatchString(key) {
			return fmt.Errorf("Invalid key in data json: %v", key)
		}
	}
	// Validate timestamp format if present
	if txn.Timestamp != "" {
		_, err := time.Parse(models.LedgerTimestampLayout, txn.Timestamp)
		if err != nil {
			return err
		}
	}

	return nil
}

// MakeTransaction creates a new transaction from the request data
func MakeTransaction(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	transaction := &models.Transaction{}
	err := unmarshalToTransaction(r, transaction)
	if err != nil {
		log.Println("Error loading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Skip if the transaction is invalid
	// by validating the delta values
	if !transaction.IsValid() {
		log.Println("Transaction is invalid:", transaction.ID)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	transactionsDB := models.NewTransactionDB(context.DB)
	// Check if a transaction with same ID already exists
	isExists, err := transactionsDB.IsExists(transaction.ID)
	if err != nil {
		log.Println("Error while checking for existing transaction:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if isExists {
		// Check if the transaction lines are different
		// and conflicts with the existing lines
		isConflict, err := transactionsDB.IsConflict(transaction)
		if err != nil {
			log.Println("Error while checking for conflicting transaction:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if isConflict {
			// The conflicting transactions are denied
			log.Println("Transaction is conflicting:", transaction.ID)
			w.WriteHeader(http.StatusConflict)
			return
		}
		// Otherwise the transaction is just a duplicate
		// The exactly duplicate transactions are ignored
		// log.Println("Transaction is duplicate:", transaction.ID)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Otherwise, do transaction
	done := transactionsDB.Transact(transaction)
	if !done {
		log.Println("Transaction failed:", transaction.ID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	return
}

// GetTransactions returns the list of transactions that matches the search query
func GetTransactions(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	engine, aerr := models.NewSearchEngine(context.DB, models.SearchNamespaceTransactions)
	if aerr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	query := string(body)

	results, aerr := engine.Query(query)
	if aerr != nil {
		log.Println("Error while querying:", aerr)
		switch aerr.ErrorCode() {
		case "search.query.invalid":
			w.WriteHeader(http.StatusBadRequest)
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	data, err := json.Marshal(results)
	if err != nil {
		log.Println("Error while parsing results:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
	return
}

// UpdateTransaction updates the data of a transaction with the input ID
func UpdateTransaction(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	transaction := &models.Transaction{}
	err := unmarshalToTransaction(r, transaction)
	if err != nil {
		log.Println("Error loading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	transactionDB := models.NewTransactionDB(context.DB)
	// Check if a transaction with same ID already exists
	isExists, err := transactionDB.IsExists(transaction.ID)
	if err != nil {
		log.Println("Error while checking for existing transaction:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !isExists {
		log.Println("Transaction doesn't exist:", transaction.ID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Otherwise, update transaction
	terr := transactionDB.UpdateTransaction(transaction)
	if terr != nil {
		log.Printf("Error while updating transaction: %v (%v)", transaction.ID, terr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}
