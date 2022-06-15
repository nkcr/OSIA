package aggregator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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

	agg := NewBasicAggregator(db, instagram, "", nil, logger)

	err = agg.Start(time.Second)
	require.EqualError(t, err, "failed to update medias: failed to refresh token: fake")
}

func TestStartStop(t *testing.T) {
	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	instagram := fakeInstagram{}

	logger := zerolog.New(io.Discard)

	tmpdir, err := ioutil.TempDir("", "OSIA")
	require.NoError(t, err)

	defer os.RemoveAll(tmpdir)

	agg := NewBasicAggregator(db, instagram, tmpdir, nil, logger)

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

func TestUpdateMediaSaveImageError(t *testing.T) {
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

	client := fakeClient{
		err: errors.New("fake"),
	}

	agg := BasicAggregator{
		api:    instagram,
		db:     db,
		client: client,
	}

	err = agg.updateMedias()
	require.EqualError(t, err, "failed to update the db: failed to save image: failed to get URL '': fake")
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

	tmpdir, err := ioutil.TempDir("", "OSIA")
	require.NoError(t, err)

	defer os.RemoveAll(tmpdir)

	image := []byte("fake image")
	client := fakeClient{
		body:       image,
		statusCode: 200,
	}

	agg := BasicAggregator{
		api:          instagram,
		db:           db,
		imagesFolder: tmpdir,
		client:       client,
	}

	err = agg.updateMedias()
	require.NoError(t, err)

	img, err := os.ReadFile(filepath.Join(tmpdir, "aa.jpg"))
	require.NoError(t, err)
	require.Equal(t, "fake image", string(img))
}

func TestSaveImageBadStatusCode(t *testing.T) {
	client := fakeClient{
		statusCode: 500,
		body:       []byte("fake body"),
	}

	err := saveImage("", "", client)
	require.EqualError(t, err, "http request failed with status 500: fake body")
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

type fakeClient struct {
	body       []byte
	err        error
	statusCode int
}

func (c fakeClient) Get(url string) (resp *http.Response, err error) {
	if c.err != nil {
		return nil, c.err
	}

	buff := bytes.NewBuffer(c.body)
	body := io.NopCloser(buff)

	return &http.Response{
		StatusCode: c.statusCode,
		Status:     strconv.Itoa(c.statusCode),
		Body:       body,
	}, nil
}
