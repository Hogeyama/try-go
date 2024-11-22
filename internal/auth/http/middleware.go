package authhttp

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	strictgin "github.com/oapi-codegen/runtime/strictmiddleware/gin"

	authdomain "demo/internal/auth/domain"
	utils "demo/internal/utils"
)

func AuthRequired(txManager utils.TransactionManager) strictgin.StrictGinMiddlewareFunc {
	authService := authdomain.NewAuthService()

	return func(f strictgin.StrictGinHandlerFunc, operationID string) strictgin.StrictGinHandlerFunc {
		return func(ctx *gin.Context, request interface{}) (interface{}, error) {
			_roles, exists := ctx.Get(CookieAuthScopes)
			if !exists {
				// 認証不要
				return f(ctx, request)
			}
			roles := _roles.([]string)
			log.Printf("operation allowed for roles: %v\n", roles)

			token, err := ctx.Cookie(CookieName)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return nil, nil
			}

			return utils.Transactional(ctx, txManager, func(dbtx utils.DBTX) (interface{}, error) {
				repo := authdomain.NewDefaultRepository(&dbtx)
				session, authErr := authService.ValidateSession(ctx, repo, token)
				if authErr != nil {
					switch authErr.Code {
					case authdomain.ValidateSessionErrorNotFound:
						ctx.AbortWithStatusJSON(http.StatusUnauthorized,
							gin.H{"error": "unauthorized"})
						return nil, nil
					case authdomain.ValidateSessionErrorUnknown:
						ctx.AbortWithStatusJSON(http.StatusInternalServerError,
							gin.H{"error": "internal server error"})
						return nil, authErr
					}
				}

				has_role := false
				user_role := session.User.Role.String()
				for _, role := range roles {
					if role == user_role {
						has_role = true
						break
					}
				}
				if !has_role {
					ctx.AbortWithStatusJSON(http.StatusUnauthorized,
						gin.H{"error": "unauthorized"})
					return nil, nil
				}

				ctx.Set("user_id", session.User.ID)
				ctx.Set("username", session.User.Username)

				return f(ctx, request)
			})
		}
	}
}
