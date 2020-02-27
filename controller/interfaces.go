package controller

import (
	ledgerError "github.com/RealImage/QLedger/errors"
	"github.com/RealImage/QLedger/models"
)

type SearchEngine interface {
	Query(q string) (interface{}, ledgerError.ApplicationError)
}

type AccountDB interface {
	GetByID(id string) (*models.Account, ledgerError.ApplicationError)
	IsExists(id string) (bool, ledgerError.ApplicationError)
	CreateAccount(account *models.Account) ledgerError.ApplicationError
	UpdateAccount(account *models.Account) ledgerError.ApplicationError
}

type TransactionDB interface {
	IsExists(id string) (bool, ledgerError.ApplicationError)
	IsConflict(transaction *models.Transaction) (bool, ledgerError.ApplicationError)
	Transact(txn *models.Transaction) bool
	UpdateTransaction(txn *models.Transaction) ledgerError.ApplicationError
}
