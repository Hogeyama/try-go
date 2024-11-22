package authdomain

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	authdb "demo/internal/auth/db"
)

type AuthService struct {
}

func NewAuthService() *AuthService {
	return &AuthService{}
}

// ----------------------------------------------------------------------------
// RegisterUser / Signup

func (s *AuthService) RegisterUser(ctx context.Context, repo AuthRepository, username, password string, role Role) *RegisterUserError {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return &RegisterUserError{
			Code: RegisterUserErrorUnknown,
			Err:  err,
		}
	}

	_, authErr := repo.CreateUser(ctx,
		authdb.CreateUserParams{
			Username:     username,
			PasswordHash: string(passwordHash),
			Role:         role.String(),
		},
	)
	if authErr != nil {
		switch authErr.Code {
		case AuthRepositoryErrorAlreadyExists:
			return &RegisterUserError{
				Code: RegisterUserErrorAlreadyExists,
				Err:  errors.New("user already exists"),
			}
		case AuthRepositoryErrorUnknown, AuthRepositoryErrorNotFound:
			return &RegisterUserError{
				Code: RegisterUserErrorUnknown,
				Err:  authErr,
			}
		}
	}

	return nil
}

type RegisterUserError struct {
	Code RegisterUserErrorCode
	Err  error
}

type RegisterUserErrorCode int

const (
	RegisterUserErrorUnknown RegisterUserErrorCode = iota
	RegisterUserErrorAlreadyExists
)

func (e *RegisterUserError) Error() string {
	return e.Err.Error()
}

// ----------------------------------------------------------------------------
// Login

func (s *AuthService) Login(ctx context.Context, repo AuthRepository, username, password string, expiresAt time.Time) (string, *LoginError) {
	user, authErr := repo.GetUserByUsername(ctx, username)
	if authErr != nil {
		switch authErr.Code {
		case AuthRepositoryErrorNotFound:
			return "", loginError(LoginErrorUserNotFound, errors.New("user not found"))
		case AuthRepositoryErrorAlreadyExists:
		case AuthRepositoryErrorUnknown:
			return "", loginError(LoginErrorUnknown, authErr.Err)
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", loginError(LoginErrorInvalidCredentials, errors.New("invalid credentials"))
	}

	// Generate session token
	token, err := generateSessionToken()
	if err != nil {
		return "", loginError(LoginErrorUnknown, err)
	}

	// Create session
	_, authErr = repo.CreateSession(ctx, authdb.CreateSessionParams{
		UserID:       user.ID,
		SessionToken: token,
		ExpiresAt:    expiresAt,
	})
	if authErr != nil {
		return "", loginError(LoginErrorUnknown, err)
	}

	return token, nil
}

type LoginError struct {
	Code LoginErrorCode
	Err  error
}
type LoginErrorCode int

const (
	LoginErrorUnknown LoginErrorCode = iota
	LoginErrorUserNotFound
	LoginErrorInvalidCredentials
)

func (e *LoginError) Error() string {
	return e.Err.Error()
}

func loginError(code LoginErrorCode, err error) *LoginError {
	return &LoginError{
		Code: code,
		Err:  err,
	}
}

// ----------------------------------------------------------------------------
// Logout

func (s *AuthService) Logout(ctx context.Context, repo AuthRepository, token string) *LogoutError {
	err := repo.DeleteSession(ctx, token)
	if err != nil {
		return &LogoutError{
			Code: LogoutErrorUnknown,
			Err:  err,
		}
	}
	return nil
}

type LogoutError struct {
	Code LogoutErrorCode
	Err  error
}

type LogoutErrorCode int

const (
	LogoutErrorUnknown LogoutErrorCode = iota
)

func (e *LogoutError) Error() string {
	return e.Err.Error()
}

// ----------------------------------------------------------------------------
// Validate Session

func (s *AuthService) ValidateSession(ctx context.Context, repo AuthRepository, token string) (Session, *ValidateSessionError) {
	session, err := repo.GetSessionByToken(ctx, token)
	if err != nil {
		switch err.Code {
		case AuthRepositoryErrorNotFound:
			return Session{}, &ValidateSessionError{
				Code: ValidateSessionErrorNotFound,
				Err:  errors.New("session not found"),
			}
		case AuthRepositoryErrorUnknown:
		case AuthRepositoryErrorAlreadyExists:
			return Session{}, &ValidateSessionError{
				Code: ValidateSessionErrorUnknown,
				Err:  err,
			}
		}
	}
	return session, nil
}

type ValidateSessionError struct {
	Code ValidateSessionErrorCode
	Err  error
}

type ValidateSessionErrorCode int

const (
	ValidateSessionErrorUnknown ValidateSessionErrorCode = iota
	ValidateSessionErrorNotFound
)

func (e *ValidateSessionError) Error() string {
	return e.Err.Error()
}

// ----------------------------------------------------------------------------
// Internal

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
