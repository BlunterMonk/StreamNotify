package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BlunterMonk/StreamNotify/pkg/config"
	"github.com/BlunterMonk/StreamNotify/pkg/toast"
	yt "github.com/BlunterMonk/StreamNotify/pkg/youtube"
)

var (
	history []string //

	// state
	sleeping = false
)

const (
	XCODE_SHUTDOWN_SIGNAL = 0
	XCODE_CONSUMER_FAILED = 4
	XCODE_PANIC           = 5
	XCODE_ABORT           = 6

	YT_API_KEY                 = "AIzaSyCIEY354CKB26BQVhGm-mBOmkgKyfakJT8"
	GOOGLE_API_SEARCH_ENDPOINT = "https://www.googleapis.com/youtube/v3/search"

	// a max request count of 99 is the absolute limit we can use on the youtube search API
	maxRequestCount = 99

	// in minutes
	liveCheckTimer = 15
	ambienceTimer  = 1
)

func main() {
	var xCode int
	var streamInfo map[string]yt.VideoDetails
	var sleeping bool
	var vlcStatus VLCStatus

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)

	killswitch := make(chan int, 0)
	go func() {
		for {
			select {
			// Shut down when we are signaled
			case <-sigc:
				log.Println("received a shutdown signal!")
				killswitch <- 0
				return
			}
		}
	}()

	// Catch any panics
	defer func() {
		if r := recover(); r != nil {
			log.Println("app panicked! ", r)
			for {
				select {
				// Shut down when we are signaled
				case <-sigc:
					log.Println("received a shutdown signal!")
					os.Exit(XCODE_PANIC)
					return
				}
			}
		}
	}()

	log.Println("starting vlc service...")
	// TODO: detect disconnects or stuck state, restart vlc and continue

	// Connect to VLC's RC interface
	conn, err := startAndConnectVlcService()
	if err != nil {
		panic(err.Error())
	}
	defer conn.Close()

	// Read and process the response from VLC
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()

			// Parse the lines and assign to struct fields
			fmt.Println("===", line)
			// fmt.Println("===")
			if strings.Contains(line, "state") {
				vlcStatus.State = parseState(line)
				// fmt.Println("state", vlcStatus.State)
			}
			if strings.Contains(line, "input") {
				vlcStatus.CurrentVideo = parseInput(line)
				// fmt.Println("input", vlcStatus.CurrentVideo)
				vlcStatus.VideoId = parseVideoId(vlcStatus.CurrentVideo)
				// fmt.Println("video id", vlcStatus.VideoId)
			}
			if strings.Contains(line, "time") {
				vlcStatus.Time = parseFloat(line)
			}
			if strings.Contains(line, "length") {
				vlcStatus.Length = parseFloat(line)
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading response:", err)
		}
	}()

	st := time.NewTicker(time.Duration(2) * time.Second)
	lt := time.NewTicker(time.Duration(config.Config.LiveTimer) * time.Minute)
	at := time.NewTicker(time.Duration(config.Config.AmbienceTimer) * time.Minute)
	qt := time.NewTicker(time.Duration(config.Config.QuietTimer) * time.Minute)
	rt := time.NewTicker(time.Duration(12 * time.Hour))

	thumbDir := fmt.Sprintf("%v/thumb/", config.ConfigPath)
	mkdir(thumbDir)
	if nil != RemoveContents(thumbDir) {
		log.Fatal("Failed to clear cached thumbnails")
	}

	// load in channel status
	streamInfo = yt.GetAllChannelStatus(config.Config.Channels)
	for _, v := range streamInfo {
		on := v.VideoDetails.IsLive
		if !on {
			continue
		}

		notify(v)

		// if !started && config.Config.AutoPlay {
		// 	playYoutubeVideo(v.VideoDetails.VideoID, v)
		// 	playingAmbient = v.VideoDetails.VideoID
		// 	started = true
		// }
	}
	if quietTime(config.Config.QuietStartTime, config.Config.QuietEndTime) {
		if vlcStatus.State > 0 {
			log.Println("quiet hours, stopping all playback")
			stopPlayback(conn)
			sleeping = true
		}
	}

