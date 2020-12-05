package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/grantfayvor/hexcord-mediaserver/lib"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	err = sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_CLIENT"),
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	defer sentry.Flush(2 * time.Second)

	lib.InitializeRoutes()

	fmt.Printf("Starting server at port 8090\n")
	if err := http.ListenAndServe(":8090", nil); err != nil {
		log.Fatal(err)
	}

}
