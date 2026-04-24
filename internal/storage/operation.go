package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OperationStore struct {
	pool *pgxpool.Pool
}

// operationSelect selects all columns needed to populate model.Operation.
// category_id and transfer_id can be NULL (transfers), so LEFT JOINs are used.
const operationSelect = `
	SELECT o.id, o.user_id, o.wallet_id, o.category_id, o.type,
	       o.amount::float8, o.operated_at, o.description, o.created_at,
	       o.transfer_id,
	       w.name, COALESCE(c.name, ''), w.currency,
	       COALESCE(tw.name, '')
	FROM operations o
	JOIN  wallets w   ON w.id = o.wallet_id
	LEFT JOIN categories c  ON c.id  = o.category_id
	LEFT JOIN wallets    tw ON tw.id = o.transfer_wallet_id`

// scanOperation scans one row of operationSelect into *model.Operation.
// category_id and transfer_id are nullable, so we scan via pgtype.UUID.
func scanOperation(row interface{ Scan(...any) error }, o *model.Operation) error {
	var catID pgtype.UUID
	var xferID pgtype.UUID

	if err := row.Scan(
		&o.ID, &o.UserID, &o.WalletID, &catID, &o.Type,
		&o.Amount, &o.OperatedAt, &o.Description, &o.CreatedAt,
		&xferID,
		&o.WalletName, &o.CategoryName, &o.Currency,
		&o.TransferWalletName,
	); err != nil {
		return err
	}

	if catID.Valid {
		o.CategoryID = uuid.UUID(catID.Bytes)
	}
	if xferID.Valid {
		id := uuid.UUID(xferID.Bytes)
		o.TransferID = &id
	}
	return nil
}

func (s *OperationStore) List(ctx context.Context, f model.OperationFilter) ([]model.Operation, error) {
	q := operationSelect + ` WHERE o.user_id = $1`
	args := []any{f.UserID}
	n := 2

	if f.WalletID != uuid.Nil {
		q += fmt.Sprintf(" AND o.wallet_id = $%d", n)
		args = append(args, f.WalletID)
		n++
	}
	if f.CategoryID != uuid.Nil {
		q += fmt.Sprintf(" AND o.category_id = $%d", n)
		args = append(args, f.CategoryID)
		n++
	}
	if f.Type != "" {
		q += fmt.Sprintf(" AND o.type = $%d", n)
		args = append(args, f.Type)
		n++
	}
	if !f.DateFrom.IsZero() {
		q += fmt.Sprintf(" AND o.operated_at >= $%d", n)
		args = append(args, f.DateFrom)
		n++
	}
	if !f.DateTo.IsZero() {
		q += fmt.Sprintf(" AND o.operated_at < $%d", n)
		args = append(args, f.DateTo)
		n++
	}

	pageSize := 50
	offset := 0
	if f.Page > 1 {
		offset = (f.Page - 1) * pageSize
	}
	q += fmt.Sprintf(" ORDER BY o.operated_at DESC, o.id DESC LIMIT %d OFFSET %d", pageSize, offset)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("OperationStore.List: %w", err)
	}
	defer rows.Close()

	var ops []model.Operation
	for rows.Next() {
		var o model.Operation
		if err := scanOperation(rows, &o); err != nil {
			return nil, err
		}
		ops = append(ops, o)
	}
	return ops, rows.Err()
}

