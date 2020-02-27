package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	ledgerError "github.com/RealImage/QLedger/errors"
)

var (
	// SearchNamespaceAccounts holds search namespace of accounts
	SearchNamespaceAccounts = "accounts"
	// SearchNamespaceTransactions holds search namespace of transactions
	SearchNamespaceTransactions = "transactions"
	// SortDescByTime option sorts search items in descending order of time
	SortDescByTime = "desc"
	// SortAscByTime option sorts search items in ascending order of time
	SortAscByTime = "asc"
)

// SearchEngine is the interface for all search operations
type SearchEngine struct {
	db        *sql.DB
	namespace string
}

// TransactionResult represents the response format of transactions
type TransactionResult struct {
	ID        string                   `json:"id"`
	Timestamp string                   `json:"timestamp"`
	Data      json.RawMessage          `json:"data"`
	Lines     []*TransactionLineResult `json:"lines"`
}

// TransactionLineResult represents the response format of transaction lines
type TransactionLineResult struct {
	AccountID string `json:"account"`
	Delta     int    `json:"delta"`
}

// AccountResult represents the response format of accounts
type AccountResult struct {
	ID      string          `json:"id"`
	Balance int             `json:"balance"`
	Data    json.RawMessage `json:"data"`
}

// NewSearchEngine returns a new instance of `SearchEngine`
func NewSearchEngine(db *sql.DB, namespace string) (*SearchEngine, ledgerError.ApplicationError) {
	if namespace != SearchNamespaceAccounts && namespace != SearchNamespaceTransactions {
		return nil, SearchNamespaceInvalidError(namespace)
	}

	return &SearchEngine{db: db, namespace: namespace}, nil
}

// Query returns the results of a search query
func (engine *SearchEngine) Query(q string) (interface{}, ledgerError.ApplicationError) {
	rawQuery, aerr := NewSearchRawQuery(q)
	if aerr != nil {
		return nil, aerr
	}

	sqlQuery := rawQuery.ToSQLQuery(engine.namespace)
	rows, err := engine.db.Query(sqlQuery.sql, sqlQuery.args...)
	if err != nil {
		return nil, DBError(err)
	}
	defer rows.Close()

	switch engine.namespace {
	case SearchNamespaceAccounts:
		accounts := make([]*AccountResult, 0)
		for rows.Next() {
			acc := &AccountResult{}
			if err := rows.Scan(&acc.ID, &acc.Balance, &acc.Data); err != nil {
				return nil, DBError(err)
			}
			accounts = append(accounts, acc)
		}
		return accounts, nil

	case SearchNamespaceTransactions:
		transactions := make([]*TransactionResult, 0)
		for rows.Next() {
			txn := &TransactionResult{}
			var rawAccounts, rawDelta string
			if err := rows.Scan(&txn.ID, &txn.Timestamp, &txn.Data, &rawAccounts, &rawDelta); err != nil {
				return nil, DBError(err)
			}

			var accounts []string
			var delta []int
			json.Unmarshal([]byte(rawAccounts), &accounts)
			json.Unmarshal([]byte(rawDelta), &delta)
			var lines []*TransactionLineResult
			for i, acc := range accounts {
				l := &TransactionLineResult{}
				l.AccountID = acc
				l.Delta = delta[i]
				lines = append(lines, l)
			}
			txn.Lines = lines
			transactions = append(transactions, txn)
		}
		return transactions, nil
	default:
		return nil, SearchNamespaceInvalidError(engine.namespace)
	}
}

// QueryContainer represents the format of query subsection inside `must` or `should`
type QueryContainer struct {
	Fields     []map[string]map[string]interface{} `json:"fields"`
	Terms      []map[string]interface{}            `json:"terms"`
	RangeItems []map[string]map[string]interface{} `json:"ranges"`
}

// SearchRawQuery represents the format of search query
type SearchRawQuery struct {
	Offset   int    `json:"from,omitempty"`
	Limit    int    `json:"size,omitempty"`
	SortTime string `json:"sort_time,omitempty"`
	Query    struct {
		MustClause   QueryContainer `json:"must"`
		ShouldClause QueryContainer `json:"should"`
	} `json:"query"`
}

// SearchSQLQuery hold information of search SQL query
type SearchSQLQuery struct {
	sql  string
	args []interface{}
}

