package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/RealImage/QLedger/controller"
	"github.com/RealImage/QLedger/handler"
	"github.com/RealImage/QLedger/models"
	"github.com/mattes/migrate"
	"github.com/mattes/migrate/database"
	"github.com/mattes/migrate/database/postgres"
	_ "github.com/mattes/migrate/source/file"
)

func main() {
	// Assert authentication
	authToken, ok := os.LookupEnv("LEDGER_AUTH_TOKEN")
	if !ok || authToken == "" {
		log.Fatal("Cannot start the server. Authentication token is not set!! Please set LEDGER_AUTH_TOKEN")
	}

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Panic("Unable to connect to Database:", err)
	}
	log.Println("Successfully established connection to database.")

	// Migrate DB changes
	migrateDB(db)

	accSearchEngine, appErr := models.NewSearchEngine(db, models.SearchNamespaceAccounts)
	if appErr != nil {
		log.Fatal(appErr)
	}

	trSearchEngine, appErr := models.NewSearchEngine(db, models.SearchNamespaceTransactions)
	if appErr != nil {
		log.Fatal(appErr)
	}

	accountDB := models.NewAccountDB(db)
	transactionDB := models.NewTransactionDB(db)
	accCtrl := controller.NewController(accSearchEngine, &accountDB, &transactionDB)
	trCtrl := controller.NewController(trSearchEngine, &accountDB, &transactionDB)
	hostPrefix := os.Getenv("HOST_PREFIX")
	router := handler.NewRouter(hostPrefix, accCtrl, trCtrl)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7000"
	}
	log.Println("Running server on port:", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func migrateDB(db *sql.DB) {
	log.Println("Starting db schema migration...")
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Panic("Unable to create database instance for migration:", err)
	}

	migrationFilesPath := os.Getenv("MIGRATION_FILES_PATH")
	if migrationFilesPath == "" {
		migrationFilesPath = "file://migrations/postgres"
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationFilesPath,
		"postgres", driver)
	if err != nil {
		log.Panic("Unable to create Migrate instance for database:", err)
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		log.Panic("Unable to get existing migration version for database:", dirty, err)
	}
	log.Println("Current schema version:", version)
	err = m.Up()
	if err != nil {
		switch err {
		case migrate.ErrNoChange:
			log.Println("No changes to migrate")
		case migrate.ErrLocked, database.ErrLocked:
			log.Println("Database locked. Skipping migration assuming another instance working on it")
		default:
			log.Panic("Error while migration:", err)
		}
	}
	version, dirty, err = m.Version()
	if err != nil {
		log.Panic("Unable to get new migration version for database:", dirty, err)
	}
	log.Println("Migrated schema version:", version)
}
