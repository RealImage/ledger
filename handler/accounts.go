package handler

import (
	"fmt"
	"net/http"

	e "github.com/RealImage/QLedger/errors"
	"github.com/RealImage/QLedger/models"
	"github.com/RealImage/QLedger/utils"
)

// AddAccount creates a new account with the input ID and data
func (s *Service) AddAccount(w http.ResponseWriter, r *http.Request) {
	account := &models.Account{}
	err := utils.UnmarshalToAccount(r, account)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	err = s.Ctrl.AddAccount(account)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	utils.WriteResponse(w, nil, http.StatusCreated)
}

// GetAccounts returns the list of accounts that matches the search query
func (s *Service) GetAccounts(w http.ResponseWriter, r *http.Request) {
	query, err := utils.ParseString(w, r)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	result, err := s.Ctrl.GetAccounts(query)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	utils.WriteResponse(w, result, http.StatusOK)
}

// UpdateAccount updates data of an account with the input ID
func (s *Service) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	var account models.Account
	err := utils.UnmarshalToAccount(r, &account)
	if err != nil {
		utils.WriteErrorStatus(w, fmt.Errorf("%w Error loading payload: %v", e.ErrBadRequest, err))
		return
	}
	err = s.Ctrl.UpdateAccount(&account)
	if err != nil {
		utils.WriteErrorStatus(w, err)
		return
	}
	utils.WriteResponse(w, nil, http.StatusOK)
}
