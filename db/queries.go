package db

import (
	"context"
	"strings"
	"time"

	"github.com/enginy88/PAN-USOM-API2EDL/usom"
)

type QueryParams struct {
	Types          []string
	DateFrom       time.Time
	DateTo         time.Time
	MinCriticality int
	MaxCriticality int
	OrderBy        string
	OrderDirection string
	Limit          int
}

func GetRecords(ctx context.Context, params QueryParams) ([]usom.Model, error) {
	query := strings.Builder{}
	query.WriteString("SELECT * FROM usom_records WHERE 1=1")

	var args []any

	if len(params.Types) > 0 {
		query.WriteString(" AND type IN (" + strings.Repeat("?,", len(params.Types)-1) + "?)")
		for _, t := range params.Types {
			args = append(args, t)
		}
	}

	if !params.DateFrom.IsZero() {
		query.WriteString(" AND date >= ?")
		args = append(args, params.DateFrom.Format("2006-01-02"))
	}

	if !params.DateTo.IsZero() {
		query.WriteString(" AND date <= ?")
		args = append(args, params.DateTo.Format("2006-01-02"))
	}

	if params.MinCriticality > 0 {
		query.WriteString(" AND criticality_level >= ?")
		args = append(args, params.MinCriticality)
	}

	if params.MaxCriticality > 0 {
		query.WriteString(" AND criticality_level <= ?")
		args = append(args, params.MaxCriticality)
	}

	if params.OrderBy != "" {
		query.WriteString(" ORDER BY " + params.OrderBy)
		if params.OrderDirection != "" {
			query.WriteString(" " + params.OrderDirection)
		}
	}

	if params.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, params.Limit)
	}

	rows, err := db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []usom.Model
	for rows.Next() {
		var r usom.Model
		err = rows.Scan(&r.ID, &r.URL, &r.Type, &r.Desc, &r.Source,
			&r.Date, &r.CriticalityLevel, &r.ConnectionType)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}
