//go:build ignore
// +build ignore

package main
import (
    "fmt"
    "golang.org/x/crypto/bcrypt"
)
func main(){
    hash1 := []byte("$2a$10$5N1IRajuO0BXcKtFdVHFWuD2.WQYEr3eRmcOehXzZZ3I5x/aaA4Iq")
    fmt.Println("hash1", bcrypt.CompareHashAndPassword(hash1, []byte("Admin123!")))
    hash2 := []byte("$2a$10$Us9OH7YeZYOqUxZqESou5eAwEY73xgaVTYImUFR0ABRTPbaWSO8SS")
    fmt.Println("hash2", bcrypt.CompareHashAndPassword(hash2, []byte("Manager123!")))
}
