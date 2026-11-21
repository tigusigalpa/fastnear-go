// Command watch-blocks polls the last final block height and prints
// every new block header as it finalizes. Ctrl-C to stop.
//
//	FASTNEAR_API_KEY=... go run ./watch-blocks
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	fastnear "github.com/tigusigalpa/fastnear-go"
	"github.com/tigusigalpa/fastnear-go/recipes"
)

func main() {
	client, err := fastnear.NewClient(fastnear.WithAPIKey(os.Getenv("FASTNEAR_API_KEY")))
	if err != nil {
		fatal(err)
	}
	defer client.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	svc := recipes.New(client)
	last, err := svc.LatestFinalHeight(ctx)
	if err != nil {
		fatal(err)
	}
	fmt.Println("starting at final height", last)

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		h, err := svc.LatestFinalHeight(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "poll:", err)
			continue
		}
		for last < h {
			next := last + 1
			env, err := svc.BlockHeaders(ctx, next)
			if err != nil {
				fmt.Fprintln(os.Stderr, "headers:", err)
				break
			}
			if env != nil {
				fmt.Printf("block %d by %s (%d chunks)\n",
					env.Header.Height, env.Author, len(env.Chunks))
			}
			last = next
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
