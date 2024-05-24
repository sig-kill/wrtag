package flags

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.senan.xyz/flagconf"
	"go.senan.xyz/wrtag"
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

func Config(cfg *wrtag.Config) {
	flag.Var(&pathFormatParser{&cfg.PathFormat}, "path-format", "music directory and go templated path format to define music library layout")
	flag.Var(&querierParser{&cfg.ResearchLinkQuerier}, "research-link", "define a helper url to help find information about an unmatched release")
	flag.Var(&keepFileParser{cfg.KeepFiles}, "keep-file", "files to keep from source directories")
	flag.Var(&tagWeightsParser{cfg.TagWeights}, "tag-weight", "adjust distance weighting for a tag between. 0 to ignore")
	flag.Var(&notificationsParser{&cfg.Notifications}, "notification-uri", "add a shoutrrr notification uri for an event")
	flag.Var(&addonsParser{&cfg.Addons}, "addon", "add some extra metadata when importing tracks")

	flag.StringVar(&cfg.MusicBrainzClient.BaseURL, "mb-base-url", `https://musicbrainz.org/ws/2/`, "musicbrainz base url")
	flag.DurationVar(&cfg.MusicBrainzClient.RateLimit, "mb-rate-limit", 1*time.Second, "musicbrainz rate limit duration")
	flag.StringVar(&cfg.CoverArtArchiveClient.BaseURL, "caa-base-url", `https://coverartarchive.org/`, "coverartarchive base url")
	flag.DurationVar(&cfg.CoverArtArchiveClient.RateLimit, "caa-rate-limit", 0, "coverartarchive rate limit duration")

	cfg.MusicBrainzClient.HTTPClient = httpClient
	cfg.CoverArtArchiveClient.HTTPClient = httpClient
}
