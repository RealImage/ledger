package controller

import (
	"fmt"

	"github.com/RealImage/QLedger/errors"
	e "github.com/RealImage/QLedger/errors"
	"github.com/RealImage/QLedger/models"
)

func (c *Controller) AddAccount(account *models.Account) error {
	isExists, err := c.AccountDB.IsExists(account.ID)
	if err != nil {
		return fmt.Errorf("%w: error while checking for existing account: %v", errors.ErrInternal, err)
	}
	if isExists {
		return fmt.Errorf("%w: account is conflicting: %s", errors.ErrConflict, account.ID)
	}
	err = c.AccountDB.CreateAccount(account)
	if err != nil {
		return fmt.Errorf("%w: error while adding account: %s (%v)", errors.ErrInternal, account.ID, err)
	}
	return nil
}

func (c *Controller) GetAccounts(query string) (interface{}, error) {
	results, err := c.SearchEngine.Query(query)
	if err != nil {
		switch err.ErrorCode() {
		case "search.query.invalid":
			return nil, fmt.Errorf("%w: error while querying :%v", e.ErrBadRequest, err)
		default:
			return nil, fmt.Errorf("%w: error while querying :%v", e.ErrInternal, err)
		}
	}
	return results, nil
}

func (c *Controller) UpdateAccount(account *models.Account) error {
	isExists, err := c.AccountDB.IsExists(account.ID)
	if err != nil {
		return fmt.Errorf("%w: error while checking for existing account: %v", e.ErrInternal, err)
	}
	if !isExists {
		return fmt.Errorf("%w: account doesn't exist: %v", e.ErrNotFound, err)
	}

	err = c.AccountDB.UpdateAccount(account)
	if err != nil {
		return fmt.Errorf("%w: error while updating account %s (%v)", e.ErrInternal, account.ID, err)
	}
	return nil
}
