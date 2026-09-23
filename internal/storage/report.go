package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// reportArgs — нормализованные под SQL параметры фильтра отчёта.
// nil-слайс pgx кодирует в SQL NULL, а не в '{}', пустой Type ломает
// приведение к enum, нулевой time.Time даёт дату 0001-01-01 — поэтому
// «пустые» значения превращаются в пустые слайсы и nil-указатели.
type reportArgs struct {
	categoryIDs []uuid.UUID
	walletIDs   []uuid.UUID
	opType      *model.OperationType
	dateFrom    *time.Time
	dateTo      *time.Time
}

func normalizeReportFilter(f model.ReportFilter) reportArgs {
	a := reportArgs{
		categoryIDs: f.CategoryIDs,
		walletIDs:   f.WalletIDs,
	}
	if a.categoryIDs == nil {
		a.categoryIDs = []uuid.UUID{}
	}
	if a.walletIDs == nil {
		a.walletIDs = []uuid.UUID{}
	}
	if f.Type != "" {
		opType := f.Type
		a.opType = &opType
	}
	if !f.DateFrom.IsZero() {
		from := f.DateFrom
		a.dateFrom = &from
	}
	if !f.DateTo.IsZero() {
		to := f.DateTo
		a.dateTo = &to
	}
	return a
}

// reportBoundsSQL — фактические границы данных под фильтр отчёта.
// operated_at NULLable (migrations/001_init.sql), переводы без категории
// в отчёт не попадают.
const reportBoundsSQL = `
	SELECT MIN(o.operated_at) AS min_at,
	       MAX(o.operated_at) AS max_at,
	       COUNT(*)           AS cnt
	FROM operations o
	JOIN categories c ON c.id = o.category_id AND c.user_id = o.user_id
	WHERE o.user_id = $1
	  AND o.transfer_id IS NULL
	  AND o.operated_at IS NOT NULL
	  AND (cardinality($2::uuid[]) = 0 OR o.category_id = ANY($2::uuid[]))
	  AND (cardinality($3::uuid[]) = 0 OR o.wallet_id   = ANY($3::uuid[]))
	  AND ($4::operation_type IS NULL OR o.type = $4)
	  AND ($5::timestamptz    IS NULL OR o.operated_at >= $5)
	  AND ($6::timestamptz    IS NULL OR o.operated_at <  $6)`

// ReportBounds возвращает минимальную и максимальную даты операций,
// попавших под фильтр, и их количество. Пустая выборка — нулевые границы.
func (s *OperationStore) ReportBounds(
	ctx context.Context,
	filter model.ReportFilter,
) (model.ReportBounds, error) {
	a := normalizeReportFilter(filter)

	var minAt, maxAt pgtype.Timestamptz
	var count int
	err := s.pool.QueryRow(ctx, reportBoundsSQL,
		filter.UserID, a.categoryIDs, a.walletIDs, a.opType, a.dateFrom, a.dateTo,
	).Scan(&minAt, &maxAt, &count)
	if err != nil {
		return model.ReportBounds{}, fmt.Errorf("OperationStore.ReportBounds: %w", err)
	}

	b := model.ReportBounds{Count: count}
	if minAt.Valid {
		b.From = minAt.Time
	}
	if maxAt.Valid {
		b.To = maxAt.Time
	}
	return b, nil
}

// reportRowsSQL — суммы по (бакет, категория). Алиасы обязательны:
// pgx.RowToStructByName матчит поля структуры по именам колонок.
// AT TIME ZONE 'UTC' убирает зависимость бакетов от таймзоны сессии —
// Go-сетка строится тоже по UTC.
const reportRowsSQL = `
	SELECT date_trunc($2, o.operated_at AT TIME ZONE 'UTC') AS bucket,
	       o.category_id                                    AS category_id,
	       c.name                                           AS name,
	       c.type                                           AS type,
	       SUM(o.amount_minor)::int8                        AS amount
	FROM operations o
	JOIN categories c ON c.id = o.category_id AND c.user_id = o.user_id
	WHERE o.user_id = $1
	  AND o.transfer_id IS NULL
	  AND o.operated_at IS NOT NULL
	  AND (cardinality($3::uuid[]) = 0 OR o.category_id = ANY($3::uuid[]))
	  AND (cardinality($4::uuid[]) = 0 OR o.wallet_id   = ANY($4::uuid[]))
	  AND ($5::operation_type IS NULL OR o.type = $5)
	  AND ($6::timestamptz    IS NULL OR o.operated_at >= $6)
	  AND ($7::timestamptz    IS NULL OR o.operated_at <  $7)
	GROUP BY 1, 2, 3, 4
	ORDER BY bucket, amount DESC, name`

// ReportRows возвращает суммы операций, сгруппированные по временному
// бакету заданной гранулярности и категории.
func (s *OperationStore) ReportRows(
	ctx context.Context,
	filter model.ReportFilter,
	granularity model.Granularity,
) ([]model.ReportRow, error) {
	a := normalizeReportFilter(filter)

	rows, err := s.pool.Query(ctx, reportRowsSQL,
		filter.UserID, string(granularity),
		a.categoryIDs, a.walletIDs, a.opType, a.dateFrom, a.dateTo,
	)
	if err != nil {
		return nil, fmt.Errorf("OperationStore.ReportRows: %w", err)
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.ReportRow])
	if err != nil {
		return nil, fmt.Errorf("OperationStore.ReportRows collect: %w", err)
	}
	return result, nil
}
