package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/taglib"

	"github.com/peterbourgon/ff/v4"
)

func main() {
	fs := ff.NewFlagSet("wrtag")
	confPathFormat := fs.StringLong("path-format", "", "path format")
	_ = fs.StringLong("config", "", "config file (optional)")

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
	if err := ff.Parse(fs, os.Args[1:], ffopt...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	pathFormat, err := template.
		New("template").
		Funcs(template.FuncMap{
			"title": func(ar []release.Artist) []string {
				return mapp(ar, func(_ int, v release.Artist) string { return v.Title })
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

	tg := taglib.TagLib{}
	mb := musicbrainz.NewClient()

	for _, dir := range fs.GetArgs() {
		if err := processDir(tg, mb, pathFormat, dir); err != nil {
			log.Printf("error processing dir %q: %v", dir, err)
			continue
		}
	}
}

func a() {
	//
	// release.ToTags(releaseMB, files)
	//
	// var errs []error
	// for _, t := range files {
	// 	errs = append(errs, t.Close())
	// }
	// if err := errors.Join(errs...); err != nil {
	// 	return fmt.Errorf("write tags to files: %w", err)
	// }
	// if err := moveFiles(pathFormat, releaseMB, paths); err != nil {
	// 	log.Printf("error processing dir %q: %v", dir, err)
	// }
	// return nil
}

func moveFiles(pathFormat *template.Template, releaseMB *release.Release, paths []string) error {
	for i, t := range releaseMB.Tracks {
		path := paths[i]
		pathFormatData := struct {
			R   *release.Release
			T   *release.Track
			Ext string
		}{
			R:   releaseMB,
			T:   &t,
			Ext: filepath.Ext(path),
		}

		var newPathBuilder strings.Builder
		if err := pathFormat.Execute(&newPathBuilder, pathFormatData); err != nil {
			return fmt.Errorf("create path: %w", err)
		}
	}
	return nil
}

func first[T comparable](is []T) T {
	var z T
	for _, i := range is {
		if i != z {
			return i
		}
	}
	return z
}

func mapp[F, T any](s []F, f func(int, F) T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = f(i, v)
	}
	return res
}
