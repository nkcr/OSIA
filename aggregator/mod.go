package aggregator

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nkcr/OSIA/instagram"
	"github.com/nkcr/OSIA/instagram/types"
	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

// Aggregator defines the primitives required for an Aggregator.
type Aggregator interface {
	// Start should start a goroutine that periodically fetches content on
	// Instagram and update the local database accordingly.
	Start(interval time.Duration) error

	// Stop should stop the periodical update and free resources.
	Stop()
}

// HTTPClient defines the primitive needed to perform HTTP queries
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// NewBasicAggregator returns a new initialized basic Aggregator.
func NewBasicAggregator(db *buntdb.DB, api instagram.InstagramAPI,
	imagesFolder string, client HTTPClient, logger zerolog.Logger) Aggregator {

	logger = logger.With().Str("role", "aggregator").Logger()

	return &BasicAggregator{
		db:           db,
		api:          api,
		quit:         make(chan struct{}),
		logger:       logger,
		imagesFolder: imagesFolder,
		client:       client,
	}
}

// Aggregator implements a basic Aggregator
//
// - implements aggregator.Aggregator
type BasicAggregator struct {
	sync.Mutex
	db           *buntdb.DB
	api          instagram.InstagramAPI
	logger       zerolog.Logger
	quit         chan struct{}
	imagesFolder string
	client       HTTPClient
}

// Start implements aggregator.Aggregator. It should be called only if the
// aggregator is not already running.
func (a *BasicAggregator) Start(interval time.Duration) error {
	a.logger.Info().Msg("aggregator starting")

	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	for {
		a.logger.Info().Msg("updating media")

		err := a.updateMedias()
		if err != nil {
			return fmt.Errorf("failed to update medias: %v", err)
		}

		select {
		case <-a.quit:
			return nil
		case <-ticker.C:
			continue
		}
	}
}

func (a *BasicAggregator) updateMedias() error {
	a.logger.Info().Msg("refreshing token")
	err := a.api.RefreshToken()
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}

	medias, err := a.api.GetMedias()
	if err != nil {
		return fmt.Errorf("failed to get medias: %v", err)
	}

	toAdd := []string{}

	err = a.db.View(func(tx *buntdb.Tx) error {
		for _, media := range medias.Data {
			_, err = tx.Get(media.ID)
			if err != nil {
				toAdd = append(toAdd, media.ID)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to view the db: %v", err)
	}

	a.logger.Info().Msgf("%d media to add", len(toAdd))

	newMedias := make([]types.Media, len(toAdd))

	for i, id := range toAdd {
		media, err := a.api.GetMedia(id)
		if err != nil {
			return fmt.Errorf("failed to get media: %v", err)
		}

		newMedias[i] = media
	}

	err = a.db.Update(func(tx *buntdb.Tx) error {
		for _, media := range newMedias {
			buf, err := json.Marshal(media)
			if err != nil {
				return fmt.Errorf("failed to marshal media: %v", err)
			}

			_, _, err = tx.Set(media.ID, string(buf), &buntdb.SetOptions{})
			if err != nil {
				return fmt.Errorf("failed to set: %v", err)
			}

			a.logger.Info().Msgf("new media '%s' added", media.ID)

			imagePath := filepath.Join(a.imagesFolder, media.ID+".jpg")

			err = saveImage(media.MediaURL, imagePath, a.client)
			if err != nil {
				return fmt.Errorf("failed to save image: %v", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update the db: %v", err)
	}

	return nil
}

func saveImage(url, path string, client HTTPClient) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get URL '%s': %v", url, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("http request failed with status %s: %s", resp.Status, buf)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %v", path, err)
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to copy bytes: %v", err)
	}

	return nil
}

// Stop implements aggregator.Aggregator. It should be called only if the
// Aggregator is started.
func (a *BasicAggregator) Stop() {
	a.quit <- struct{}{}
}
