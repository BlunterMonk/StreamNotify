package youtube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var (
	vRegex    *regexp.Regexp
	ytirRegex *regexp.Regexp
)

const (
	YT_API_KEY                 = "AIzaSyCIEY354CKB26BQVhGm-mBOmkgKyfakJT8"
	GOOGLE_API_SEARCH_ENDPOINT = "https://www.googleapis.com/youtube/v3/search"

	// a max request count of 99 is the absolute limit we can use on the youtube search API
	maxRequestCount = 99
)

func init() {

	vRegex = regexp.MustCompile(`<link\srel="canonical"\shref="https:\/\/www.youtube.com\/watch\?v\=([a-zA-Z0-9]+)">`)
	ytirRegex = regexp.MustCompile(`<script\s+nonce=".*">var\sytInitialPlayerResponse\s=\s(.*);<\/script>`)

}

func GetAllChannelStatus(channels map[string]string) map[string]VideoDetails {
	streamInfo := make(map[string]VideoDetails, 0)

	// record stream status
	for k, v := range channels {
		res, err := getChannelLiveStatus(v)
		if err != nil {
			log.Printf("failed to get live status for channel: %v - %v\n", v, k)
			continue
		}

		// log.Printf("Channel: %v, Live: %v, ID: %v, Title: %v\n", k, res.VideoDetails.IsLive, res.VideoDetails.VideoID, res.VideoDetails.Title)
		streamInfo[k] = *res
	}

	return streamInfo
}

/////////////// Playback

func getLiveStatus(channelID string) (bool, *searchItem, error) {

	queryChannel := fmt.Sprintf("%v?part=snippet&channelId=%v&type=video&eventType=live&key=%v", GOOGLE_API_SEARCH_ENDPOINT, channelID, YT_API_KEY)
	resp, err := http.Get(queryChannel)
	if err != nil {
		return false, nil, err
	}

	fmt.Println(resp.StatusCode)

	// read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, nil, err
	}

	// decode response
	var res YoutubeSearchOutput
	rb := bytes.NewReader(body)
	err = json.NewDecoder(rb).Decode(&res)
	if err != nil {
		return false, nil, err
	}

	fmt.Println(string(body))
	if len(res.Items) == 0 {
		return false, nil, nil
	}

	return res.Items[0].Snippet.LiveStatus == "live", &(res.Items[0]), nil
}

func getChannelLiveStatus(channelID string) (*VideoDetails, error) {

	query := "https://www.youtube.com/" + channelID + "/live"

	resp, err := http.Get(query)
	if err != nil {
		return nil, fmt.Errorf("error occurred getting youtube page: %v: %v\n", query, err)
	}

	// log.Printf("getChannelLiveStatus: %v, statusCode: %v\n", query, resp.StatusCode)

	// read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error occurred trying to read response body: %v: %v\n", query, err)
	}

	b := string(body)
	mr := ytirRegex.MatchString(b)
	// log.Printf("matched initial response: %v\n", mr)
	if !mr {
		return nil, fmt.Errorf("failed to get initial response, regex did not match")
	}

	values := ytirRegex.FindAllStringSubmatch(b, -1)
	if values == nil {
		return nil, fmt.Errorf("failed to get initial response")
	}

	var res VideoDetails
	rb := bytes.NewReader([]byte(values[0][1]))
	err = json.NewDecoder(rb).Decode(&res)
	if err != nil {
		log.Fatalf("failed to decode response: %v\n", err)
	}

	return &res, nil
}

func checkYoutubePage(channelID string) (bool, string, string) {
	var title string

	query := "https://www.youtube.com/channel/" + channelID + "/live"

	fmt.Println("Searching Page: " + query)

	resp, err := http.Get(query)
	if err != nil {
		log.Printf("error occurred getting youtube page: %v: %v\n", query, err)
		return false, "", ""
	}

	fmt.Println(resp.StatusCode)

	// read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error occurred trying to read response body: %v: %v\n", query, err)
		return false, "", ""
	}

	// saveOutput(fmt.Sprintf("./%v-page.html", channelID), body)

	b := string(body)
	mr := ytirRegex.MatchString(b)
	fmt.Printf("matched initial response: %v\n", mr)
	if mr {
		values := ytirRegex.FindAllStringSubmatch(b, -1)
		if values == nil {
			log.Println("failed to get initial response")
		}

		var res VideoDetails
		rb := bytes.NewReader([]byte(values[0][1]))
		err = json.NewDecoder(rb).Decode(&res)
		if err != nil {
			log.Fatalf("failed to decode response: %v\n", err)
		}

		title = res.VideoDetails.Title
		// saveOutput(fmt.Sprintf("./%v-response.json", channelID), []byte(values[0][1]))
	}

	if !strings.Contains(b, `<div id="player-wrap">`) || strings.Contains(b, `LIVE_STREAM_OFFLINE`) {
		fmt.Println("did not find video player")
		return false, "", title
	}

	matched := vRegex.MatchString(b)
	fmt.Printf("matched regex: %v\n", matched)
	if matched {
		log.Println("found stream url")
		values := vRegex.FindAllStringSubmatch(b, -1)
		if values == nil {
			log.Println("failed to get stream url")
		}

		return true, values[0][1], title
	}

	return false, "", title
}

func strcontains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
