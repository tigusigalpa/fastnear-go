// Command send-tx submits an ALREADY SIGNED base64 transaction and
// waits for final execution. Signing (Borsh + ed25519) is out of
// scope of this SDK — produce the signed payload with another tool.
//
//	FASTNEAR_API_KEY=... go run ./send-tx <signed_tx_base64>
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/rpc"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: send-tx <signed_tx_base64>")
		os.Exit(2)
	}
	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	outcome, err := client.RPC().SendTx(ctx, os.Args[1], rpc.WaitFinal)
	if err != nil {
		fatal(err)
	}
	if outcome != nil {
		fmt.Println("final_execution_status:", outcome.FinalExecutionStatus)
		if st := outcome.Status(); st != nil {
			fmt.Println("success:", st.IsSuccess())
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
