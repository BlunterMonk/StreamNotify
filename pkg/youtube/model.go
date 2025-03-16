package youtube

type VideoDetails struct {
	PlayabilityStatus map[string]interface{} `json:"playabilityStatus"`
	VideoDetails      videoDetails           `json:"videoDetails"`
}
type videoDetails struct {
	Author    string          `json:"author"`
	VideoID   string          `json:"videoId"`
	Title     string          `json:"title"`
	IsLive    bool            `json:"isLive"`
	Thumbnail thumbnailObject `json:"thumbnail"`
}
type thumbnailObject struct {
	Thumbnails []thumbnailDetails `json:"thumbnails"`
}
type thumbnailDetails struct {
	Url    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type YoutubeSearchOutput struct {
	Kind       string       `json:"kind"`
	ETag       string       `json:"etag"`
	RegionCode string       `json:"regionCode"`
	Items      []searchItem `json:"items"`
}
type searchItem struct {
	Kind    string        `json:"kind"`
	ETag    string        `json:"etag"`
	VideoID videoId       `json:"id"`
	Snippet searchSnippet `json:"snippet"`
}
type videoId struct {
	Kind string `json:"kind"`
	ID   string `json:"videoId"`
}
type searchSnippet struct {
	ChannelID    string `json:"channelId"`
	ChannelTitle string `json:"channelTitle"`
	LiveStatus   string `json:"liveBroadcastContent"`
	PublishTime  string `json:"publishTime"`
}

func (d *VideoDetails) GetThumbnail() string {
	for _, v := range d.VideoDetails.Thumbnail.Thumbnails {
		if v.Width == 1920 {
			return v.Url
		}
	}

	return d.VideoDetails.Thumbnail.Thumbnails[0].Url
}
