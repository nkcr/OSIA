package instagram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/nkcr/OSIA/instagram/types"
)

// InstagramAPI defines the primitives we expect the Instagram API to provide
type InstagramAPI interface {
	GetMedias() (types.Medias, error)
	GetMedia(id string) (types.Media, error)
	RefreshToken() error
}

// HTTPClient defines the function we expect from an HTTP client
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// NewHTTPAPI returns a new initialized Instagram HTTP API
func NewHTTPAPI(token string, client HTTPClient) InstagramAPI {
	return &HTTPAPI{
		base:   "https://graph.instagram.com/",
		token:  token,
		client: client,
	}
}

// HTTPAPI implements the Instagram API over HTTP
//
// - implements InstagramAPI
type HTTPAPI struct {
	base   string
	token  string
	client HTTPClient
}

// GetMedias implements InstagramAPI
func (h HTTPAPI) GetMedias() (types.Medias, error) {
	vals := url.Values{
		"access_token": []string{h.token},
		"fields":       []string{"id"},
	}

	u := h.base + "me/media/" + "?" + vals.Encode()

	resp, err := h.client.Get(u)
	if err != nil {
		return types.Medias{}, fmt.Errorf("failed to get '%s': %v", u, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return types.Medias{}, statusError(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	var medias types.Medias

	err = decoder.Decode(&medias)
	if err != nil {
		return types.Medias{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return medias, nil
}

// GetMedia implements InstagramAPI
func (h HTTPAPI) GetMedia(id string) (types.Media, error) {
	vals := url.Values{
		"access_token": []string{h.token},
		"fields":       []string{"id,caption,media_type,media_url,permalink,username,timestamp"},
	}

	u := h.base + id + "?" + vals.Encode()

	resp, err := h.client.Get(u)
	if err != nil {
		return types.Media{}, fmt.Errorf("failed to get '%s': %v", u, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return types.Media{}, statusError(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	var media types.Media

	err = decoder.Decode(&media)
	if err != nil {
		return types.Media{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return media, nil
}

// RefreshToken implements InstagramAPI
func (h *HTTPAPI) RefreshToken() error {
	vals := url.Values{
		"access_token": []string{h.token},
		"grant_type":   []string{"ig_refresh_token"},
	}

	u := h.base + "refresh_access_token?" + vals.Encode()

	resp, err := h.client.Get(u)
	if err != nil {
		return fmt.Errorf("failed to get '%s': %v", u, err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return statusError(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	var refresh types.RefreshResponse

	err = decoder.Decode(&refresh)
	if err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	h.token = refresh.AccessToken

	return nil
}

func statusError(resp *http.Response) error {
	buf, _ := ioutil.ReadAll(resp.Body)
	return fmt.Errorf("http request failed with status %s: %s", resp.Status, buf)
}
