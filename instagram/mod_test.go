package instagram

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/nkcr/OSIA/instagram/types"
	"github.com/stretchr/testify/require"
)

func TestGetMediasGetFail(t *testing.T) {
	client := fakeHTTPClient{
		err: errors.New("fake"),
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedias()
	require.EqualError(t, err, "failed to get 'https://graph.instagram.com/me/media/?access_token=fake&fields=id': fake")
}

func TestGetMediasBadStatus(t *testing.T) {
	client := fakeHTTPClient{
		statusCode: 500,
		body:       []byte("body"),
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedias()
	require.EqualError(t, err, "http request failed with status 500: body")
}

func TestGetMediasFailUnmarshal(t *testing.T) {
	client := fakeHTTPClient{
		body:       []byte("invalid json"),
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedias()
	require.EqualError(t, err, "failed to decode response: invalid character 'i' looking for beginning of value")
}

func TestGetMediasSuccess(t *testing.T) {
	medias := types.Medias{
		Data: []types.Media{
			{ID: "aa"},
		},
	}

	buff, err := json.Marshal(&medias)
	require.NoError(t, err)

	client := fakeHTTPClient{
		body:       buff,
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	mediasResponse, err := api.GetMedias()
	require.NoError(t, err)

	require.Equal(t, medias, mediasResponse)

	expectedURL := "https://graph.instagram.com/me/media/?access_token=fake&fields=id"
	require.Equal(t, expectedURL, client.url)
}

// ----------------------------------------------------------------------------

func TestGetMediaGetFail(t *testing.T) {
	client := fakeHTTPClient{
		err: errors.New("fake"),
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedia("fakeID")
	require.EqualError(t, err, "failed to get 'https://graph.instagram.com/fakeID?access_token=fake&fields=id%2Ccaption%2Cmedia_type%2Cmedia_url%2Cpermalink%2Cusername%2Ctimestamp': fake")
}

func TestGetMediaBadStatus(t *testing.T) {
	client := fakeHTTPClient{
		statusCode: 500,
		body:       []byte("body"),
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedia("")
	require.EqualError(t, err, "http request failed with status 500: body")
}

func TestGetMediaFailUnmarshal(t *testing.T) {
	client := fakeHTTPClient{
		body:       []byte("invalid json"),
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	_, err := api.GetMedia("")
	require.EqualError(t, err, "failed to decode response: invalid character 'i' looking for beginning of value")
}

func TestGetMediaSuccess(t *testing.T) {
	media := types.Media{
		ID: "aa",
	}

	buff, err := json.Marshal(&media)
	require.NoError(t, err)

	client := fakeHTTPClient{
		body:       buff,
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	mediaResponse, err := api.GetMedia("fakeID")
	require.NoError(t, err)

	require.Equal(t, media, mediaResponse)

	expectedURL := "https://graph.instagram.com/fakeID?access_token=fake&fields=id%2Ccaption%2Cmedia_type%2Cmedia_url%2Cpermalink%2Cusername%2Ctimestamp"
	require.Equal(t, expectedURL, client.url)
}

// ----------------------------------------------------------------------------

func TestRefeshTokenGetFail(t *testing.T) {
	client := fakeHTTPClient{
		err: errors.New("fake"),
	}

	api := NewHTTPAPI("fake", &client)

	err := api.RefreshToken()
	require.EqualError(t, err, "failed to get 'https://graph.instagram.com/refresh_access_token?access_token=fake&grant_type=ig_refresh_token': fake")
}

func TestRefeshTokenBadStatus(t *testing.T) {
	client := fakeHTTPClient{
		statusCode: 500,
		body:       []byte("body"),
	}

	api := NewHTTPAPI("fake", &client)

	err := api.RefreshToken()
	require.EqualError(t, err, "http request failed with status 500: body")
}

func TestRefeshTokenFailUnmarshal(t *testing.T) {
	client := fakeHTTPClient{
		body:       []byte("invalid json"),
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	err := api.RefreshToken()
	require.EqualError(t, err, "failed to decode response: invalid character 'i' looking for beginning of value")
}

func TestRefeshTokenSuccess(t *testing.T) {
	refresh := types.RefreshResponse{
		AccessToken: "aa",
	}

	buff, err := json.Marshal(&refresh)
	require.NoError(t, err)

	client := fakeHTTPClient{
		body:       buff,
		statusCode: 200,
	}

	api := NewHTTPAPI("fake", &client)

	err = api.RefreshToken()
	require.NoError(t, err)

	require.Equal(t, refresh.AccessToken, api.(*HTTPAPI).token)

	expectedURL := "https://graph.instagram.com/refresh_access_token?access_token=fake&grant_type=ig_refresh_token"
	require.Equal(t, expectedURL, client.url)
}

// ----------------------------------------------------------------------------
// Utility functions

type fakeHTTPClient struct {
	HTTPClient
	err        error
	body       []byte
	statusCode int
	url        string
}

func (h *fakeHTTPClient) Get(url string) (resp *http.Response, err error) {
	if h.err != nil {
		return nil, h.err
	}

	h.url = url

	buff := bytes.NewBuffer(h.body)
	body := io.NopCloser(buff)

	return &http.Response{
		Body:       body,
		StatusCode: h.statusCode,
		Status:     strconv.Itoa(h.statusCode),
	}, h.err
}
