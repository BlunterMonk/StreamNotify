package kodi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/BlunterMonk/StreamNotify/pkg/config"
)

const (
	GET_FILES_REQUEST_BODY         = `{"jsonrpc":"2.0", "id":1, "method":"Files.GetDirectory", "params": { "directory": "%v", "media": "video" }}`
	GET_PLAYLIST_REQUEST_BODY      = `{"jsonrpc":"2.0", "id":1, "method":"Playlist.GetItems", "params":{"playlistid":1}}`
	GET_ACTIVE_PLAYER_REQUEST_BODY = `{"jsonrpc":"2.0", "id":1, "method":"Player.GetActivePlayers"}`
	GET_ITEM_REQUEST_BODY          = `{"jsonrpc":"2.0", "id":1, "method":"Player.GetItem", "params":{"playerid":%v}}`
	ADD_TO_QUEUE_REQUEST_BODY      = `{"jsonrpc":"2.0", "id":1, "method":"Playlist.Add", "params":{"playlistid":1, "item":{"file":"%v"}}}`

	SET_VOLUME_REQUEST_BODY = `{"jsonrpc":"2.0", "id":1, "method":"Application.SetVolume", "params":{"volume":%v}}`

	PLAY_FILE_REQUEST_BODY  = `{"jsonrpc":"2.0", "method":"Player.Open", "id":1, "params":{"item":{"file":"%v"}}}`
	STOP_VIDEO_REQUEST_BODY = `{"jsonrpc":"2.0", "method":"Player.Stop", "id":1, "params":{"playerid":1}}`
)

func Monitor() {

}

/////////////// Cache

func cacheAll(dir string) error {
	list, err := getAllFiles(dir)
	if err != nil {
		return err
	}

	f, err := os.OpenFile("./all.txt", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, v := range list {
		f.WriteString(v + "\n")
	}

	return nil
}

func getAllFiles(path string) ([]string, error) {
	list := make([]string, 0)
	res, err := getFiles(path)
	if err != nil {
		return nil, err
	}
	for k, v := range res.Result.Files {
		if v.Filetype == "file" {
			fmt.Printf("%v - %v\n", k, v.Filename)
			list = append(list, v.Filename)
		} else if v.Filetype == "directory" {
			subfiles, err := getAllFiles(v.Filename)
			if err != nil {
				return nil, err
			}
			list = append(list, subfiles...)
		}
	}

	return list, nil
}

func getFiles(path string) (FilesGetDirectoryOutput, error) {

	// load cached data if it exists
	outFile := fmt.Sprintf("./data/%v.json", strings.TrimSuffix(strings.TrimPrefix(path, config.Config.StorageDir), "/"))

	f, err := os.OpenFile(outFile, os.O_RDONLY|os.O_EXCL, 0666)
	if err == nil || errors.Is(err, os.ErrExist) {
		fmt.Printf("Returning Cached: %v\n", outFile)

		body, err := ioutil.ReadAll(f)
		if err != nil {
			return FilesGetDirectoryOutput{}, err
		}

		// decode response
		var res FilesGetDirectoryOutput
		rb := bytes.NewReader(body)
		err = json.NewDecoder(rb).Decode(&res)
		if err != nil {
			return FilesGetDirectoryOutput{}, err
		}

		return res, nil
	}

	q := fmt.Sprintf(GET_FILES_REQUEST_BODY, path)

	resp, err := http.Post(requestEndpoint(), "application/json", bytes.NewBuffer([]byte(q)))
	if err != nil {
		return FilesGetDirectoryOutput{}, err
	}

	fmt.Println(resp.StatusCode)

	// read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return FilesGetDirectoryOutput{}, err
	}

	// save response to file
	mkdir(outFile)
	saveOutput(outFile, body)

	// decode response
	var res FilesGetDirectoryOutput
	rb := bytes.NewReader(body)
	err = json.NewDecoder(rb).Decode(&res)
	if err != nil {
		return FilesGetDirectoryOutput{}, err
	}

	return res, nil
}

/////////////// Playback

func playAmbienceMV() (string, error) {
	log.Println("Attempting to play ambient music")

	res, err := getFiles(config.Config.MusicDir)
	if err != nil {
		return "", err
	}
	count := len(res.Result.Files)
	rand.Seed(time.Now().UnixNano())
	ind := rand.Intn(count)
	fn := res.Result.Files[ind].Filename
	playFile(fn)

	if on, _ := videoIsPlaying(); on {
		log.Println("now playing: " + fn)
		request[KodiOutput](fmt.Sprintf(SET_VOLUME_REQUEST_BODY, 50))
	}

	return fn, nil
}

func playYoutubeVideo(videoID string) {
	pluginUrl := `plugin://plugin.video.youtube/?action=play_video&videoid=` + videoID
	request[KodiOutput](fmt.Sprintf(`{
		"jsonrpc":"2.0",
		"method":"Player.Open",
		"id":1,
		"params":{
			"item":{
				"file":"%v"
			}
		}
	}`, pluginUrl))
}

func playFile(path string) {

	q := fmt.Sprintf(PLAY_FILE_REQUEST_BODY, path)

	request[KodiOutput](q)
}

func request[v KodiResponse](query string) (*v, error) {
	fmt.Println(query)

	resp, err := http.Post(requestEndpoint(), "application/json", bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, err
	}

	fmt.Println(resp.StatusCode)

	// read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(body))

	var res v
	rb := bytes.NewReader(body)
	err = json.NewDecoder(rb).Decode(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func getPlayingVideoTitle(id int) string {

	res, err := request[PlayerGetItemOutput](fmt.Sprintf(GET_ITEM_REQUEST_BODY, id))
	if err != nil {
		log.Printf("failed to get video title: %v\n", err)
		return ""
	}

	return res.Result.Item.Label
}

func videoIsPlaying() (bool, int) {

	active, err := request[GetActivePlayersOutput](GET_ACTIVE_PLAYER_REQUEST_BODY)
	if err != nil {
		fmt.Printf("GET_ACTIVE_PLAYER returned an error: %v\n", err)
		return false, -1
	}
	if len(active.Result) == 0 {
		return false, -1
	}
	if active.Result[0].Type != "video" {
		return false, -1
	}

	return true, active.Result[0].PlayerID
}

func requestEndpoint() string {
	return fmt.Sprintf("%v:%v/jsonrpc", config.Config.Host, config.Config.Port)
}

func stopPlayback() {
	request[KodiOutput](STOP_VIDEO_REQUEST_BODY)
}

/////////////// IO

func mkdir(filepath string) error {
	filepath = path.Dir(filepath)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		fmt.Printf("generating directory: %v\n", filepath)
		if err := os.Mkdir(filepath, os.ModePerm); err != nil {
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func loadOutput[v KodiResponse](outputFile string) (*v, error) {

	f, err := os.OpenFile(outputFile, os.O_RDONLY|os.O_EXCL, 0666)
	if err == nil || errors.Is(err, os.ErrExist) {
		fmt.Printf("Loading Cache: %v\n", outputFile)

		body, err := ioutil.ReadAll(f)
		if err != nil {
			return nil, err
		}

		// decode response
		var res v
		rb := bytes.NewReader(body)
		err = json.NewDecoder(rb).Decode(&res)
		if err != nil {
			return nil, err
		}

		return &res, nil
	}

	return nil, errors.New(err.Error() + ":failed to load cached file: " + outputFile)
}

func saveOutput(outputFile string, body []byte) error {

	// create output file
	out, err := os.OpenFile(outputFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write(body)
	if err != nil {
		return err
	}

	return nil
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
