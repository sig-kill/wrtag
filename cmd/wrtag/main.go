package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/taglib"

	"github.com/peterbourgon/ff/v4"
)

func main() {
	ffs := ff.NewFlagSet("wrtag")
	confPathFormat := ffs.StringLong("path-format", "", "path format")
	_ = ffs.StringLong("config", "", "config file (optional)")

	userConfig, _ := os.UserConfigDir()
	configPath := filepath.Join(userConfig, "wrtag", "config")

	ffopt := []ff.Option{
		ff.WithEnvVarPrefix("WRTAG"),
		ff.WithConfigFileFlag("config"),
	}
	if stat, err := os.Stat(configPath); err == nil && stat.Mode().IsRegular() {
		ffopt = append(ffopt,
			ff.WithConfigFile(configPath),
			ff.WithConfigFileParser(ff.PlainParser),
		)
	}
	if err := ff.Parse(ffs, os.Args[1:], ffopt...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	pathFormat, err := template.
		New("template").
		Funcs(template.FuncMap{
			"title": func(ar []musicbrainz.ArtistCredit) []string {
				return mapp(ar, func(_ int, v musicbrainz.ArtistCredit) string { return v.Artist.Name })
			},
			"join": func(delim string, items []string) string { return strings.Join(items, delim) },
			"year": func(t time.Time) string { return fmt.Sprintf("%d", t.Year()) },
			"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
		}).
		Parse(*confPathFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing path format: %v\n", err)
		os.Exit(1)
	}

	mb := musicbrainz.NewClient()
	tg := taglib.TagLib{}

	for _, dir := range ffs.GetArgs() {
		_ = dir
		if err := wrtag.ProcessJob(context.Background(), mb, tg, pathFormat, nil, &wrtag.Job{}, wrtag.JobConfig{}); err != nil {
			log.Printf("error processing dir %q: %v", dir, err)
			continue
		}
	}
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}
