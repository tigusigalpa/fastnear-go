// Command fastdata reads latest KV values by exact key and streams
// the latest values a contract holds.
//
//	FASTNEAR_API_KEY=... go run ./fastdata
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/kv"
)

func main() {
	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Exact-key read: keys contain '/', encoded as %2F automatically.
	list, err := client.KV().LatestByExactKey(ctx,
		"social.near", "james.near", "graph/follow/sleet.near", true)
	if err != nil {
		fatal(err)
	}
	if list != nil {
		for _, e := range list.Entries {
			fmt.Printf("key %s = %s\n", e.Key, string(e.Value))
		}
	}

	// Stream latest values of a contract (first 10).
	lim := uint32(10)
	n := 0
	for entry, err := range client.KV().LatestByAccountSeq(ctx, "social.near", kv.LatestQuery{Limit: &lim}) {
		if err != nil {
			fatal(err)
		}
		fmt.Printf("social.near key %s (block %d)\n", entry.Key, entry.BlockHeight)
		if n++; n >= 10 {
			break
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
