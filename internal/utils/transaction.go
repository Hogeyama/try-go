package utils

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sqlcが生成するものと同じinterface
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

type TransactionManager interface {
	withinTransaction(ctx context.Context, fn func(DBTX) error) error
}

type sqlTransactionManager struct {
	db *pgxpool.Pool
}

func NewTransactionManager(db *pgxpool.Pool) TransactionManager {
	return &sqlTransactionManager{db: db}
}

func (m *sqlTransactionManager) withinTransaction(ctx context.Context, fn func(DBTX) error) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func Transactional[T any](ctx context.Context, txManager TransactionManager, fn func(DBTX) (T, error)) (T, error) {
	var result T
	err := txManager.withinTransaction(ctx, func(tx DBTX) error {
		var err error
		result, err = fn(tx)
		return err
	})
	return result, err
}

type Savepoint string

func NewSavepoint(db DBTX, ctx context.Context, savepointName string) (Savepoint, error) {
	_, err := db.Exec(ctx, fmt.Sprintf("SAVEPOINT %s", savepointName))
	if err != nil {
		return "", fmt.Errorf("failed to create savepoint: %w", err)
	}
	return Savepoint(savepointName), nil
}

func RollbackTo(db DBTX, ctx context.Context, savepoint Savepoint) error {
	_, err := db.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepoint))
	if err != nil {
		return fmt.Errorf("failed to rollback to savepoint: %w", err)
	}
	return nil
}
