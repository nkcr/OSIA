package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
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

// args defines the CLI arguments. You can always use -h to see the help.
type args struct {
	Interval     time.Duration `short:"i" long:"interval" default:"1h" description:"Refresh interval used by the Aggregator."`
	DBFilePath   string        `short:"d" long:"dbfilepath" default:"osia.db" description:"File path of the database."`
	ImagesFolder string        `short:"j" long:"imagesfolder" description:"Folder used to saved images. By default it uses $HOME/.OSIA/images."`
	HTTPListen   string        `short:"l" long:"listen" default:"0.0.0.0:3333" description:"The listen address of the HTTP server that servers the API."`
	Version      bool          `short:"v" long:"version" description:"Displays the version."`
}

func main() {
	var args args
	parser := flags.NewParser(&args, flags.Default)

	remaining, err := parser.Parse()
	if err != nil {
		flagsErr, ok := err.(*flags.Error)
		if ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}

		fmt.Println("failed to parse arguments:", err.Error())
		os.Exit(1)
	}

	if len(remaining) != 0 {
		fmt.Printf("unknown flags: %v\n", remaining)
		os.Exit(1)
	}

	if args.Version {
		fmt.Println("OSIA", Version, "-", BuildTime)
		os.Exit(0)
	}

	// set the default value for the imagesFolder argument
	if args.ImagesFolder == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(fmt.Sprintf("failed to get home dir: %v", err))
		}

		imagesFolder := filepath.Join(homeDir, ".OSIA", "images")
		args.ImagesFolder = imagesFolder
	}

	var logger = zerolog.New(logout).Level(zerolog.InfoLevel).
		With().Timestamp().Logger().
		With().Caller().Logger()

	logger.Info().Msgf("hi,\n"+
		"┌───────────────────────────────────────────────┐\n"+
		"│    ** Open Source Instagram Aggregator **\t│\n"+
		"├───────────────────────────────────────────────┤\n"+
		"│ Version %s │ Build time %s\t│\n"+
		"├───────────────────────────────────────────────┤\n"+
		"│ Interval %s\t│\n"+
		"├───────────────────────────────────────────────┤\n"+
		"│ DBFilePath %s\t│\n"+
		"├───────────────────────────────────────────────┤\n"+
		"│ ImagesFolder %s\t│\n"+
		"├───────────────────────────────────────────────┤\n"+
		"│ HTTPListen %s\t│\n"+
		"└───────────────────────────────────────────────┘\n",
		Version, BuildTime, args.Interval.String(), args.DBFilePath,
		args.ImagesFolder, args.HTTPListen)

	err = os.MkdirAll(filepath.Dir(args.DBFilePath), 0744)
	if err != nil {
		panic(fmt.Sprintf("failed to create db dir: %v", err))
	}

	db, err := buntdb.Open(args.DBFilePath)
	if err != nil {
		panic(err)
	}

	defer db.Close()

	err = db.CreateIndex("timestamp", "*", buntdb.IndexJSON("timestamp"))
	if err != nil {
		panic(err)
	}

	token := os.Getenv(tokenKey)
	if token == "" {
		panic(fmt.Sprintf("please set the %s variable", tokenKey))
	}

	err = os.MkdirAll(args.ImagesFolder, 0744)
	if err != nil {
		panic(fmt.Sprintf("failed to create config dir: %v", err))
	}

	client := http.DefaultClient

	api := instagram.NewHTTPAPI(token, client)

	agg := aggregator.NewInstagramAggregator(db, api, args.ImagesFolder, client, logger)
	httpserver := httpapi.NewInstagramHTTP(args.HTTPListen, db, args.ImagesFolder, logger)

	wait := sync.WaitGroup{}

	wait.Add(1)
	go func() {
		defer wait.Done()
		err = agg.Start(args.Interval)
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