F: //loop
	for {
		select {
		case <-rt.C:
			// Clear cache and history every 12 hours so it doesn't get out of hand
			RemoveContents(thumbDir)
			break
		case <-st.C:
			// Send the "status" command to VLC
			// fmt.Println("updating vlc status")
			_, err := conn.Write([]byte("status\n"))
			if err != nil {
				fmt.Println("Error sending command:", err)
				// return vlcStatus, err
			}
		case <-lt.C:
			config.LoadConfig()
			streamInfo = yt.GetAllChannelStatus(config.Config.Channels)

			// go down the list of streamers to track, play the highest priority that is streaming
			for _, v := range streamInfo {

				on := v.VideoDetails.IsLive
				if !on {
					continue
				}

				notify(v)
			}
			break
		case <-qt.C:
			// halt all playback during quiet hours
			if quietTime(config.Config.QuietStartTime, config.Config.QuietEndTime) {
				if vlcStatus.State > 0 {
					log.Println("quiet hours, stopping all playback")
					stopPlayback(conn)
					sleeping = true
				}
			}
			break

		case <-at.C:
			// var np string
			priority := strings.Split(config.Config.Priority, ",")
			// on, id := videoIsPlaying(conn)
			on := vlcStatus.State > 0 && vlcStatus.State != 5
			// id := vlcStatus.CurrentVideo
			np := vlcStatus.CurrentVideo

			// if a video is playing, find out what video
			if on {
				// np = getPlayingVideoTitle(id)
				log.Printf("Now Playing: %v\n", np)

				// reset sleep timer if a new video starts playing after quiet hours were triggered
				// this probably means a video was played manually when staying up later than normal
				if sleeping {
					sleeping = false
					continue F
				}
			}

			// Do not attempt to play anything during quiet hours
			if quietTime(config.Config.QuietStartTime, config.Config.QuietEndTime) {
				continue
			}

			log.Println("attempting to play priority live stream")

			// go down the list of streamers to track, play the highest priority that is streaming
			for i := 0; i < len(priority); i++ {
				// cid := Config.Channels[priority[i]]
				// on, vid, title := checkYoutubePage(cid)
				// fmt.Printf("Channel: %v, live: %v\n", priority[i], on)
				name := strings.Trim(priority[i], " ")
				on := streamInfo[name].VideoDetails.IsLive
				// title := streamInfo[name].VideoDetails.Title
				vid := streamInfo[name].VideoDetails.VideoID
				if !on {
					continue
				}

				// don't try to play the same video
				if strings.Contains(vlcStatus.VideoId, vid) {
					log.Println("video already playing:", vid)
					continue F
				}

				playYoutubeVideo(conn, vid, streamInfo[name])
				continue F
			}

			log.Println("no priority streams available, attempting to play a low priority stream")

			for _, v := range streamInfo {
				if strings.Contains(vlcStatus.VideoId, v.VideoDetails.VideoID) {
					on = v.VideoDetails.IsLive
					break
				}
			}

			// only continue if what's playing is ambient music or nothing
			// this way we don't overwrite manually chosen videos
			if !on || strings.HasPrefix(np, "file:") {
				// if no one on the priority list is streaming
				// just play the first live channel found
				// by randomizing the order of low priority channels registered
				vid := selectRandomLiveStream(streamInfo)
				if config.Config.RandomizeStreams && vid.VideoDetails.VideoID != "" {
					playYoutubeVideo(conn, vid.VideoDetails.VideoID, vid)
					continue F
				}

				log.Println("no live streams, playing ambient music")

				// If no streams were found just play some BGM
				if !on {
					var err error
					_, err = playAmbienceMV(conn, config.Config.MusicDir)
					if err != nil {
						log.Println(err.Error())
					}
				}
			} else {
				log.Println("nothing to play.")
			}
			break
		case xCode = <-killswitch:
			log.Println("app killswitch")
			break F
		}
	}

	os.Exit(xCode)
}

func quietTime(s, e string) bool {
	now := time.Now()
	// fmt.Println(now.Round(0))
	yyyy, mm, dd := now.Date()
	now = time.Date(yyyy, mm, dd, now.Hour(), now.Minute(), now.Second(), 0, time.UTC)

	// fmt.Println(fmt.Sprintf("%d-%d-%dT%s:00", yyyy, mm, dd, s))
	qs, err := time.ParseInLocation("2006-01-02T15:04:05", fmt.Sprintf("%d-%02d-%02dT%s:00", yyyy, mm, dd, s), time.UTC)
	if err != nil {
		fmt.Println(err.Error())
	}
	// qs = qs.Local()
	qe, err := time.ParseInLocation("2006-01-02T15:04:05", fmt.Sprintf("%d-%02d-%02dT%s:00", yyyy, mm, dd, e), time.UTC)
	if err != nil {
		fmt.Println(err.Error())
	}
	// qe = qe.Local()
	fmt.Println("checking quiet time", qs, "<", now, "<", qe)
	if qs.Before(qe) {
		return qs.Before(now) && qe.After(now)
	}

	fmt.Println("checking quiet time", qe, "<", now, "<", qs)
	return qe.Before(now) && qs.After(now)
}

