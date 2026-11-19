// Command admin runs maintenance tasks against the database in DATABASE_URL.
//
//	go run ./cmd/admin delete-test-accounts -email 'cookie-check-%@example.com' [-email …] [-yes]
//
// Without -yes it only lists what it would delete (a dry run). Run it from the
// backend/ folder. Instructions for production: docs/setup.md, section 8.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Jokubas-J/Eneba/backend/internal/admin"
	"github.com/Jokubas-J/Eneba/backend/internal/config"
	"github.com/Jokubas-J/Eneba/backend/internal/database"
)

// emails collects repeated -email flags.
type emails []string

func (e *emails) String() string     { return strings.Join(*e, ", ") }
func (e *emails) Set(v string) error { *e = append(*e, v); return nil }

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 || os.Args[1] != "delete-test-accounts" {
		log.Fatal("usage: admin delete-test-accounts -email PATTERN [-email PATTERN …] [-yes]")
	}
	flags := flag.NewFlagSet("delete-test-accounts", flag.ExitOnError)
	var patterns emails
	flags.Var(&patterns, "email", "SQL LIKE pattern of the accounts' e-mail, e.g. 'cookie-check-%@example.com' (repeatable)")
	yes := flags.Bool("yes", false, "really delete (without it: dry run)")
	if err := flags.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	if err := config.LoadDotEnv(".env"); err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	result, err := admin.DeleteTestAccounts(ctx, pool, patterns, *yes)
	if err != nil {
		log.Fatal(err)
	}
	verb := "Would delete"
	if *yes {
		verb = "Deleted"
	}
	fmt.Printf("%s %d accounts and %d orders; %d keys back on sale.\n", verb, len(result.Emails), result.Orders, result.KeysReturned)
	for _, e := range result.Emails {
		fmt.Println("  " + e)
	}
	if !*yes && len(result.Emails) > 0 {
		fmt.Println("Dry run: nothing changed. Add -yes to delete.")
	}
}
