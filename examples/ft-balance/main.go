// Command ft-balance prints the human-readable FT balance of an
// account using the TokenRegistry (ft_metadata + ft_balance_of).
//
//	FASTNEAR_API_KEY=... go run ./ft-balance a.near wrap.near
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/recipes"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: ft-balance <account_id> <token_contract>")
		os.Exit(2)
	}
	accountID, tokenID := os.Args[1], os.Args[2]

	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reg := recipes.NewTokenRegistry(client)
	bal, found, err := reg.BalanceOf(ctx, accountID, tokenID)
	if err != nil {
		fatal(err)
	}
	if !found {
		fmt.Printf("%s holds no %s\n", accountID, tokenID)
		return
	}
	fmt.Printf("%s: %s %s (raw %s)\n", accountID,
		bal.Format(4), bal.Meta.Symbol, bal.Raw)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
