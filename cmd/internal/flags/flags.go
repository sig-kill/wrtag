package flags

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.senan.xyz/flagconf"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/lyrics"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

func EnvPrefix(prefix string) {
	flagconf.ReadEnvPrefix = func(_ *flag.FlagSet) string {
		return prefix
	}
}

func Parse() {
	userConfig, _ := os.UserConfigDir()
	defaultConfigPath := filepath.Join(userConfig, "wrtag", "config")
	configPath := flag.String("config-path", defaultConfigPath, "path config file")

	printVersion := flag.Bool("version", false, "print the version")
	printConfig := flag.Bool("config", false, "print the parsed config")

	flag.TextVar(&logLevel, "log-level", &logLevel, "set the logging level")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	if *printVersion {
		fmt.Printf("%s %s\n", flag.CommandLine.Name(), wrtag.Version)
		os.Exit(0)
	}
	if *printConfig {
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Printf("%-16s %s\n", f.Name, f.Value)
		})
		os.Exit(0)
	}
}

func PathFormat() *pathformat.Format {
	var r pathformat.Format
	flag.Var(&pathFormatParser{&r}, "path-format", "music directory and go templated path format to define music library layout")
	return &r
}

func Querier() *researchlink.Querier {
	var r researchlink.Querier
	flag.Var(&querierParser{&r}, "research-link", "define a helper url to help find information about an unmatched release")
	return &r
}

func KeepFiles() map[string]struct{} {
	var r = map[string]struct{}{}
	flag.Var(&keepFileParser{r}, "keep-file", "files to keep from source directories")
	return r
}

func TagWeights() tagmap.TagWeights {
	r := tagmap.TagWeights{}
	flag.Var(&tagWeightsParser{r}, "tag-weight", "adjust distance weighting for a tag between. 0 to ignore")
	return r
}

func Notifications() *notifications.Notifications {
	var r notifications.Notifications
	flag.Var(&notificationsParser{&r}, "notification-uri", "add a shoutrrr notification uri for an event")
	return &r
}

type MusicBrainzClient struct {
	*musicbrainz.MBClient
	*musicbrainz.CAAClient
}

func MusicBrainz() MusicBrainzClient {
	var mb musicbrainz.MBClient
	mb.HTTPClient = http.DefaultClient
	flag.StringVar(&mb.BaseURL, "mb-base-url", `https://musicbrainz.org/ws/2/`, "musicbrainz base url")
	flag.DurationVar(&mb.RateLimit, "mb-rate-limit", 1*time.Second, "musicbrainz rate limit duration")

	var caa musicbrainz.CAAClient
	caa.HTTPClient = http.DefaultClient
	flag.StringVar(&caa.BaseURL, "caa-base-url", `https://coverartarchive.org/`, "coverartarchive base url")
	flag.DurationVar(&caa.RateLimit, "caa-rate-limit", 0, "coverartarchive rate limit duration")

	return MusicBrainzClient{&mb, &caa}
}

func Lyrics() lyrics.Source {
	var musixmatch lyrics.Musixmatch
	musixmatch.HTTPClient = http.DefaultClient
	musixmatch.RateLimit = 500 * time.Millisecond

	var genius lyrics.Genius
	genius.HTTPClient = http.DefaultClient
	genius.RateLimit = 500 * time.Millisecond

	return lyrics.ChainSource{&genius, &musixmatch}
}

var httpClient *http.Client

func init() {
	httpClient = &http.Client{Transport: clientutil.Chain(
		clientutil.WithLogging(slog.Default()),
		clientutil.WithUserAgent(fmt.Sprintf(`wrtag/%s`, wrtag.Version)),
	)(http.DefaultTransport)}

	http.DefaultClient = httpClient
}
