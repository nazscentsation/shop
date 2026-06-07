// seed — one-time CLI to create the first admin account.
//
// Usage (from backend/ directory):
//   go run ./cmd/seed -email admin@example.com -password MySecret123
//
// The tool reads DATABASE_URL from the .env file or environment.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nazscentsation/shop/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	email    := flag.String("email",    "", "Admin email address (required)")
	password := flag.String("password", "", "Admin password, min 8 chars (required)")
	flag.Parse()

	if *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./cmd/seed -email <email> -password <password>")
		os.Exit(1)
	}
	if len(*password) < 8 {
		log.Fatal("password must be at least 8 characters")
	}

	_ = godotenv.Load("../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := database.Connect(dbURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	var id int64
	err = db.QueryRow(
		`INSERT INTO users (email, password_hash, first_name, last_name, role, email_verified)
		 VALUES ($1, $2, 'Admin', 'User', 'admin', TRUE)
		 ON CONFLICT (email) DO UPDATE SET
		   role = 'admin',
		   email_verified = TRUE,
		   password_hash = EXCLUDED.password_hash
		 RETURNING id`,
		*email, string(hash),
	).Scan(&id)
	if err != nil {
		log.Fatalf("insert admin: %v", err)
	}

	fmt.Printf("Admin account ready — email: %s  id: %d\n", *email, id)
}
