package handler

import (
	"net/http"

	ctrl "github.com/RealImage/QLedger/controller"
	"github.com/RealImage/QLedger/middlewares"
	"github.com/julienschmidt/httprouter"
)

type Service struct {
	Ctrl *ctrl.Controller
}

func AccountRouter(hostPrefix string, router *httprouter.Router, ctrl *ctrl.Controller) {
	service := Service{Ctrl: ctrl}
	router.HandlerFunc(http.MethodPost, hostPrefix+"/v1/accounts",
		middlewares.TokenAuthMiddleware(service.AddAccount))
	router.HandlerFunc(http.MethodPost, hostPrefix+"/v1/transactions",
		middlewares.TokenAuthMiddleware(service.MakeTransaction))
	// Read or search accounts and transactions
	router.HandlerFunc(http.MethodGet, hostPrefix+"/v1/accounts",
		middlewares.TokenAuthMiddleware(service.GetAccounts))
	router.HandlerFunc(http.MethodPost, hostPrefix+"/v1/accounts/_search",
		middlewares.TokenAuthMiddleware(service.GetAccounts))
}

func TransactionRouter(hostPrefix string, router *httprouter.Router, ctrl *ctrl.Controller) {
	service := Service{Ctrl: ctrl}
	router.HandlerFunc(http.MethodGet, hostPrefix+"/v1/transactions",
		middlewares.TokenAuthMiddleware(service.GetTransactions))
	router.HandlerFunc(http.MethodPost, hostPrefix+"/v1/transactions/_search",
		middlewares.TokenAuthMiddleware(service.GetTransactions))
	// Update data of accounts and transactions
	router.HandlerFunc(http.MethodPut, hostPrefix+"/v1/accounts",
		middlewares.TokenAuthMiddleware(service.UpdateAccount))
	router.HandlerFunc(http.MethodPut, hostPrefix+"/v1/transactions",
		middlewares.TokenAuthMiddleware(service.UpdateTransaction))
}

func NewRouter(hostPrefix string, accCtrl *ctrl.Controller, trCtrl *ctrl.Controller) *httprouter.Router {
	router := httprouter.New()
	router.HandlerFunc(http.MethodGet, hostPrefix+"/ping", Ping)
	AccountRouter(hostPrefix, router, accCtrl)
	TransactionRouter(hostPrefix, router, trCtrl)
	return router
}
