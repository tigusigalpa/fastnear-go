// Command account-history streams the indexed transaction history of
// an account (most recent first). Usage:
//
//	FASTNEAR_API_KEY=... go run ./account-history intents.near 20
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/tx"
)

func main() {
	accountID := "intents.near"
	limit := 20
	if len(os.Args) > 1 {
		accountID = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil {
			limit = n
		}
	}
	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	desc, lim := true, uint32(50)
	q := tx.NewAccountHistoryQuery(accountID)
	q.Desc, q.Limit = &desc, &lim

	n := 0
	for row, err := range client.Tx().AccountHistorySeq(ctx, q) {
		if err != nil {
			fatal(err)
		}
		fmt.Printf("%d  %s  signer=%v receiver=%v success=%v\n",
			row.TxBlockHeight, row.TransactionHash,
			row.IsSigner, row.IsReceiver, row.IsSuccess)
		if n++; n >= limit {
			break
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
