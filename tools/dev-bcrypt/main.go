// Command dev-bcrypt prints a bcrypt hash for the given password (default: password123).
// Used to document local-dev login hashes; do not use for production secrets.
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	in := "password123"
	if len(os.Args) > 1 {
		in = os.Args[1]
	}
	h, err := bcrypt.GenerateFromPassword([]byte(in), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(h))
}
