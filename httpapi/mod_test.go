package httpapi

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/nkcr/OSIA/instagram/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/buntdb"
)

// This test performs a simple scenario. It starts the server and makes an HTTP
// request. The process should not return any error.
func TestScenario(t *testing.T) {
	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	logger := zerolog.New(io.Discard)

	httpapi := NewNativeHTTP("localhost:0", db, logger)

	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		defer wait.Done()
		err = httpapi.Start()
		require.NoError(t, err)
	}()

	defer func() {
		t.Log("stopping")
		httpapi.Stop()
		wait.Wait()
		t.Log("stopped")
	}()

	time.Sleep(time.Second * 1)

	addr := httpapi.GetAddr()
	require.NotNil(t, addr)

	url := "http://" + addr.String() + "/api/medias"
	t.Logf("fetching url %s", url)

	resp, err := http.Get(url)
	require.NoError(t, err)

	require.Equal(t, 200, resp.StatusCode)
}

func TestWrongAddr(t *testing.T) {
	a := HTTPAPI{
		server: &http.Server{Addr: "x"},
	}

	err := a.Start()
	require.EqualError(t, err, "failed to create conn 'x': listen tcp: address x: missing port in address")
}

// If the listener is nil, the server should return a nil address.
func TestGetAddr(t *testing.T) {
	a := HTTPAPI{}

	addr := a.GetAddr()
	require.Nil(t, addr)
}

func TestGetMedias(t *testing.T) {
	db, err := buntdb.Open(":memory:")
	require.NoError(t, err)

	err = db.CreateIndex("timestamp", "*", buntdb.IndexJSON("timestamp"))
	require.NoError(t, err)

	n := 20
	medias := make([]types.Media, n)
	for i := range medias {
		media := getRandomMedia(t)
		medias[i] = media

		mediaBuf, err := json.Marshal(&media)
		require.NoError(t, err)

		err = db.Update(func(tx *buntdb.Tx) error {
			_, _, err = tx.Set(media.ID, string(mediaBuf), nil)
			return err
		})
		require.NoError(t, err)
	}

	handler := getMedias(db)

	t.Run("Get Medias without count", getTestWithtoutCount(db, medias, handler))
	t.Run("Get Medias with count", getTestWithCount(db, medias, handler))
	t.Run("Get Medias with wrong count", getTestWithWrongCount(db, medias, handler))
	t.Run("Get Medias with over maximum count", getTestWithOverMaximumCount(db, medias, handler))
}

func getTestWithtoutCount(db *buntdb.DB, medias []types.Media,
	handler func(http.ResponseWriter, *http.Request)) func(t *testing.T) {

	return func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "", nil)
		require.NoError(t, err)

		handler(rr, req)
		require.Equal(t, 200, rr.Result().StatusCode)

		// the result should be sorted by timestamp
		sort.SliceStable(medias, func(i, j int) bool {
			return medias[i].Timestamp > medias[j].Timestamp
		})

		result := []types.Media{}

		err = json.Unmarshal(rr.Body.Bytes(), &result)
		require.NoError(t, err)

		// there should be the maximum of 12 medias
		require.Len(t, result, 12)

		require.Equal(t, medias[:12], result)
	}
}

func getTestWithCount(db *buntdb.DB, medias []types.Media,
	handler func(http.ResponseWriter, *http.Request)) func(t *testing.T) {

	return func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "http://example.com?count=5", nil)
		require.NoError(t, err)

		handler(rr, req)
		require.Equal(t, 200, rr.Result().StatusCode)

		// the result should be sorted by timestamp
		sort.SliceStable(medias, func(i, j int) bool {
			return medias[i].Timestamp > medias[j].Timestamp
		})

		result := []types.Media{}

		err = json.Unmarshal(rr.Body.Bytes(), &result)
		require.NoError(t, err)

		// there should be the count of 5
		require.Len(t, result, 5)

		require.Equal(t, medias[:5], result)
	}
}

func getTestWithWrongCount(db *buntdb.DB, medias []types.Media,
	handler func(http.ResponseWriter, *http.Request)) func(t *testing.T) {

	return func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "http://example.com?count=-1", nil)
		require.NoError(t, err)

		handler(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	}
}

func getTestWithOverMaximumCount(db *buntdb.DB, medias []types.Media,
	handler func(http.ResponseWriter, *http.Request)) func(t *testing.T) {

	return func(t *testing.T) {
		t.Parallel()

		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodGet, "http://example.com?count=50", nil)
		require.NoError(t, err)

		handler(rr, req)
		require.Equal(t, 200, rr.Result().StatusCode)

		// the result should be sorted by timestamp
		sort.SliceStable(medias, func(i, j int) bool {
			return medias[i].Timestamp > medias[j].Timestamp
		})

		result := []types.Media{}

		err = json.Unmarshal(rr.Body.Bytes(), &result)
		require.NoError(t, err)

		// there should be the maximum of 12
		require.Len(t, result, 12)

		require.Equal(t, medias[:12], result)
	}
}

// -----------------------------------------------------------------------------
// Utility functions

func getRandomMedia(t *testing.T) types.Media {
	buf := make([]byte, 7)

	n, err := rand.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 7, n)

	return types.Media{
		ID:        hex.EncodeToString(buf[0:1]),
		Caption:   hex.EncodeToString(buf[1:2]),
		MediaType: hex.EncodeToString(buf[2:3]),
		MediaURL:  hex.EncodeToString(buf[3:4]),
		Permalink: hex.EncodeToString(buf[4:5]),
		Username:  hex.EncodeToString(buf[5:6]),
		Timestamp: hex.EncodeToString(buf[6:7]),
	}
}
