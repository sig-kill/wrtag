package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
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
			"now": func() int64 {
				return time.Now().UnixMilli()
			},
			"query": template.URLQueryEscaper,
			"nomatch": func(err error) bool {
				return errors.Is(err, wrtag.ErrNoMatch)
			},
		}).
		ParseFS(ui, "*.html"),
)

var wsu websocket.Upgrader

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

	wsc := map[*websocket.Conn]struct{}{}

	sendJobWS := func(jobPath string) {
		job, ok := jobs[jobPath]
		if !ok {
			return
		}
		var buff bytes.Buffer
		if err := templ.ExecuteTemplate(&buff, "release.html", job); err != nil {
			log.Printf("render job: %v", err)
			return
		}
		for c := range wsc {
			if err := c.WriteMessage(websocket.TextMessage, buff.Bytes()); err != nil {
				c.Close()
				delete(wsc, c)
			}
		}
	}

	for i := 0; i < 5; i++ {
		go func() {
			for jobPath := range jobQueue {
				if _, ok := jobs[jobPath]; !ok {
					jobs[jobPath] = newJob(jobPath, "")
				}
				if err := processJob(context.Background(), mb, tg, jobs[jobPath]); err != nil {
					jobs[jobPath].Error = err
					log.Printf("error processing %q: %v", jobPath, err)
					continue
				}
				sendJobWS(jobPath)
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

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsu.Upgrade(w, r, nil)
		if err != nil {
			fmt.Printf("error upgrading: %v", err)
			return
		}
		wsc[conn] = struct{}{}
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobPath := decodeJobID(muxpatterns.PathValue(r, "id"))

		useMBID := muxpatterns.PathValue(r, "mbid")
		job := newJob(jobPath, useMBID)
		jobs[jobPath] = job

		jobQueue <- jobPath

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
	ID      string
	Path    string
	UseMBID string
	Release *release.Release
	Score   float64
	Diff    []release.Diff
	Error   error
}

func newJob(path string, mbid string) *Job {
	return &Job{ID: encodeJobID(path), Path: path, UseMBID: mbid}
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

func encodeJobID(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

func decodeJobID(id string) string {
	r, _ := base64.RawURLEncoding.DecodeString(id)
	return string(r)
}
