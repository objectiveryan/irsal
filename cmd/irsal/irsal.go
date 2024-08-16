package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/carlmjohnson/flowmatic"

	"github.com/objectiveryan/irsal/internal/db"
	"github.com/objectiveryan/irsal/internal/hyp"
	"github.com/objectiveryan/irsal/internal/poller"
	"github.com/objectiveryan/irsal/internal/tbot"
)

func flagError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	token := flag.String("token", "", "Telegram bot token")
	dbpath := flag.String("db", "", "Path to database file")
	flag.Parse()

	if *token == "" {
		flagError("No Telegram bot token given")
	}
	if *dbpath == "" {
		flagError("No db path given")
	}
	if len(flag.Args()) > 0 {
		flagError("Unexpected argument: %q", flag.Arg(0))
	}

	fmt.Println("main()")
	storage, err := db.NewSqliteStorage(*dbpath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	hypFactory := hyp.NewClientFactory()
	b := &tbot.Bot{
		*token,
		storage,
		hypFactory,
	}
	br := tbot.NewBotRunner(b)
	p := &poller.Poller{
		hypFactory,
		storage,
		br,
	}
	err = flowmatic.All(context.Background(), p.Run, br.Run)
	if err != nil {
		log.Fatal(err)
	}
}
