package authhttp

import (
	"context"
	"time"

	authdomain "demo/internal/auth/domain"
	utils "demo/internal/utils"

	"github.com/gin-gonic/gin"
)

type AuthHandlers struct {
	txManager   utils.TransactionManager
	authService *authdomain.AuthService
}

func InstallHandlers(r gin.IRouter, txManager utils.TransactionManager, middlewares []StrictMiddlewareFunc) {
	handlers := &AuthHandlers{
		txManager:   txManager,
		authService: authdomain.NewAuthService(),
	}
	RegisterHandlers(r, NewStrictHandler(handlers, middlewares))
}

func (s *AuthHandlers) Signup(ctx context.Context, req SignupRequestObject) (SignupResponseObject, error) {
	return utils.Transactional(ctx, s.txManager, func(dbtx utils.DBTX) (SignupResponseObject, error) {
		repo := authdomain.NewDefaultRepository(&dbtx)

		role := authdomain.RoleReader
		if req.Body.Roles != nil {
			switch *req.Body.Roles {
			case Sysadmin:
				role = authdomain.RoleSystemAdmin
			case Admin:
				role = authdomain.RoleAdmin
			case Writer:
				role = authdomain.RoleWriter
			case Reader:
				role = authdomain.RoleReader
			}
		}

		err := s.authService.RegisterUser(ctx, repo, req.Body.Username, req.Body.Password, role)
		if err != nil {
			switch err.Code {
			case authdomain.RegisterUserErrorAlreadyExists:
				return Signup400JSONResponse("ユーザー名が既に使われています"), nil
			case authdomain.RegisterUserErrorUnknown:
				return nil, err
			}
		}

		return Signup200JSONResponse("OK"), nil
	})
}

func (s *AuthHandlers) Login(ctx context.Context, req LoginRequestObject) (LoginResponseObject, error) {
	return utils.Transactional(ctx, s.txManager, func(dbtx utils.DBTX) (LoginResponseObject, error) {
		repo := authdomain.NewDefaultRepository(&dbtx)

		lifetime := 24 * 60 * 60
		expiresAt := time.Now().Add(24 * time.Hour)

		token, err := s.authService.Login(ctx, repo, req.Body.Username, req.Body.Password, expiresAt)
		if err != nil {
			switch err.Code {
			case authdomain.LoginErrorUserNotFound:
			case authdomain.LoginErrorInvalidCredentials:
				return Login401JSONResponse("ユーザー名かパスワードが一致しません"), nil
			case authdomain.LoginErrorUnknown:
				panic(err)
			}
		}

		ginCtx := ctx.(*gin.Context)
		ginCtx.SetCookie("session_token", token, lifetime, "/", "", false, true)

		return Login200JSONResponse("OK"), nil
	})
}

func (s *AuthHandlers) Logout(ctx context.Context, request LogoutRequestObject) (LogoutResponseObject, error) {
	var resp LogoutResponseObject
	ginCtx := ctx.(*gin.Context)
	token, err := ginCtx.Cookie("session_token")
	if err != nil {
		resp = Logout400JSONResponse("ログインしていません")
		return resp, nil
	}

	return utils.Transactional(ctx, s.txManager, func(dbtx utils.DBTX) (LogoutResponseObject, error) {
		repo := authdomain.NewDefaultRepository(&dbtx)

		if err := s.authService.Logout(ctx, repo, token); err != nil {
			switch err.Code {
			case authdomain.LogoutErrorUnknown:
				panic(err)
			}
		}

		return Logout200JSONResponse("OK"), nil
	})
}
