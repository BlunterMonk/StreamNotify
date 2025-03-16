package kodi

type KodiResponse interface {
	KodiOutput | GetActivePlayersOutput | PlayerGetItemOutput
}
type KodiOutput struct {
	ID      int         `json:"id"`
	Version string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}
type FileDescriptor struct {
	Filename string `json:"file"`
	Filetype string `json:"filetype"`
	Label    string `json:"label"`
	Type     string `json:"type"`
}
type ActivePlayerDescriptor struct {
	PlayerID   int    `json:"playerid"`
	PlayerType string `json:"playertype"`
	Type       string `json:"type"`
}
type GetFileResult struct {
	Files []FileDescriptor `json:"files"`
}
type FilesGetDirectoryOutput struct {
	KodiOutput
	Result GetFileResult `json:"result"`
}
type GetActivePlayersOutput struct {
	KodiOutput
	Result []ActivePlayerDescriptor `json:"result"`
}
type PlayerGetItemOutput struct {
	KodiOutput
	Result PlayerGetItemResult `json:"result"`
}
type PlayerGetItemResult struct {
	Item PlayerItem `json:"item"`
}
type PlayerItem struct {
	Label string `json:"label"`
	Type  string `json:"type"`
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
