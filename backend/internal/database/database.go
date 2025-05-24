package database

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Initialize sets up the database connection based on the DATABASE_TYPE environment variable
func Initialize() {
	var err error
	
	dbType := os.Getenv("DATABASE_TYPE")
	if dbType == "" {
		dbType = "sqlite" // default to SQLite
	}
	
	switch dbType {
	case "postgres":
		DB, err = connectPostgres()
	case "sqlite":
		DB, err = connectSQLite()
	default:
		log.Fatalf("Unsupported database type: %s", dbType)
	}
	
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Auto-migrate the schema
	err = DB.AutoMigrate(&Media{}, &User{})
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}
	
	log.Printf("✅ Database initialized with %s", dbType)
}

func connectPostgres() (*gorm.DB, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)
	
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
}

func connectSQLite() (*gorm.DB, error) {
	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "/app/data/viewra.db"
	}
	
	// Create directory if it doesn't exist
	os.MkdirAll("/app/data", 0755)
	
	return gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
