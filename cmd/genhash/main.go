// genhash generates bcrypt password hashes for ViewRA user accounts.
//
// This utility is used to create hashed passwords for manual user setup,
// such as seeding the database or resetting passwords directly in SQL.
//
// Usage:
//
//	go run ./cmd/genhash [password]
//
// If no password is provided, it defaults to "dev" for development purposes.
//
// Example:
//
//	$ go run ./cmd/genhash mysecretpassword
//	$2a$10$...
//
// The output can be inserted directly into the users table password_hash column.
package main

import (
	"fmt"
	"os"

	"github.com/mantonx/viewra/internal/infrastructure/auth"
)

func main() {
	password := "dev"
	if len(os.Args) > 1 {
		password = os.Args[1]
	}

	hasher := auth.NewPasswordHasher(nil)
	hash, err := hasher.Hash(password)
	if err != nil {
		panic(err)
	}
	fmt.Println(hash)
}