func notify(videoData yt.VideoDetails) {
	videoID := videoData.VideoDetails.VideoID

	if strcontains(history, videoID) {
		// log.Println("already notified: ", videoID)
		return
	}

	// config.ConfigPath = "E:/User/src/go/src/github.com/BlunterMonk/StreamNotify"
	fn := fixPath(fmt.Sprintf("%v/thumb/%v.jpg", config.ConfigPath, videoID))
	log.Println("thumbnail: ", fn)
	if _, err := os.Stat(fn); errors.Is(err, os.ErrNotExist) {
		// path/to/whatever does not exist
		thmb := videoData.GetThumbnail()
		log.Println("downloading thumbnail: ", thmb)

		out, err := os.Create(fn)
		if err != nil {
			log.Println(err, "couldn't display notif")
			return
		}
		defer out.Close()

		resp, err := http.Get(thmb)
		if err != nil {
			log.Println(err, "couldn't display notif")
			return
		}
		defer resp.Body.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Println(err, "couldn't display notif")
			return
		}
	}

	url := fmt.Sprintf("https://www.youtube.com/watch?v=%v", videoID)
	log.Println("notification: ", url)
	err := toast.Push("test message",
		toast.WithTitle(videoData.VideoDetails.Title),
		toast.WithAppID("Streamer"),
		toast.WithIcon(fn),
		toast.WithAudio(toast.Default),
		toast.WithLongDuration(),
		toast.WithProtocolAction("Watch", url),
		toast.WithProtocolAction("Dismiss", "dismiss"),
	)
	if err != nil {
		log.Println(err)
	}

	history = append(history, videoID)
}

func selectRandomLiveStream(streamInfo map[string]yt.VideoDetails) yt.VideoDetails {

	rand.Seed(time.Now().UnixNano())

	count := len(streamInfo)
	indices := make([]int, 0)
	keys := make([]string, 0, len(streamInfo))
	for k := range streamInfo {
		keys = append(keys, k)
		// fmt.Println(k)
	}

	// randomize indices
	for len(indices) < count {
		ind := rand.Intn(count)
		if contains(indices, ind) {
			continue
		}
		indices = append(indices, ind)
	}

	for i := 0; i < count; i++ {
		v := streamInfo[keys[indices[i]]]
		if !v.VideoDetails.IsLive {
			continue
		}

		// fmt.Println("returning live video:", v)
		return v
	}

	return yt.VideoDetails{}
}

/////////////////////////////////////////////////////////////
// Helper

func strcontains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func fixPath(dir string) string {
	return filepath.ToSlash(path.Clean(dir))
}

func fileMustExist(path string) {
	if _, err := os.Stat(path); os.IsExist(err) {
		panic(fmt.Errorf("file does not exist, aborting script: %v", path))
	}
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

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

func saveStatus(streamInfo map[string]yt.VideoDetails) error {
	// create output file
	body, err := json.Marshal(streamInfo)
	if err != nil {
		return err
	}

	statusFilename := fmt.Sprintf("%v/.livestatus.json", config.ConfigPath)

	out, err := os.OpenFile(statusFilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
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

func videoIsPlaying(conn net.Conn) (bool, int) {
	fmt.Println("Video is Playing")
	status, err := getVlcStatus(conn)
	if err != nil {
		return false, 0
	}
	fmt.Println("status:", status)
	// TODO: determine if a video is already active
	return status.State > 0, 0
}

func stopPlayback(conn net.Conn) {
	fmt.Println("Stop Playback")

	// Stop the current video and start the new one
	_, err := conn.Write([]byte("clear\n"))
	if err != nil {
		log.Fatalf("Error clearing playlist: %v", err)
	}

}

func getPlayingVideoTitle(id int) string {
	fmt.Println("Get Video Title:", id)
	return ""
}

func playYoutubeVideo(conn net.Conn, videoID string, details yt.VideoDetails) {
	fmt.Println("Play Youtube Video:", videoID)

	// The YouTube URL you want to stream
	youtubeURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID) // Example YouTube URL
	playVideoOnVlcHttp(conn, youtubeURL)

	fmt.Println("VLC is streaming the video.")
}

func playVideoWithVlc(filepath string) error {

	// VLC command with the YouTube URL
	// cmd := exec.Command("vlc", "--one-instance", filepath)
	cmd := exec.Command("vlc", "-I", "rc", "--rc-host=localhost:4212", "--one-instance", filepath)

	// Run the command
	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting VLC:", err)
		return err
	}

	return nil
}

func playAmbienceMV(conn net.Conn, dir string) (string, error) {

	videoPath, err := loadRandomLocalVideo(dir)
	if err != nil {
		return "", err
	}

	fmt.Println("Playing Ambience MV:", videoPath)
	playVideoOnVlcHttp(conn, videoPath)
	return videoPath, nil
}

func loadRandomLocalVideo(dir string) (string, error) {
	var videoPath string

	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	n := rand.Intn(len(files))
	if n >= len(files) {
		n = len(files) - 1
	}

	videoPath = strings.ReplaceAll(fmt.Sprintf("%s/%s", dir, files[n].Name()), "/", "\\")
	fmt.Println("random video chosen:", n, videoPath)
	return videoPath, nil
}

func startAndConnectVlcService() (net.Conn, error) {
	var conn net.Conn
	var err error

	conn, err = net.Dial("tcp", "localhost:4212")
	if err != nil {
		fmt.Println("vlc not running, trying to connect")

		for i := 0; i < 5; i++ {
			err = startVlcService()
			if err != nil {
				fmt.Println(err.Error())
				continue
			}
			time.Sleep(time.Second)
			conn, err = net.Dial("tcp", "localhost:4212")
			if err != nil {
				log.Println("failed to start vlc service:", err.Error())
				continue
			}
			return conn, nil
		}
		fmt.Println("Error connecting to VLC:", err)
		return nil, err
	}

	return conn, nil
}

func startVlcService() error {
	// VLC command with the YouTube URL
	cmd := exec.Command("vlc", "-I", "rc", "--rc-host=localhost:4212", "--one-instance", "--fullscreen")

	// Run the command
	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting VLC:", err)
		return err
	}

	return nil
}

