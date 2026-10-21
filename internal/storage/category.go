package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryStore struct {
	pool *pgxpool.Pool
}

func (s *CategoryStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, type, description FROM categories
		 WHERE user_id=$1 ORDER BY type, name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("CategoryStore.ListByUser: %w", err)
	}
	defer rows.Close()
	var cats []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Description); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *CategoryStore) ListByType(
	ctx context.Context,
	userID uuid.UUID,
	catType model.OperationType,
) ([]model.Category, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, type, description FROM categories
		 WHERE user_id=$1 AND type=$2 ORDER BY name`,
		userID, catType,
	)
	if err != nil {
		return nil, fmt.Errorf("CategoryStore.ListByType: %w", err)
	}
	defer rows.Close()
	var cats []model.Category
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Description); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *CategoryStore) Create(
	ctx context.Context,
	userID uuid.UUID,
	inp model.CategoryInput,
) (*model.Category, error) {
	var c model.Category
	err := s.pool.QueryRow(ctx,
		`INSERT INTO categories(user_id, name, type, description) VALUES($1, $2, $3, $4)
		 RETURNING id, user_id, name, type, description`,
		userID, inp.Name, inp.Type, inp.Description,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Description)
	if err != nil {
		return nil, fmt.Errorf("CategoryStore.Create: %w", err)
	}
	return &c, nil
}

func (s *CategoryStore) GetByID(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Category, error) {
	var c model.Category
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, type, description FROM categories WHERE id=$1 AND user_id=$2`,
		id, userID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("CategoryStore.GetByID: %w", err)
	}
	return &c, nil
}

// Update меняет название и тип категории; описание не трогает, в интерфейсе
// его не редактируют. Тип меняется, только пока по категории нет операций:
// иначе их тип разойдётся с типом категории.
func (s *CategoryStore) Update(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	name string,
	catType model.OperationType,
) (*model.Category, error) {
	var c model.Category
	err := s.pool.QueryRow(ctx,
		`UPDATE categories SET name=$1, type=$2
		 WHERE id=$3 AND user_id=$4
		   AND (type = $2 OR NOT EXISTS (
		       SELECT 1 FROM operations WHERE category_id = $3 AND user_id = $4))
		 RETURNING id, user_id, name, type, description`,
		name, catType, id, userID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		// пустой результат - либо нет категории, либо мешают операции
		if _, err := s.GetByID(ctx, id, userID); err != nil {
			return nil, err
		}
		return nil, ErrCategoryInUse
	}
	if err != nil {
		return nil, fmt.Errorf("CategoryStore.Update: %w", err)
	}
	return &c, nil
}

func (s *CategoryStore) Delete(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM categories WHERE id=$1 AND user_id=$2`, id, userID)
	return checkRowsAffected(tag, err)
}
