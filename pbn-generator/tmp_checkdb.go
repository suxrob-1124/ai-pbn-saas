//go:build ignore
// +build ignore

package main
import (
    "context"
    "database/sql"
    "fmt"
    _ "github.com/jackc/pgx/v5/stdlib"
    "golang.org/x/crypto/bcrypt"
)
func main(){
    db, err := sql.Open("pgx", "postgres://auth:auth@localhost:5432/auth?sslmode=disable")
    if err != nil { panic(err) }
    defer db.Close()
    var hash []byte
    err = db.QueryRowContext(context.Background(), "SELECT password_hash FROM users WHERE email=$1", "admin@example.com").Scan(&hash)
    if err != nil { panic(err) }
    fmt.Println("len", len(hash))
    fmt.Println("compare", bcrypt.CompareHashAndPassword(hash, []byte("Admin123!")))
}