// Define a struct to represent the VLC playback status
type VLCStatus struct {
	State        int     `json:"state"`
	CurrentVideo string  `json:"current_video"`
	Time         float64 `json:"time"`
	Length       float64 `json:"length"`
	VideoId      string  `json:"video_id"`
}

func getVlcStatus(conn net.Conn) (VLCStatus, error) {
	// VLC command with the YouTube URL
	// cmd := exec.Command("vlc", "-I", "rc", "--rc-host=localhost:4212", "--one-instance", filepath)

	// Run the command
	// err := cmd.Start()
	// if err != nil {
	// 	fmt.Println("Error starting VLC:", err)
	// 	return err
	// }
	// Initialize the struct
	var vlcStatus VLCStatus

	// Connect to VLC's RC interface
	// conn, err := net.Dial("tcp", "localhost:4212")
	// if err != nil {
	// 	fmt.Println("Error connecting to VLC:", err)
	// 	return vlcStatus, err
	// }
	// defer conn.Close()

	// // Set a timeout for the connection (optional)
	// conn.SetDeadline(time.Now().Add(5 * time.Second))

	fmt.Println("current status:")
	fmt.Println(vlcStatus)
	fmt.Println("===============")

	return vlcStatus, nil
}

// Parse state (e.g., "state: playing")
var parseStateRegexp = regexp.MustCompile("play state: ([0-9])")

func parseState(line string) int {
	matches := parseStateRegexp.FindStringSubmatch(line)
	if len(matches) < 2 {
		fmt.Println("parse state regex failed")
		return 0
	}
	n, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		fmt.Println("parse state as int failed:", err.Error())
		return 0
	}
	return int(n)
}

// Parse current video file (e.g., "input: file:///C:/path/to/video.mp4")
var parseInputRegexp = regexp.MustCompile("new input: (.*)\\s")
var parseVideoIdRegexp = regexp.MustCompile("/id/(.*)/source")

func parseInput(line string) string {
	matches := parseInputRegexp.FindStringSubmatch(line)
	if len(matches) < 2 {
		fmt.Println("parse state regex failed")
		return ""
	}
	return matches[1]
}

func parseVideoId(line string) string {
	matches := parseVideoIdRegexp.FindStringSubmatch(line)
	// fmt.Println(matches)
	if len(matches) < 2 {
		fmt.Println("parse video id regex failed")
		return ""
	}
	return matches[1]
}

// Parse time (e.g., "time: 120.8")
func parseFloat(line string) float64 {
	parts := strings.Split(line, ": ")
	if len(parts) > 1 {
		var time float64
		fmt.Sscanf(parts[1], "%f", &time)
		return time
	}
	return 0
}

func playVideoOnVlcHttp(conn net.Conn, videoPath string) {
	// Connect to VLC's RC interface at port 4212
	// conn, err := net.Dial("tcp", "localhost:4212")
	// if err != nil {
	// 	log.Fatalf("Error connecting to VLC RC interface: %v", err)
	// }
	// defer conn.Close()

	// Stop the current video and start the new one
	_, err := conn.Write([]byte("clear\n"))
	if err != nil {
		log.Fatalf("Error clearing playlist: %v", err)
	}

	// Send the 'add' command to start the video playback
	_, err = conn.Write([]byte(fmt.Sprintf("add %s\n", videoPath)))
	if err != nil {
		log.Fatalf("Error sending command: %v", err)
	}

	// Optionally, wait and monitor playback status or send further commands
	fmt.Println("Playback command sent to VLC. The video should start playing.")
}
