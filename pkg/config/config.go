package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	Config        *config
	ConfigPath    string
	defaultConfig = config{
		LiveTimer:        1,
		AmbienceTimer:    1,
		QuietTimer:       2,
		QuietStartTime:   "03:00",
		QuietEndTime:     "08:00",
		AutoPlay:         true,
		RandomizeStreams: true,
		AutoPlayApp:      "web",
		MusicDir:         "E:/User/Videos/bgm",
		Priority:         "elira,doki,mint,eva",
		Channels: map[string]string{
			"eva":   "@EvaAnanova",
			"doki":  "@dokibird",
			"elira": "@EliraPendora",
			"aia":   "@AiaAmare",
			"mori":  "@MoriCalliope",
			"mint":  "@MintFantome",
			"rana":  "@RanaIanna",
			"irys":  "@IRyS",
		},
	}
)

type config struct {
	Host             string            `json:"host"`             // kodi host IP
	Port             string            `json:"port"`             // kodi host port
	MusicDir         string            `json:"musicDir"`         // full path to folder containing music that should be randomized
	StorageDir       string            `json:"storageDir"`       // full path to storage directory
	AmbienceTimer    int               `json:"ambienceTimer"`    // time in minutes when kodi should attempt to play ambient music
	LiveTimer        int               `json:"liveTimer"`        // time in minutes when checking for livestreams
	QuietTimer       int               `json:"quietTimer"`       // time in minutes when kodi should stop all playback and sleep
	QuietStartTime   string            `json:"quietStartTime"`   // [0-23] the hour when quiet time starts
	QuietEndTime     string            `json:"quietEndTime"`     // [0-23] the hour when quiet time ends
	RandomizeStreams bool              `json:"randomizeStreams"` // if true, a random registered streamer will play if no other priority streamer is playing
	AutoPlay         bool              `json:"autoPlay"`         // automatically open videos
	AutoPlayApp      string            `json:"autoPlayApp"`      // application to open videos in ("vlc", "web")
	Priority         string            `json:"priority"`         // a priority queue for live channels stored as list separated by commas
	Channels         map[string]string `json:"channels"`         // list of channel IDs, play priority based on list order
}

func init() {
	LoadConfig()
}

func getConfigPath() (string, error) {

	// UserConfigDir returns the default root directory to use for user-specific configuration data. Users should create their own application-specific subdirectory within this one and use that.
	// On Unix systems, it returns $XDG_CONFIG_HOME as specified by https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html if non-empty, else $HOME/.config. On Darwin, it returns $HOME/Library/Application Support. On Windows, it returns %AppData%. On Plan 9, it returns $home/lib.
	// If the location cannot be determined (for example, $HOME is not defined), then it will return an error.
	// C:\Users\YourUser\AppData\Roaming
	c, err := os.Executable()
	// c, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("%v: missing config path: %v/StreamNofity/\n", err, c)
		return "", err
	}
	// log.Println("config path:", strings.Replace(c, "StreamNotify.exe", "config.json", 1))
	c = filepath.ToSlash(strings.Replace(c, "\\StreamNotify.exe", "", 1))
	// log.Printf("loading config: %s/config.json\n", c)
	return fmt.Sprintf("%v", c), nil
}

func LoadConfig() {

	path, err := getConfigPath()
	if err != nil {
		Config = &defaultConfig
		ConfigPath = "./"
		return
	}

	ConfigPath = path
	cfg, err := loadConfigFile(path)
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println("loading default config")

		Config = &defaultConfig

		j, err := json.Marshal(Config)
		if err != nil {
			fmt.Println("failed to save default config to file: ", err.Error())
			return
		}

		err = SaveFile(configFilename(path), j)
		if err != nil {
			fmt.Println("failed to save default config to file: ", err.Error())
			return
		}
		return
	}

	Config = cfg
}

func configFilename(configPath string) string {
	return filepath.ToSlash(path.Clean(fmt.Sprintf("%v/config.json", configPath)))
}

func loadConfigFile(configPath string) (*config, error) {

	fn := configFilename(configPath)
	f, err := os.OpenFile(fn, os.O_RDONLY|os.O_EXCL, 0666)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// decode response
	var res config
	rb := bytes.NewReader(body)
	err = json.NewDecoder(rb).Decode(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func SaveFile(filename string, data []byte) error {
	// remove any old output files to prevent corrupted results
	if _, err := os.Stat(filename); os.IsExist(err) {
		e := os.Remove(filename)
		if e != nil {
			return e
		}
	}

	// Ensure the directories exist before writing the file
	err := os.MkdirAll(filepath.Dir(filename), 0755)
	if err != nil {
		panic(err)
	}

	// create output file
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write(data)
	if err != nil {
		return err
	}

	return nil
}