func hasValidKeys(items interface{}) bool {
	var validKey = regexp.MustCompile(`^[a-z_A-Z]+$`)
	switch t := items.(type) {
	case []map[string]interface{}:
		for _, item := range t {
			for key := range item {
				if !validKey.MatchString(key) {
					return false
				}
			}
		}
		return true
	case []map[string]map[string]interface{}:
		for _, item := range t {
			for key := range item {
				if !validKey.MatchString(key) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

// NewSearchRawQuery returns a new instance of `SearchRawQuery`
func NewSearchRawQuery(q string) (*SearchRawQuery, ledgerError.ApplicationError) {
	var rawQuery *SearchRawQuery
	err := json.Unmarshal([]byte(q), &rawQuery)
	if err != nil {
		return nil, SearchQueryInvalidError(err)
	}

	checkList := []interface{}{
		rawQuery.Query.MustClause.Fields,
		rawQuery.Query.MustClause.Terms,
		rawQuery.Query.MustClause.RangeItems,
		rawQuery.Query.ShouldClause.Fields,
		rawQuery.Query.MustClause.Terms,
		rawQuery.Query.MustClause.RangeItems,
	}
	for _, item := range checkList {
		if !hasValidKeys(item) {
			return nil, SearchQueryInvalidError(errors.New("Invalid key(s) in search query"))
		}
	}
	return rawQuery, nil
}

// ToSQLQuery converts a raw search query to SQL format of the same
func (rawQuery *SearchRawQuery) ToSQLQuery(namespace string) *SearchSQLQuery {
	var q string
	var args []interface{}

	switch namespace {
	case SearchNamespaceAccounts:
		q = "SELECT id, balance, data FROM current_balances"
	case SearchNamespaceTransactions:
		q = `SELECT transactions.id, timestamp, data,
					array_to_json(ARRAY(
						SELECT lines.account_id FROM lines
							WHERE transaction_id=transactions.id
							ORDER BY lines.account_id
					)) AS account_array,
					array_to_json(ARRAY(
						SELECT lines.delta FROM lines
							WHERE transaction_id=transactions.id
							ORDER BY lines.account_id
					)) AS delta_array
			FROM transactions
			LEFT JOIN lines ON lines.transaction_id = transactions.id`
	default:
		return nil
	}

	// Process must queries
	var mustWhere []string
	mustClause := rawQuery.Query.MustClause
	fieldsWhere, fieldsArgs := convertFieldsToSQL(mustClause.Fields)
	mustWhere = append(mustWhere, fieldsWhere...)
	args = append(args, fieldsArgs...)

	termsWhere, termsArgs := convertTermsToSQL(mustClause.Terms)
	mustWhere = append(mustWhere, termsWhere...)
	args = append(args, termsArgs...)

	rangesWhere, rangesArgs := convertRangesToSQL(mustClause.RangeItems)
	mustWhere = append(mustWhere, rangesWhere...)
	args = append(args, rangesArgs...)

	// Process should queries
	var shouldWhere []string
	shouldClause := rawQuery.Query.ShouldClause
	fieldsWhere, fieldsArgs = convertFieldsToSQL(shouldClause.Fields)
	shouldWhere = append(shouldWhere, fieldsWhere...)
	args = append(args, fieldsArgs...)

	termsWhere, termsArgs = convertTermsToSQL(shouldClause.Terms)
	shouldWhere = append(shouldWhere, termsWhere...)
	args = append(args, termsArgs...)

	rangesWhere, rangesArgs = convertRangesToSQL(shouldClause.RangeItems)
	shouldWhere = append(shouldWhere, rangesWhere...)
	args = append(args, rangesArgs...)

	var offset = rawQuery.Offset
	var limit = rawQuery.Limit

	if len(mustWhere) == 0 && len(shouldWhere) == 0 {
		return &SearchSQLQuery{sql: q, args: args}
	}

	q += " WHERE "
	if len(mustWhere) != 0 {
		q += "(" + strings.Join(mustWhere, " AND ") + ")"
		if len(shouldWhere) != 0 {
			q += " AND "
		}
	}

	if len(shouldWhere) != 0 {
		q += "(" + strings.Join(shouldWhere, " OR ") + ")"
	}

	if namespace == SearchNamespaceTransactions {
		q += " GROUP BY transactions.id"
		if rawQuery.SortTime == SortDescByTime {
			q += " ORDER BY timestamp DESC"
		} else {
			q += " ORDER BY timestamp"
		}
	}

	if offset > 0 {
		q += " OFFSET " + strconv.Itoa(offset) + " "
	}
	if limit > 0 {
		q += " LIMIT " + strconv.Itoa(limit)
	}

	q = enumerateSQLPlacholder(q)
	return &SearchSQLQuery{sql: q, args: args}
}
