package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/nkcr/OSIA/aggregator"
	"github.com/nkcr/OSIA/httpapi"
	"github.com/nkcr/OSIA/instagram"
	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

const tokenKey = "INSTAGRAM_TOKEN"

var logout = zerolog.ConsoleWriter{
	Out:        os.Stdout,
	TimeFormat: time.RFC3339,
}

func main() {
	var interval time.Duration
	flag.DurationVar(&interval, "interval", time.Hour, "Refresh interval for the aggregator")
	flag.Parse()

	var logger = zerolog.New(logout).Level(zerolog.InfoLevel).
		With().Timestamp().Logger().
		With().Caller().Logger()

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

	api := instagram.NewHTTPAPI(token, http.DefaultClient)

	agg := aggregator.NewBasicAggregator(db, api, logger)
	httpserver := httpapi.NewNativeHTTP(":3333", db, logger)

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
	logger.Info().Msg("done")
}
