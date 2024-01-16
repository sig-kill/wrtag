package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"

	"github.com/jba/muxpatterns"
	"github.com/peterbourgon/ff/v4"
	"github.com/r3labs/sse/v2"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/tags/taglib"
)

//go:embed *.html *.ico
var ui embed.FS
var uiTempl = template.Must(
	template.
		New("template").
		Funcs(wrtag.TemplateFuncMap).
		ParseFS(ui, "*.html"),
)

func main() {
	ffs := ff.NewFlagSet("wrtag")
	confPathFormat := ffs.StringLong("path-format", "", "path format")
	confListenAddr := ffs.StringLong("listen-addr", "", "listen addr")

	var confSearchLinkTemplates searchLinkTemplates
	ffs.ValueLong("search-link", &confSearchLinkTemplates, "search link")

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
		log.Fatal("parse err")
	}
	if *confAPIKey == "" {
		log.Fatal("need api key")
	}

	var searchLinkTemplates []wrtag.SearchLinkTemplate
	for _, c := range confSearchLinkTemplates {
		templ, err := texttemplate.New("template").Funcs(wrtag.TemplateFuncMap).Parse(c.Template)
		if err != nil {
			log.Fatalf("error parsing search template: %v", err)
		}
		searchLinkTemplates = append(searchLinkTemplates, wrtag.SearchLinkTemplate{
			Name:  c.Name,
			Templ: templ,
		})
	}

	pathFormat, err := wrtag.PathFormatTemplate(*confPathFormat)
	if err != nil {
		log.Fatalf("error parsing path format template: %v", err)
	}

	tg := &taglib.TagLib{}
	mb := musicbrainz.NewClient()

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
		if err := uiTempl.ExecuteTemplate(&buff, "jobs.html", listJobs(jobs)); err != nil {
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
			for jobC := range jobQueue {
				var job *wrtag.Job
				jmu.Lock()
				if _, ok := jobs[jobC.Path]; !ok {
					jobs[jobC.Path] = &wrtag.Job{ID: encodeJobID(jobC.Path), SourcePath: jobC.Path}
				}
				job = jobs[jobC.Path]
				jmu.Unlock()
				notifyClient()

				if err := wrtag.ProcessJob(context.Background(), mb, tg, pathFormat, searchLinkTemplates, job, jobC); err != nil {
					log.Printf("error processing %q: %v", jobC.Path, err)
				}
				notifyClient()
			}
		}()
	}

	mux := muxpatterns.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		jobQueue <- wrtag.JobConfig{Path: path}
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := muxpatterns.PathValue(r, "id")
		jobPath := decodeJobID(id)

		confirm, _ := strconv.ParseBool(r.FormValue("confirm"))
		mbid := r.FormValue("mbid")

		jobQueue <- wrtag.JobConfig{Path: jobPath, UseMBID: mbid, ConfirmAnyway: confirm}

		jmu.RLock()
		if err := uiTempl.ExecuteTemplate(w, "release.html", &wrtag.Job{Loading: true}); err != nil {
			log.Printf("err in template: %v", err)
		}
		jmu.RUnlock()
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		jmu.RLock()
		defer jmu.RUnlock()

		if err := uiTempl.ExecuteTemplate(w, "index.html", listJobs(jobs)); err != nil {
			log.Printf("err in template: %v", err)
		}
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

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

func encodeJobID(path string) string { return base64.RawURLEncoding.EncodeToString([]byte(path)) }
func decodeJobID(id string) string   { r, _ := base64.RawURLEncoding.DecodeString(id); return string(r) }

type searchLinkTemplates []searchLinkTemplate
type searchLinkTemplate struct {
	Name     string
	Template string
}

func (sls searchLinkTemplates) String() string {
	var names []string
	for _, sl := range sls {
		names = append(names, sl.Name)
	}
	return strings.Join(names, ", ")
}

func (sls *searchLinkTemplates) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	*sls = append(*sls, searchLinkTemplate{Name: name, Template: value})
	return nil
}
