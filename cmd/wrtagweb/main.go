package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jba/muxpatterns"
	"github.com/peterbourgon/ff/v4"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/release"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

//go:embed *.html
var ui embed.FS
var templ = template.Must(
	template.
		New("").
		Funcs(template.FuncMap{
			"query": template.URLQueryEscaper,
			"nomatch": func(err error) bool {
				return errors.Is(err, wrtag.ErrNoMatch)
			},
		}).
		ParseFS(ui, "*.html"),
)

func main() {
	ffs := ff.NewFlagSet("wrtag")
	_ = ffs.StringLong("path-format", "", "path format")
	confListenAddr := ffs.StringLong("listen-addr", "", "listen addr")
	confAPIKey := ffs.StringLong("api-key", "", "api-key")

	userConfig, _ := os.UserConfigDir()
	configPath := filepath.Join(userConfig, "wrtag", "config")
	_ = ffs.StringLong("config", configPath, "config file (optional)")

	ffopt := []ff.Option{
		ff.WithEnvVarPrefix("WRTAG"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	}
	if err := ff.Parse(ffs, os.Args[1:], ffopt...); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *confAPIKey == "" {
		fmt.Fprintln(os.Stderr, "need api key")
		os.Exit(1)
	}

	tg := &taglib.TagLib{}
	mb := musicbrainz.NewClient()

	jobs := map[string]*Job{} // TODO: sync
	jobQueue := make(chan string)

	for i := 0; i < 5; i++ {
		go func() {
			for jobPath := range jobQueue {
				if _, ok := jobs[jobPath]; !ok {
					jobs[jobPath] = &Job{Path: jobPath}
				}
				if err := processJob(context.Background(), mb, tg, jobs[jobPath]); err != nil {
					jobs[jobPath].Error = err
					log.Printf("error processing %q: %v", jobPath, err)
					continue
				}
			}
		}()
	}

	jobQueue <- "/home/senan/downloads/(2019) Fouk - Release The Kraken EP [FLAC]"
	jobQueue <- "/home/senan/downloads/testalb/"

	mux := muxpatterns.NewServeMux()
	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		jobQueue <- path
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobPath := muxpatterns.PathValue(r, "id")
		job, ok := jobs[jobPath]
		if !ok {
			http.Error(w, "unknown job", http.StatusBadRequest)
			return
		}
		*job = Job{
			Path:    job.Path,
			UseMBID: r.FormValue("mbid"),
		}
		jobQueue <- job.Path
		if err := templ.ExecuteTemplate(w, "release.html", job); err != nil {
			log.Printf("err in template: %v", err)
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Jobs map[string]*Job
		}{
			Jobs: jobs,
		}
		if err := templ.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("err in template: %v", err)
		}
	})

	log.Printf("starting on %s", *confListenAddr)
	log.Panicln(http.ListenAndServe(*confListenAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(*confAPIKey)) != 1 {
			w.Header().Set("WWW-Authenticate", "Basic")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})))
}

type Job struct {
	Path    string
	UseMBID string
	Release *release.Release
	Score   float64
	Diff    []release.Diff
	Error   error
}

func processJob(ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader, job *Job) (err error) {
	releaseFiles, err := wrtag.ReadDir(tg, job.Path)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	defer func() {
		var fileErrs []error
		for _, f := range releaseFiles {
			fileErrs = append(fileErrs, f.Close())
		}
		if err != nil {
			return
		}
		err = errors.Join(fileErrs...)
	}()

	releaseTags := release.FromTags(releaseFiles)

	releaseMB, err := wrtag.SearchReleaseMusicBrainz(ctx, mb, releaseTags)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.Release = releaseMB
	job.Score, job.Diff = release.DiffReleases(releaseTags, releaseMB)

	if job.Score < 95 {
		return wrtag.ErrNoMatch
	}

	release.ToTags(releaseMB, releaseFiles)

	return nil
}
