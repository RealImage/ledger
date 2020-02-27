package controller

import (
	"fmt"

	e "github.com/RealImage/QLedger/errors"
	"github.com/RealImage/QLedger/models"
)

func (c *Controller) MakeTransaction(transaction *models.Transaction) (bool, error) {
	if !transaction.IsValid() {
		return false, fmt.Errorf("%w: transaction is invalid: %s", e.ErrBadRequest, transaction.ID)
	}
	isExists, err := c.TransactionDB.IsExists(transaction.ID)
	if err != nil {
		return false, fmt.Errorf("%w: error while checking for existing transaction: %v", e.ErrInternal, err)
	}
	if !isExists {
		done := c.TransactionDB.Transact(transaction)
		if !done {
			return false, fmt.Errorf("%w: transaction failed: %s", e.ErrInternal, transaction.ID)
		}
		return false, nil
	}
	isConflict, err := c.TransactionDB.IsConflict(transaction)
	if err != nil {
		return false, fmt.Errorf("%w: error while checking for conflicting transaction: %v", e.ErrInternal, err)
	}
	if isConflict {
		// The conflicting transactions are denied
		return false, fmt.Errorf("%w: transaction is conflicting: %s", e.ErrConflict, transaction.ID)
	}
	return true, nil
}

func (c *Controller) GetTransactions(query string) (interface{}, error) {
	results, err := c.SearchEngine.Query(query)
	if err != nil {
		switch err.ErrorCode() {
		case "search.query.invalid":
			return nil, fmt.Errorf("%w: error while querying: %v", e.ErrBadRequest, err)
		default:
			return nil, fmt.Errorf("%w: error while querying: %v", e.ErrInternal, err)
		}
	}
	return results, nil
}

func (c *Controller) UpdateTransaction(transaction *models.Transaction) error {
	isExists, err := c.TransactionDB.IsExists(transaction.ID)
	if err != nil {
		return fmt.Errorf("%w: error while checking for existing transaction: %v", e.ErrInternal, err)
	}
	if !isExists {
		return fmt.Errorf("%w: transaction doesn't exist: %s", e.ErrNotFound, transaction.ID)
	}
	err = c.TransactionDB.UpdateTransaction(transaction)
	if err != nil {
		return fmt.Errorf("%w: error while updating transaction: %s (%v)", e.ErrInternal, transaction.ID, err)
	}
	return nil
}
