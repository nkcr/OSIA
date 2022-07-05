package types

// Medias defines a list of Instagram media
type Medias struct {
	Data   []Media `json:"data"`
	Paging struct {
		Cursors struct {
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"cursors"`
	} `json:"paging"`
}

// Media defines an Instagram media
type Media struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	MediaType string `json:"media_type"`
	MediaURL  string `json:"media_url"`
	Permalink string `json:"permalink"`
	Username  string `json:"username"`
	Timestamp string `json:"timestamp"`
}
