package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

type HTTP interface {
	Start() error
	Stop()
	GetAddr() net.Addr
}

type key int

const requestIDKey key = 0
const maxMedias = 12

func NewNativeHTTP(addr string, db *buntdb.DB, logger zerolog.Logger) HTTP {
	logger = logger.With().Str("role", "http").Logger()
	logger.Info().Msg("Server is starting...")

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/medias", getMedias(db))

	server := &http.Server{
		Addr:         addr,
		Handler:      tracing(nextRequestID)(logging(logger)(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return &HTTPAPI{
		logger: logger,
		server: server,
		quit:   make(chan struct{}),
	}
}

type HTTPAPI struct {
	logger zerolog.Logger
	server *http.Server
	quit   chan struct{}
	ln     net.Listener
}

func (n *HTTPAPI) Start() error {
	ln, err := net.Listen("tcp", n.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to create conn '%s': %v", n.server.Addr, err)
	}

	n.ln = ln

	done := make(chan bool)

	go func() {
		<-n.quit
		n.logger.Info().Msg("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		n.server.SetKeepAlivesEnabled(false)

		err := n.server.Shutdown(ctx)
		if err != nil {
			n.logger.Err(err).Msg("Could not gracefully shutdown the server")
		}
		close(done)
	}()

	n.logger.Info().Msgf("Server is ready to handle requests at %s", ln.Addr().String())

	err = n.server.Serve(ln)
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to listen on %s: %v", ln.Addr().String(), err)
	}

	<-done
	n.logger.Info().Msg("Server stopped")

	return nil
}

func (n HTTPAPI) Stop() {
	n.logger.Info().Msg("stopping")
	// we don't close it so it can be called multiple times without harm
	select {
	case n.quit <- struct{}{}:
	default:
	}
}

func (n HTTPAPI) GetAddr() net.Addr {
	if n.ln == nil {
		return nil
	}

	return n.ln.Addr()
}

func getMedias(db *buntdb.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		count := maxMedias

		countStr := r.URL.Query().Get("count")
		if countStr != "" {
			c, err := strconv.Atoi(countStr)
			if err != nil || c < 1 {
				http.Error(w, "bad count value: "+countStr, http.StatusBadRequest)
				return
			}

			count = c

			if count > maxMedias {
				count = maxMedias
			}
		}

		result := make([]json.RawMessage, count)
		i := 0

		err := db.View(func(tx *buntdb.Tx) error {
			tx.Descend("timestamp", func(key, value string) bool {
				result[i] = []byte(value)
				i++

				return i < count
			})

			return nil
		})

		if err != nil {
			http.Error(w, fmt.Errorf("failed to view the db: %v", err).Error(),
				http.StatusInternalServerError)
			return
		}

		result = result[:i]

		w.Header().Add("Content-Type", "application/json")

		encoder := json.NewEncoder(w)

		err = encoder.Encode(result)
		if err != nil {
			http.Error(w, fmt.Errorf("failed to encode: %v", err).Error(),
				http.StatusInternalServerError)
			return
		}
	}
}

// logging is a utility function that logs the http server events
func logging(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Info().Str("requestID", requestID).
					Str("method", r.Method).
					Str("url", r.URL.Path).
					Str("remoteAddr", r.RemoteAddr).
					Str("agent", r.UserAgent()).Msg("")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// tracing is a utility function that adds header tracing
func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
