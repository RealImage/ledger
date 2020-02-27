package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"

	e "github.com/RealImage/QLedger/errors"
	"github.com/RealImage/QLedger/models"
)

func WriteErrorStatus(w http.ResponseWriter, err error) {
	var status int
	switch {
	case errors.Is(err, e.ErrBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, e.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, e.ErrNotFound):
		status = http.StatusNotFound
	default:
		status = http.StatusInternalServerError
	}
	log.Println(err)
	w.WriteHeader(status)
}

func UnmarshalToAccount(r *http.Request, account *models.Account) error {
	err := json.NewDecoder(r.Body).Decode(account)
	if err != nil {
		return fmt.Errorf("%w %v", e.ErrBadRequest, err)
	}
	var validKey = regexp.MustCompile(`^[a-z_A-Z]+$`)
	for key := range account.Data {
		if !validKey.MatchString(key) {
			return fmt.Errorf("%w: Invalid key in data json: %v", e.ErrBadRequest, key)
		}
	}
	return nil
}

func UnmarshalToTransaction(r *http.Request, txn *models.Transaction) error {
	err := json.NewDecoder(r.Body).Decode(txn)
	if err != nil {
		return fmt.Errorf("%w %v", e.ErrBadRequest, err)
	}
	var validKey = regexp.MustCompile(`^[a-z_A-Z]+$`)
	for key := range txn.Data {
		if !validKey.MatchString(key) {
			return fmt.Errorf("%w: Invalid key in data json: %v", e.ErrBadRequest, key)
		}
	}
	// Validate timestamp format if present
	if txn.Timestamp != "" {
		_, err := time.Parse(models.LedgerTimestampLayout, txn.Timestamp)
		if err != nil {
			return fmt.Errorf("%w: %v", e.ErrBadRequest, err)
		}
	}

	return nil
}

func WriteResponse(w http.ResponseWriter, data interface{}, status int) {
	if data != nil {
		data, err := json.Marshal(data)
		if err != nil {
			WriteErrorStatus(w, fmt.Errorf("%w: %v", e.ErrInternal, err))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(data)
		if err != nil {
			log.Println(err)
		}
	}
	w.WriteHeader(status)
}

func ParseString(w http.ResponseWriter, r *http.Request) (string, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("%w: error reading payload: %v", e.ErrBadRequest, err)
	}
	return string(body), nil
}
