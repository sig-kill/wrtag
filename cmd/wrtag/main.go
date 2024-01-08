package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/tagcommon"
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

func processDir(tg taglib.TagLib, mb *musicbrainz.Client, pathFormat *template.Template, dir string) error {
	rtags, err := readReleaseDir(tg, dir)
	if err != nil {
		return fmt.Errorf("read release dir: %w", err)
	}

	var query musicbrainz.Query
	query.MBReleaseID = rtags.MBID
	query.MBArtistID = first(rtags.Artists).MBID
	query.MBReleaseGroupID = rtags.ReleaseGroupMBID
	query.Release = rtags.Title
	query.Artist = rtags.ArtistCredit
	query.Format = rtags.MediaFormat
	query.Date = fmt.Sprint(rtags.Date.Year())
	query.Label = rtags.Label
	query.CatalogueNum = rtags.CatalogueNum
	query.NumTracks = len(rtags.Tracks)

	score, resp, err := mb.SearchRelease(context.Background(), query)
	if err != nil {
		return fmt.Errorf("search release: %w", err)
	}
	if score < 100 {
		return fmt.Errorf("score too low")
	}

	releaseMB := release.FromMusicBrainz(resp)
	if len(rtags.Tracks) != len(releaseMB.Tracks) {
		return fmt.Errorf("track count mismatch %d/%d", len(rtags.Tracks), len(releaseMB.Tracks))
	}

	fmt.Println()
	fmt.Print(release.Diff(rtags, releaseMB))

	return nil
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

func readReleaseDir(tg taglib.TagLib, dir string) (*release.Release, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return nil, fmt.Errorf("glob dir: %w", err)
	}
	sort.Strings(paths)

	var files []tagcommon.File
	for _, path := range paths {
		if tg.CanRead(path) {
			file, err := tg.Read(path)
			if err != nil {
				return nil, fmt.Errorf("read track: %w", err)
			}
			files = append(files, file)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no tracks in dir")
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	return release.FromTags(files), nil
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
