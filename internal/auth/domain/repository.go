package authdomain

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	authdb "demo/internal/auth/db"
	"demo/internal/utils"
)

// paramsをわざわざ詰め替えるのは微妙なのでauthdbの型をそのまま使っている
type AuthRepository interface {
	CreateUser(ctx context.Context, param authdb.CreateUserParams) (User, *AuthRepositoryError)
	GetUserByUsername(ctx context.Context, username string) (User, *AuthRepositoryError)
	CreateSession(ctx context.Context, param authdb.CreateSessionParams) (Session, *AuthRepositoryError)
	DeleteExpiredSessions(ctx context.Context) *AuthRepositoryError
	DeleteSession(ctx context.Context, sessionToken string) *AuthRepositoryError
	GetSessionByToken(ctx context.Context, sessionToken string) (Session, *AuthRepositoryError)
}

type AuthRepositoryError struct {
	Code AuthRepositoryErrorCode
	Err  error
}

func (e *AuthRepositoryError) Error() string {
	return e.Err.Error()
}

type AuthRepositoryErrorCode int

const (
	AuthRepositoryErrorUnknown AuthRepositoryErrorCode = iota
	AuthRepositoryErrorNotFound
	AuthRepositoryErrorAlreadyExists
)

// ----------------------------------------------------------------------------
// Implementations

type DefaultAuthRepository struct {
	db   *authdb.Queries
	dbtx *utils.DBTX
}

func NewDefaultRepository(dbtx *utils.DBTX) *DefaultAuthRepository {
	return &DefaultAuthRepository{
		db:   authdb.New(*dbtx),
		dbtx: dbtx,
	}
}

func (r *DefaultAuthRepository) CreateUser(ctx context.Context, param authdb.CreateUserParams) (User, *AuthRepositoryError) {
	savepoint, err := utils.NewSavepoint(*r.dbtx, ctx, "create_user")
	if err != nil {
		return User{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	user, err := r.db.CreateUser(ctx, param)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // ユニーク制約違反
			err := utils.RollbackTo(*r.dbtx, ctx, savepoint)
			if err != nil {
				return User{}, &AuthRepositoryError{
					Code: AuthRepositoryErrorUnknown,
					Err:  err,
				}
			}
			return User{}, &AuthRepositoryError{
				Code: AuthRepositoryErrorAlreadyExists,
				Err:  err,
			}
		}
		return User{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	return restoreUser(user), nil
}

func (r *DefaultAuthRepository) GetUserByUsername(ctx context.Context, username string) (User, *AuthRepositoryError) {
	user, err := r.db.GetUserByUsername(ctx, username)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, &AuthRepositoryError{
				Code: AuthRepositoryErrorNotFound,
				Err:  err,
			}
		}
		return User{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	return restoreUser(user), nil
}

func (r *DefaultAuthRepository) CreateSession(ctx context.Context, param authdb.CreateSessionParams) (Session, *AuthRepositoryError) {
	dbSession, err := r.db.CreateSession(ctx, param)
	if err != nil {
		return Session{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	dbUser, err := r.db.GetUserById(ctx, dbSession.UserID)
	if err != nil {
		return Session{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	return restoreSession(dbSession, dbUser), nil
}

func (r *DefaultAuthRepository) GetSessionByToken(ctx context.Context, sessionToken string) (Session, *AuthRepositoryError) {
	dbSession, err := r.db.GetSessionByToken(ctx, sessionToken)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Session{}, &AuthRepositoryError{
				Code: AuthRepositoryErrorNotFound,
				Err:  err,
			}
		}
		return Session{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	dbUser, err := r.db.GetUserById(ctx, dbSession.UserID)
	if err != nil {
		return Session{}, &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}

	return restoreSession(dbSession, dbUser), nil
}

func (r *DefaultAuthRepository) DeleteExpiredSessions(ctx context.Context) *AuthRepositoryError {
	err := r.db.DeleteExpiredSessions(ctx)
	if err != nil {
		return &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}
	return nil
}

func (r *DefaultAuthRepository) DeleteSession(ctx context.Context, sessionToken string) *AuthRepositoryError {
	err := r.db.DeleteSession(ctx, sessionToken)
	if err != nil {
		return &AuthRepositoryError{
			Code: AuthRepositoryErrorUnknown,
			Err:  err,
		}
	}
	return nil
}

// Internal

func restoreUser(user authdb.User) User {
	return User{
		ID:           user.ID,
		Username:     user.Username,
		Role:         RoleFromString(user.Role),
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func restoreSession(session authdb.Session, user authdb.User) Session {
	return Session{
		ID:           session.ID,
		User:         restoreUser(user),
		SessionToken: session.SessionToken,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}
}
