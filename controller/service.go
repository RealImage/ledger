package controller

type Controller struct {
	SearchEngine  SearchEngine
	AccountDB     AccountDB
	TransactionDB TransactionDB
}

func NewController(s SearchEngine, a AccountDB, t TransactionDB) *Controller {
	return &Controller{SearchEngine: s, AccountDB: a, TransactionDB: t}
}