func (s *OperationStore) Create(
	ctx context.Context,
	userID uuid.UUID,
	inp model.OperationInput,
) (*model.Operation, error) {
	var o model.Operation
	row := s.pool.QueryRow(ctx,
		`INSERT INTO operations(user_id, wallet_id, category_id, type, amount, operated_at, description)
		 VALUES($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, user_id, wallet_id, category_id, type, amount::float8, operated_at, description, created_at,
		           transfer_id,
		           (SELECT name FROM wallets     WHERE id=$2),
		           (SELECT name FROM categories  WHERE id=$3),
		           (SELECT currency FROM wallets WHERE id=$2),
		           ''`,
		userID, inp.WalletID, inp.CategoryID, inp.Type, inp.Amount, inp.OperatedAt, inp.Description,
	)
	if err := scanOperation(row, &o); err != nil {
		return nil, fmt.Errorf("OperationStore.Create: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) GetByID(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Operation, error) {
	var o model.Operation
	err := scanOperation(
		s.pool.QueryRow(ctx, operationSelect+` WHERE o.id=$1 AND o.user_id=$2`, id, userID),
		&o,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("OperationStore.GetByID: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) Update(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	inp model.OperationInput,
) (*model.Operation, error) {
	var o model.Operation
	row := s.pool.QueryRow(ctx,
		`UPDATE operations
		 SET wallet_id=$1, category_id=$2, type=$3, amount=$4, operated_at=$5, description=$6
		 WHERE id=$7 AND user_id=$8 AND transfer_id IS NULL
		 RETURNING id, user_id, wallet_id, category_id, type, amount::float8, operated_at, description, created_at,
		           transfer_id,
		           (SELECT name FROM wallets     WHERE id=$1),
		           (SELECT name FROM categories  WHERE id=$2),
		           (SELECT currency FROM wallets WHERE id=$1),
		           ''`,
		inp.WalletID, inp.CategoryID, inp.Type, inp.Amount, inp.OperatedAt, inp.Description, id, userID,
	)
	err := scanOperation(row, &o)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("OperationStore.Update: %w", err)
	}
	return &o, nil
}

func (s *OperationStore) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM operations WHERE id=$1 AND user_id=$2 AND transfer_id IS NULL`,
		id, userID,
	)
	return checkRowsAffected(tag, err)
}

func (s *OperationStore) CreateTransfer(
	ctx context.Context,
	userID uuid.UUID,
	inp model.TransferInput,
) (*model.Operation, *model.Operation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("OperationStore.CreateTransfer begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	transferID := uuid.Must(uuid.NewV7())

	insertSQL := `
		INSERT INTO operations(user_id, wallet_id, type, amount, operated_at, description, transfer_id, transfer_wallet_id)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, wallet_id, category_id, type, amount::float8, operated_at, description, created_at,
		          transfer_id,
		          (SELECT name FROM wallets WHERE id=$2),
		          '',
		          (SELECT currency FROM wallets WHERE id=$2),
		          (SELECT name FROM wallets WHERE id=$8)`

	var expense model.Operation
	if err := scanOperation(
		tx.QueryRow(ctx, insertSQL,
			userID, inp.FromWalletID, model.TypeExpense, inp.Amount, inp.OperatedAt, inp.Description, transferID, inp.ToWalletID,
		),
		&expense,
	); err != nil {
		return nil, nil, fmt.Errorf("OperationStore.CreateTransfer expense: %w", err)
	}

	var income model.Operation
	if err := scanOperation(
		tx.QueryRow(ctx, insertSQL,
			userID, inp.ToWalletID, model.TypeIncome, inp.Amount, inp.OperatedAt, inp.Description, transferID, inp.FromWalletID,
		),
		&income,
	); err != nil {
		return nil, nil, fmt.Errorf("OperationStore.CreateTransfer income: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("OperationStore.CreateTransfer commit: %w", err)
	}
	return &expense, &income, nil
}

func (s *OperationStore) DeleteTransfer(
	ctx context.Context,
	transferID uuid.UUID,
	userID uuid.UUID,
) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM operations WHERE transfer_id=$1 AND user_id=$2`,
		transferID, userID,
	)
	if err != nil {
		return fmt.Errorf("OperationStore.DeleteTransfer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *OperationStore) DashboardData(
	ctx context.Context,
	userID uuid.UUID,
) ([]model.CategoryAmount, []model.MonthlyTotal, error) {
	catRows, err := s.pool.Query(ctx, `
		SELECT c.name, COALESCE(SUM(o.amount), 0)::float8
		FROM categories c
		JOIN operations o ON o.category_id = c.id AND o.user_id = c.user_id
		WHERE c.user_id = $1 AND o.type = $2
		  AND date_trunc('month', o.operated_at) = date_trunc('month', CURRENT_DATE)
		GROUP BY c.name
		ORDER BY SUM(o.amount) DESC`,
		userID, model.TypeExpense,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("DashboardData categories: %w", err)
	}
	defer catRows.Close()
	var catAmounts []model.CategoryAmount
	for catRows.Next() {
		var ca model.CategoryAmount
		if err := catRows.Scan(&ca.Name, &ca.Amount); err != nil {
			return nil, nil, err
		}
		catAmounts = append(catAmounts, ca)
	}
	if err := catRows.Err(); err != nil {
		return nil, nil, err
	}

	monthRows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('month', operated_at), 'YYYY-MM'),
		       SUM(CASE WHEN type=$2 THEN amount ELSE 0 END)::float8,
		       SUM(CASE WHEN type=$3 THEN amount ELSE 0 END)::float8
		FROM operations
		WHERE user_id = $1
		  AND operated_at >= date_trunc('month', CURRENT_DATE) - INTERVAL '5 months'
		GROUP BY date_trunc('month', operated_at)
		ORDER BY date_trunc('month', operated_at)`,
		userID, model.TypeIncome, model.TypeExpense,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("DashboardData monthly: %w", err)
	}
	defer monthRows.Close()
	var monthly []model.MonthlyTotal
	for monthRows.Next() {
		var mt model.MonthlyTotal
		if err := monthRows.Scan(&mt.Month, &mt.Income, &mt.Expense); err != nil {
			return nil, nil, err
		}
		monthly = append(monthly, mt)
	}
	return catAmounts, monthly, monthRows.Err()
}
