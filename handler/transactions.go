package handler

import (
	"net/http"

	"github.com/RealImage/QLedger/models"
	"github.com/RealImage/QLedger/utils"
)

// MakeTransaction creates a new transaction from the request data
func (s *Service) MakeTransaction(w http.ResponseWriter, r *http.Request) {
	var transaction models.Transaction
	err := utils.UnmarshalToTransaction(r, &transaction)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	exists, err := s.Ctrl.MakeTransaction(&transaction)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	if !exists {
		utils.WriteResponse(w, nil, http.StatusCreated)
		return
	}
	utils.WriteResponse(w, nil, http.StatusAccepted)
}

// GetTransactions returns the list of transactions that matches the search query
func (s *Service) GetTransactions(w http.ResponseWriter, r *http.Request) {
	query, err := utils.ParseString(w, r)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	results, err := s.Ctrl.GetTransactions(query)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	utils.WriteResponse(w, results, http.StatusOK)
}

// UpdateTransaction updates the data of a transaction with the input ID
func (s *Service) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	var transaction models.Transaction
	err := utils.UnmarshalToTransaction(r, &transaction)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	err = s.Ctrl.UpdateTransaction(&transaction)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	utils.WriteResponse(w, nil, http.StatusOK)
}
