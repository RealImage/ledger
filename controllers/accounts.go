package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"

	ledgerContext "github.com/RealImage/QLedger/context"
	"github.com/RealImage/QLedger/models"
)

// GetAccounts returns the list of accounts that matches the search query
func GetAccounts(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	query := string(body)

	engine, aerr := models.NewSearchEngine(context.DB, models.SearchNamespaceAccounts)
	if aerr != nil {
		log.Println("Error while creating Search Engine:", aerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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

func unmarshalToAccount(r *http.Request, account *models.Account) error {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, account)
	if err != nil {
		return err
	}
	var validKey = regexp.MustCompile(`^[a-z_A-Z0-9]+$`)
	for key := range account.Data {
		if !validKey.MatchString(key) {
			return fmt.Errorf("Invalid key in data json: %v", key)
		}
	}
	return nil
}

// AddAccount creates a new account with the input ID and data
func AddAccount(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	account := &models.Account{}
	err := unmarshalToAccount(r, account)
	if err != nil {
		log.Println("Error loading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		//TODO Should we return any error message?
		return
	}

	accountsDB := models.NewAccountDB(context.DB)
	// Check if an account with same ID already exists
	isExists, err := accountsDB.IsExists(account.ID)
	if err != nil {
		log.Println("Error while checking for existing account:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if isExists {
		log.Println("Account is conflicting:", account.ID)
		w.WriteHeader(http.StatusConflict)
		return
	}

	// Otherwise, add account
	aerr := accountsDB.CreateAccount(account)
	if aerr != nil {
		log.Printf("Error while adding account: %v (%v)", account.ID, aerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	return
}

// UpdateAccount updates data of an account with the input ID
func UpdateAccount(w http.ResponseWriter, r *http.Request, context *ledgerContext.AppContext) {
	account := &models.Account{}
	err := unmarshalToAccount(r, account)
	if err != nil {
		log.Println("Error loading payload:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	accountsDB := models.NewAccountDB(context.DB)
	// Check if an account with same ID already exists
	isExists, err := accountsDB.IsExists(account.ID)
	if err != nil {
		log.Println("Error while checking for existing account:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !isExists {
		log.Println("Account doesn't exist:", account.ID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Otherwise, update account
	aerr := accountsDB.UpdateAccount(account)
	if aerr != nil {
		log.Printf("Error while updating account: %v (%v)", account.ID, aerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	return
}
