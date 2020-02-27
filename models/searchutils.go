package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// replaces ? to corresponding placeholder index ($1, $2,...)
func enumerateSQLPlacholder(msql string) (psql string) {
	splitItems := strings.Split(msql, "?")
	for i, item := range splitItems {
		if i != len(splitItems)-1 {
			psql = psql + item + fmt.Sprintf("$%d", i+1)
		} else {
			psql = psql + item
		}
	}
	return
}

func jsonify(input interface{}) string {
	j, _ := json.Marshal(input)
	return string(j)
}

func sqlComparisonOp(op string) string {
	switch op {
	case "gt":
		return ">"
	case "lt":
		return "<"
	case "gte":
		return ">="
	case "lte":
		return "<="
	case "ne":
		return "!="
	case "like":
		return "LIKE"
	case "notlike":
		return "NOT LIKE"
	case "is":
		return "IS"
	case "isnot":
		return "IS NOT"
	}
	return "="
}

func convertTermsToSQL(terms []map[string]interface{}) (where []string, args []interface{}) {
	// Sample terms
	/*
	   "terms": [
	       {"status": "completed", "active": true},
	       {"charge": 2000},
	       {"colors": ["red", "green"]},
	       {"products":{"qw":{"coupons":["x001"]}}}
	   ]
	*/
	// Corresponding SQL
	/*
	   -- string value
	   SELECT id FROM transactions WHERE data->'status' @> '"completed"'::jsonb;
	   -- boolean value
	   SELECT id FROM transactions WHERE data->'active' @> 'true'::jsonb;
	   -- numeric value
	   SELECT id FROM transactions WHERE data->'charge' @> '2000'::jsonb;
	   -- array value
	   SELECT id FROM transactions WHERE data->'colors' @> '["red", "green"]'::jsonb;
	   -- object value
	   SELECT id FROM transactions WHERE data->'products' @> '{"qw":{"coupons": ["x001"]}}'::jsonb;
	*/
	for _, term := range terms {
		var conditions []string
		for key, value := range term {
			conditions = append(
				conditions,
				fmt.Sprintf("data->'%s' @> ?::jsonb", key),
			)
			args = append(args, jsonify(value))
		}
		where = append(where, "("+strings.Join(conditions, " AND ")+")")
	}
	return
}

func convertRangesToSQL(ranges []map[string]map[string]interface{}) (where []string, args []interface{}) {
	// Sample ranges
	/*
	   "ranges": [
	       {"charge": {"gte": 2000, "lte": 4000}},
	       {"date": {"gt": "2017-01-01","lt": "2017-06-31"}}
	   ]
	*/
	// Corresponding SQL
	/*
	   -- numeric value
	   SELECT id, data->'charge' FROM transactions WHERE (data->>'charge')::float >= 2000 AND (data->>'charge')::float <= 4000;
	   -- other values
	   SELECT id, data->'date' FROM transactions WHERE data->>'date' >= '2017-01-01' AND data->>'date' < '2017-06-31';
	*/
	for _, rangeItem := range ranges {
		var conditions []string
		for key, comparison := range rangeItem {
			for op, value := range comparison {
				condn, arguments := getSQLConditionAndArgsFromRange(key, op, value)
				conditions = append(conditions, condn)
				for _, arg := range arguments {
					if arg != nil {
						args = append(args, arg)
					}
				}
			}
		}
		where = append(where, "("+strings.Join(conditions, " AND ")+")")
	}
	return
}

func getSQLConditionAndArgsFromRange(key string, op string, value interface{}) (condition string, args []interface{}) {
	getConditionAndArgs := func(key string, op string, val interface{}) (condn string, arg interface{}) {
		switch val.(type) {
		case int, int8, int16, int32, int64, float32, float64:
			condn = fmt.Sprintf("(data->>'%s')::float %s ?", key, sqlComparisonOp(op))
			arg = val
		case nil:
			condn = fmt.Sprintf("data->>'%s' %s null", key, sqlComparisonOp(op))
			arg = nil
		default:
			condn = fmt.Sprintf("data->>'%s' %s ?", key, sqlComparisonOp(op))
			arg = val
		}
		return
	}

	switch op {
	case "in", "nin":
		// Convert IN, NOT IN condition to OR of EQ and NE conditions
		var opnew string
		if op == "in" {
			opnew = "eq"
		} else if op == "nin" {
			opnew = "ne"
		}
		values, _ := value.([]interface{})
		for i, val := range values {
			c, arg := getConditionAndArgs(key, opnew, val)
			args = append(args, arg)
			if i == 0 {
				condition = c
			} else {
				condition = condition + " OR " + c
			}
		}
	default:
		c, arg := getConditionAndArgs(key, op, value)
		condition = c
		args = append(args, arg)
	}
	return
}

func convertFieldsToSQL(fields []map[string]map[string]interface{}) (where []string, args []interface{}) {
	// Sample ranges
	/*
	   "fields": [
	       {"id": {"eq": "ACME.CREDIT"}, "balance": {"lt": 0}},
	   ]
	*/
	// Corresponding SQL
	/*
	   -- numeric value
	   SELECT id, balance, data FROM accounts WHERE id = 'ACME.CREDIT' AND balance < 0;
	*/
	for _, field := range fields {
		var conditions []string
		for key, comparison := range field {
			if key == "id" {
				key = "transaction.id"
			}
			for op, value := range comparison {
				condn := fmt.Sprintf("%s %s ?", key, sqlComparisonOp(op))
				conditions = append(conditions, condn)
				args = append(args, value)
			}
		}
		where = append(where, "("+strings.Join(conditions, " AND ")+")")
	}
	return
}
