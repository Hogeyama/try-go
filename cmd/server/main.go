package main

import (
	"context"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	strictgin "github.com/oapi-codegen/runtime/strictmiddleware/gin"

	authhttp "demo/internal/auth/http"
	"demo/internal/utils"
)

func main() {
	// Initialize database
	dbUrl, set := os.LookupEnv("DATABASE_URL")
	if !set {
		log.Fatal("DATABASE_URL is not set in .env file")
		return
	}
	if err := InitDB(dbUrl); err != nil {
		log.Fatal("Failed to initialize database:", err)
		return
	}
	defer Close()

	router := gin.Default()

	txManager := utils.NewTransactionManager(GetDB())
	authMiddleware := authhttp.AuthRequired(txManager)
	middlewares := []strictgin.StrictGinMiddlewareFunc{authMiddleware}

	authhttp.InstallHandlers(router, txManager, middlewares)

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}

}

var dbPool *pgxpool.Pool

func InitDB(url string) error {
	var err error
	dbPool, err = pgxpool.New(context.Background(), url)
	if err != nil {
		return err
	}

	// Test the connection
	if err := dbPool.Ping(context.Background()); err != nil {
		return err
	}

	return nil
}

func GetDB() *pgxpool.Pool {
	return dbPool
}

// Close closes the database connection pool.
func Close() {
	if dbPool != nil {
		dbPool.Close()
	}
}
