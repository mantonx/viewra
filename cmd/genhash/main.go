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
