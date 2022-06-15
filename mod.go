package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/nkcr/OSIA/aggregator"
	"github.com/nkcr/OSIA/httpapi"
	"github.com/nkcr/OSIA/instagram"
	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

// Version contains the current or build version. This variable can be changed
// at build time with:
//
//   go build -ldflags="-X 'main.Version=v1.0.0'"
//
// Version should be fetched from git: `git describe --tags`
var Version = "unknown"

// BuildTime indicates the time at which the binary has been built. Must be set
// as with Version.
var BuildTime = "unknown"

const tokenKey = "INSTAGRAM_TOKEN"

var logout = zerolog.ConsoleWriter{
	Out:        os.Stdout,
	TimeFormat: time.RFC3339,
}

func main() {
	fmt.Println("version;", Version)
	var interval time.Duration
	flag.DurationVar(&interval, "interval", time.Hour, "Refresh interval for the aggregator")
	flag.Parse()

	var logger = zerolog.New(logout).Level(zerolog.InfoLevel).
		With().Timestamp().Logger().
		With().Caller().Logger()

	logger.Info().Msgf("hi,\n┌───────────────────────────────────────────────┐\n│ Open Source Instagram Aggregator\t\t│\n├───────────────────────────────────────────────┤\n│ Version %s │ Build time %s\t│\n└───────────────────────────────────────────────┘\n", Version, BuildTime)
	logger.Info().Msgf("using the following refresh interval: %s", interval.String())

	db, err := buntdb.Open("db.db")
	if err != nil {
		panic(err)
	}

	err = db.CreateIndex("timestamp", "*", buntdb.IndexJSON("timestamp"))
	if err != nil {
		panic(err)
	}

	token := os.Getenv(tokenKey)
	if token == "" {
		panic(fmt.Sprintf("please set the %s variable", tokenKey))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home dir: %v", err))
	}

	imagesFolder := filepath.Join(homeDir, ".OSIA", "images")

	err = os.MkdirAll(imagesFolder, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("failed to create config dir: %v", err))
	}

	client := http.DefaultClient

	api := instagram.NewHTTPAPI(token, client)

	agg := aggregator.NewBasicAggregator(db, api, imagesFolder, client, logger)
	httpserver := httpapi.NewNativeHTTP(":3333", db, imagesFolder, logger)

	wait := sync.WaitGroup{}

	wait.Add(1)
	go func() {
		defer wait.Done()
		err = agg.Start(interval)
		if err != nil {
			logger.Err(err).Msg("failed to start the aggregator... exiting")
			os.Exit(1)
		}
		logger.Info().Msg("aggregator done")
	}()

	wait.Add(1)
	go func() {
		defer wait.Done()
		httpserver.Start()
		logger.Info().Msg("http server done")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	<-quit

	agg.Stop()
	httpserver.Stop()

	wait.Wait()
	db.Close()

	logger.Info().Msg("done")
}
