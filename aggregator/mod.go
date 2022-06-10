package aggregator

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nkcr/OSIA/instagram"
	"github.com/nkcr/OSIA/instagram/types"
	"github.com/rs/zerolog"
	"github.com/tidwall/buntdb"
)

type Aggregator interface {
	Start(interval time.Duration) error
	Stop()
}

func NewBasicAggregator(db *buntdb.DB, api instagram.InstagramAPI,
	logger zerolog.Logger) Aggregator {

	logger = logger.With().Str("role", "aggregator").Logger()

	return &BasicAggregator{
		db:     db,
		api:    api,
		quit:   make(chan struct{}),
		logger: logger,
	}
}

// Aggregator implements a basic Aggregator
//
// - implements aggregator.Aggregator
type BasicAggregator struct {
	sync.Mutex
	db     *buntdb.DB
	api    instagram.InstagramAPI
	logger zerolog.Logger
	quit   chan struct{}
}

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
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update the db: %v", err)
	}

	return nil
}

func (a *BasicAggregator) Stop() {
	a.quit <- struct{}{}
}
