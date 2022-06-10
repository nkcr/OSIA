package aggregator

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/nkcr/OSIA/instagram"
	"github.com/nkcr/OSIA/instagram/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/buntdb"
)

func TestStartFail(t *testing.T) {
	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	instagram := fakeInstagram{
		refreshErr: errors.New("fake"),
	}

	logger := zerolog.New(io.Discard)

	agg := NewBasicAggregator(db, instagram, logger)

	err = agg.Start(time.Second)
	require.EqualError(t, err, "failed to update medias: failed to refresh token: fake")
}

func TestStartStop(t *testing.T) {
	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	instagram := fakeInstagram{}

	logger := zerolog.New(io.Discard)

	agg := NewBasicAggregator(db, instagram, logger)

	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		defer wait.Done()

		err = agg.Start(time.Millisecond)
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 10)
	agg.Stop()

	wait.Wait()
}

func TestUpdateMediasRefreshFail(t *testing.T) {
	instagram := fakeInstagram{
		refreshErr: errors.New("fake"),
	}

	agg := BasicAggregator{
		api: instagram,
	}

	err := agg.updateMedias()
	require.EqualError(t, err, "failed to refresh token: fake")
}

func TestUpdateMediasGetMediasFail(t *testing.T) {
	instagram := fakeInstagram{
		mediasErr: errors.New("fake"),
	}

	agg := BasicAggregator{
		api: instagram,
	}

	err := agg.updateMedias()
	require.EqualError(t, err, "failed to get medias: fake")
}

func TestUpdateMediaGetMediaError(t *testing.T) {
	medias := types.Medias{
		Data: []types.Media{
			{ID: "aa"},
			{ID: "bb"},
		},
	}

	instagram := fakeInstagram{
		medias:   medias,
		mediaErr: errors.New("fake"),
	}

	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	agg := BasicAggregator{
		api: instagram,
		db:  db,
	}

	err = agg.updateMedias()
	require.EqualError(t, err, "failed to get media: fake")
}

func TestUpdateMediasSuccess(t *testing.T) {
	medias := types.Medias{
		Data: []types.Media{
			{ID: "aa"},
			{ID: "bb"},
		},
	}

	instagram := fakeInstagram{
		medias: medias,
	}

	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	agg := BasicAggregator{
		api: instagram,
		db:  db,
	}

	err = agg.updateMedias()
	require.NoError(t, err)
}

// ----------------------------------------------------------------------------
// Utility functions

type fakeInstagram struct {
	instagram.InstagramAPI
	refreshErr error
	mediasErr  error
	mediaErr   error
	medias     types.Medias
}

func (i fakeInstagram) RefreshToken() error {
	return i.refreshErr
}

func (i fakeInstagram) GetMedias() (types.Medias, error) {
	return i.medias, i.mediasErr
}

func (i fakeInstagram) GetMedia(id string) (types.Media, error) {
	for _, media := range i.medias.Data {
		if media.ID == id {
			return media, i.mediaErr
		}
	}

	return types.Media{}, fmt.Errorf("media not found")
}
