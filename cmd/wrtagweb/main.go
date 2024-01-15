package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
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

	"github.com/jba/muxpatterns"
	"github.com/peterbourgon/ff/v4"
	"github.com/r3labs/sse/v2"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/taglib"
)

//go:embed *.html *.ico
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
			"url": func(u string) template.URL {
				return template.URL(u)
			},
			"query": template.URLQueryEscaper,
			"nomatch": func(err error) bool {
				return errors.Is(err, wrtag.ErrNoMatch)
			},
		}).
		ParseFS(ui, "*.html"),
)

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

	jobs := map[string]*wrtag.Job{}
	jobQueue := make(chan wrtag.JobConfig)
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
				var job *wrtag.Job
				jmu.RLock()
				job = jobs[jobConfig.Path]
				jmu.RUnlock()
				notifyClient()

				if err := wrtag.ProcessJob(context.Background(), mb, tg, pathFormat, job, jobConfig); err != nil {
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
		jobs[path] = wrtag.NewJob(path)
		jmu.Unlock()

		jobQueue <- wrtag.JobConfig{Path: path}
	})

	mux.HandleFunc("POST /job/{id}/{action...}", func(w http.ResponseWriter, r *http.Request) {
		id := muxpatterns.PathValue(r, "id")
		action := muxpatterns.PathValue(r, "action")
		mbid := r.FormValue("mbid")

		jobPath := wrtag.DecodeJobID(id)

		jmu.Lock()
		jobs[jobPath] = wrtag.NewJob(jobPath)
		jmu.Unlock()

		switch action {
		case "confirm":
			jobQueue <- wrtag.JobConfig{Path: jobPath, UseMBID: mbid, ConfirmAnyway: true}
		default:
			jobQueue <- wrtag.JobConfig{Path: jobPath, UseMBID: mbid}
		}

		jmu.RLock()
		if err := templ.ExecuteTemplate(w, "release.html", jobs[jobPath]); err != nil {
			log.Printf("err in template: %v", err)
		}
		jmu.RUnlock()
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

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

func listJobs(jobs map[string]*wrtag.Job) []*wrtag.Job {
	var r []*wrtag.Job
	for _, j := range jobs {
		r = append(r, j)
	}
	sort.Slice(r, func(i, j int) bool {
		return r[i].SourcePath < r[j].SourcePath
	})
	return r
}
