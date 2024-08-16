package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/objectiveryan/irsal/internal/common"
	"github.com/objectiveryan/irsal/internal/db"
)

func flagError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	token := flag.String("token", "", "Hypothesis token")
	group := flag.String("group", "", "Hypothesis group")
	timestr := flag.String("time", "", "Time of earliest Hypothesis annotation to consider, in RFC3339 format")
	chatID := flag.Int64("chat", 0, "Telegram chat ID")
	dbpath := flag.String("db", "", "Path to database file")
	flag.Parse()

	if *token == "" {
		flagError("No Hypothesis token given")
	}
	if *group == "" {
		flagError("No Hypothesis group given")
	}
	if *dbpath == "" {
		flagError("No db path given")
	}
	if *chatID == 0 {
		flagError("No chat ID given")
	}
	if len(flag.Args()) > 0 {
		flagError("Unexpected argument: %q", flag.Arg(0))
	}

	searchAfter := time.Now()
	if *timestr != "" {
		var err error
		searchAfter, err = time.Parse(time.RFC3339, *timestr)
		if err != nil {
			log.Fatalf("Failed to parse time: %v", err)
		}
	}

	storage, err := db.NewSqliteStorage(*dbpath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	err = storage.AddSubscription(&common.Subscription{
		*token, *group, searchAfter, *chatID})
	if err != nil {
		log.Fatalf("Failed to add subscription: %v", err)
	}
}
