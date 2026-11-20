// Command snapshot prints a complete account overview: indexed
// balances plus on-chain state. Usage:
//
//	FASTNEAR_API_KEY=... go run ./snapshot root.near
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
	accountID := "root.near"
	if len(os.Args) > 1 {
		accountID = os.Args[1]
	}
	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	snap, err := recipes.New(client).AccountSnapshot(ctx, accountID)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("account:  %s\n", accountID)
	if snap.Balance != nil {
		fmt.Printf("balance:  %s\n", snap.Balance)
		fmt.Printf("locked:   %s\n", snap.Locked)
	}
	fmt.Printf("storage:  %d bytes\n", snap.StorageUsage)
	if snap.Full != nil {
		fmt.Printf("ft tokens: %d, nfts: %d, pools: %d\n",
			len(snap.Full.Tokens), len(snap.Full.NFTs), len(snap.Full.Pools))
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
