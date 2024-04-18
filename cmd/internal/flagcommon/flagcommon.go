package flagcommon

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

const name = "wrtag"

func init() {
	flag.CommandLine.Init(name, flag.ExitOnError)
}

var (
	userConfig, _     = os.UserConfigDir()
	DefaultConfigPath = filepath.Join(userConfig, name, "config")
)

func PathFormat() *pathformat.Format {
	var r pathformat.Format
	flag.Var(pathFormatParser{&r}, "path-format", "music directory and go templated path format to define music library layout")
	return &r
}

func Querier() *researchlink.Querier {
	var r researchlink.Querier
	flag.Var(querierParser{&r}, "research-link", "define a helper url to help find information about an unmatched release")
	return &r
}

func KeepFiles() map[string]struct{} {
	var r = map[string]struct{}{}
	flag.Func("keep-file", "files to keep from source directories",
		func(s string) error { r[s] = struct{}{}; return nil })
	return r
}

func TagWeights() tagmap.TagWeights {
	var r tagmap.TagWeights
	flag.Var(tagWeightsParser{&r}, "tag-weight", "adjust distance weighting for a tag between. 0 to ignore")
	return r
}

func Notifications() *notifications.Notifications {
	var r notifications.Notifications
	flag.Var(notificationsParser{&r}, "notification-uri", "add a shoutrrr notification uri for an event")
	return &r
}

type MusicBrainzClient struct {
	*musicbrainz.MBClient
	*musicbrainz.CAAClient
}

func MusicBrainz() MusicBrainzClient {
	const defaultUserAgent = `wrtag/v0.0.0-alpha ( https://go.senan.xyz/wrtag )`

	var mb musicbrainz.MBClient
	flag.StringVar(&mb.UserAgent, "mb-user-agent", defaultUserAgent, "")
	flag.StringVar(&mb.BaseURL, "mb-base-url", `https://musicbrainz.org/ws/2/`, "")
	flag.DurationVar(&mb.RateLimit, "mb-rate-limit", 1*time.Second, "")

	var caa musicbrainz.CAAClient
	flag.StringVar(&caa.UserAgent, "caa-user-agent", defaultUserAgent, "")
	flag.StringVar(&caa.BaseURL, "caa-base-url", `https://coverartarchive.org/`, "")
	flag.DurationVar(&caa.RateLimit, "caa-rate-limit", 0, "")

	return MusicBrainzClient{&mb, &caa}
}

func ConfigPath() *string {
	return flag.String("config-path", DefaultConfigPath, "path config file")
}
