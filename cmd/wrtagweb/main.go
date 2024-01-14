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
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	texttemplate "text/template"
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
			"file": func(p string) string {
				ur, _ := url.Parse("file://")
				ur.Path = p
				return ur.String()
			},
			"url": func(u string) template.URL{
				return template.URL(u)
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
	confPathFormat := ffs.StringLong("path-format", "", "path format")
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
	pathFormat := texttemplate.Must(wrtag.PathFormat.Parse(*confPathFormat))

	jobs := map[string]*Job{}
	jobQueue := make(chan JobConfig)
	var jmu sync.RWMutex

	sseServ := sse.New()
	defer sseServ.Close()

	const jobsStream = "jobs"
	sseServ.CreateStream(jobsStream)

	notifyClient := func() {
		jmu.RLock()
		defer jmu.RUnlock()

		var buff bytes.Buffer
		if err := templ.ExecuteTemplate(&buff, "jobs.html", listJobs(jobs)); err != nil {
			log.Printf("render job: %v", err)
			return
		}
		data := bytes.ReplaceAll(buff.Bytes(),
			[]byte("\n"), []byte(""),
		)
		sseServ.Publish(jobsStream, &sse.Event{Data: data})
	}

	for i := 0; i < 5; i++ {
		go func() {
			for jobConfig := range jobQueue {
				var job *Job
				jmu.RLock()
				job = jobs[jobConfig.Path]
				jmu.RUnlock()
				notifyClient()

				if err := processJob(context.Background(), mb, tg, pathFormat, job, jobConfig); err != nil {
					log.Printf("error processing %q: %v", jobConfig.Path, err)
				}
				notifyClient()
			}
		}()
	}

	mux := muxpatterns.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")

		jmu.Lock()
		jobs[path] = newJob(path)
		jmu.Unlock()

		jobQueue <- JobConfig{Path: path}
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobPath := decodeJobID(muxpatterns.PathValue(r, "id"))

		jmu.Lock()
		jobs[jobPath] = newJob(jobPath)
		jmu.Unlock()

		mbid := r.FormValue("mbid")
		jobQueue <- JobConfig{jobPath, mbid, false}

		jmu.RLock()
		if err := templ.ExecuteTemplate(w, "release.html", jobs[jobPath]); err != nil {
			log.Printf("err in template: %v", err)
		}
		jmu.RUnlock()
	})

	mux.HandleFunc("POST /job/{id}/confirm", func(w http.ResponseWriter, r *http.Request) {
		jobPath := decodeJobID(muxpatterns.PathValue(r, "id"))

		jmu.Lock()
		jobs[jobPath] = newJob(jobPath)
		jmu.Unlock()

		mbid := r.FormValue("mbid")
		jobQueue <- JobConfig{jobPath, mbid, true}

		jmu.RLock()
		if err := templ.ExecuteTemplate(w, "release.html", jobs[jobPath]); err != nil {
			log.Printf("err in template: %v", err)
		}
		jmu.RUnlock()
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		jmu.RLock()
		defer jmu.RUnlock()

		if err := templ.ExecuteTemplate(w, "index.html", listJobs(jobs)); err != nil {
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

func processJob(ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader, pathFormat *texttemplate.Template, job *Job, jobConfig JobConfig) (err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	defer func() {
		job.Loading = false
		job.Error = err
	}()

	releaseFiles, err := wrtag.ReadDir(tg, job.SourcePath)
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
	releaseMB, err := wrtag.SearchReleaseMusicBrainz(ctx, mb, releaseTags, jobConfig.UseMBID)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.MBID = releaseMB.MBID
	job.Score, job.Diff = release.DiffReleases(releaseTags, releaseMB)

	job.DestPath, err = wrtag.DestDir(pathFormat, *releaseMB)
	if err != nil {
		return fmt.Errorf("gen dest dir: %w", err)
	}

	if !jobConfig.ConfirmAnyway && job.Score < 95 {
		return wrtag.ErrNoMatch
	}

	release.ToTags(releaseMB, releaseFiles)
	job.Score, job.Diff = release.DiffReleases(releaseMB, releaseMB)

	return nil
}

func encodeJobID(path string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(path))
}

func decodeJobID(id string) string {
	r, _ := base64.RawURLEncoding.DecodeString(id)
	return string(r)
}

func listJobs(jobs map[string]*Job) []*Job {
	var r []*Job
	for _, j := range jobs {
		r = append(r, j)
	}
	sort.Slice(r, func(i, j int) bool {
		return r[i].SourcePath < r[j].SourcePath
	})
	return r
}

type JobConfig struct {
	Path          string
	UseMBID       string
	ConfirmAnyway bool
}

type Job struct {
	mu sync.Mutex

	ID, SourcePath, DestPath string

	MBID  string
	Score float64
	Diff  []release.Diff

	Loading bool
	Error   error
}

func newJob(path string) *Job {
	return &Job{ID: encodeJobID(path), SourcePath: path, Loading: true}
}
