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
	"sort"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jba/muxpatterns"
	"github.com/peterbourgon/ff/v4"
	"github.com/r3labs/sse/v2"
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

	_jobs := map[string]*Job{} // TODO: sync
	jobQueue := make(chan JobConfig)

	ssesrv := sse.New()
	defer ssesrv.Close()

	const jobsStream = "jobs"
	ssesrv.CreateStream(jobsStream)

	sendJobWS := func() {
		var buff bytes.Buffer
		if err := templ.ExecuteTemplate(&buff, "jobs.html", jobList(_jobs)); err != nil {
			log.Printf("render job: %v", err)
			return
		}
		data := bytes.ReplaceAll(buff.Bytes(),
			[]byte("\n"), []byte(""),
		)
		ssesrv.Publish(jobsStream, &sse.Event{Data: data})
	}

	mux := muxpatterns.NewServeMux()
	mux.Handle("GET /events", ssesrv)

	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		jobQueue <- JobConfig{Path: path}
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobPath := decodeJobID(muxpatterns.PathValue(r, "id"))
		mbid := r.FormValue("mbid")
		jobQueue <- JobConfig{jobPath, mbid, false}

		if err := templ.ExecuteTemplate(w, "release.html", _jobs[jobPath]); err != nil {
			log.Printf("err in template: %v", err)
		}
	})
	mux.HandleFunc("POST /job/{id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		jobPath := decodeJobID(muxpatterns.PathValue(r, "id"))
		jobQueue <- JobConfig{jobPath, "", true}
		if err := templ.ExecuteTemplate(w, "release.html", _jobs[jobPath]); err != nil {
			log.Printf("err in template: %v", err)
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		data := struct{ Jobs []*Job }{Jobs: jobList(_jobs)}
		if err := templ.ExecuteTemplate(w, "index.html", data); err != nil {
			log.Printf("err in template: %v", err)
		}
	})

	for i := 0; i < 5; i++ {
		go func() {
			for jobConfig := range jobQueue {
				if err := processJob(context.Background(), mb, tg, _jobs, jobConfig); err != nil {
					log.Printf("error processing %q: %v", jobConfig.Path, err)
				}
				sendJobWS()
			}
		}()
	}

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

type JobConfig struct {
	Path    string
	UseMBID string
	ConfirmAnyway bool
}

type Job struct {
	ID       string
	Path     string
	Release  *release.Release
	Score    float64
	Diff     []release.Diff
	Complete bool
	Error    error
}

func newJob(path string) *Job {
	return &Job{ID: encodeJobID(path), Path: path}
}

func processJob(ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader, jobs map[string]*Job, jobConfig JobConfig) (err error) {
	if _, ok := jobs[jobConfig.Path]; !ok {
		jobs[jobConfig.Path] = newJob(jobConfig.Path)
	}

	job := jobs[jobConfig.Path]
	defer func() {
		job.Error = err
		job.Complete = true
	}()

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
	if jobConfig.UseMBID != "" {
		releaseTags.MBID = jobConfig.UseMBID
	}

	releaseMB, err := wrtag.SearchReleaseMusicBrainz(ctx, mb, releaseTags)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.Release = releaseMB
	job.Score, job.Diff = release.DiffReleases(releaseTags, releaseMB)

	if !jobConfig.ConfirmAnyway && job.Score < 95 {
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

func jobList(jobs map[string]*Job) []*Job {
	var r []*Job
	for _, j := range jobs {
		r = append(r, j)
	}
	sort.Slice(r, func(i, j int) bool {
		return r[i].Path < r[j].Path
	})
	return r
}
