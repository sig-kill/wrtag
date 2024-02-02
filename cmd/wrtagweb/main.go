package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	htmltemplate "html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/jba/muxpatterns"
	"github.com/r3labs/sse/v2"
	"github.com/timshannon/bolthold"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/config"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags/tagcommon"
	"go.senan.xyz/wrtag/tags/taglib"
)

func main() {
	confListenAddr := flag.String("listen-addr", "", "listen addr")
	confAPIKey := flag.String("api-key", "", "api key")
	confDBPath := flag.String("db-path", "wrtag.db", "db path")
	config.Parse()

	if *confAPIKey == "" {
		log.Fatal("need api key")
	}

	pathFormat, err := pathformat.New(config.PathFormat)
	if err != nil {
		log.Fatalf("gen path format: %v", err)
	}

	var tg = &taglib.TagLib{}
	var mb = musicbrainz.NewClient()

	var researchLinkQuerier = &researchlink.Querier{}
	for _, r := range config.ResearchLinks {
		if err := researchLinkQuerier.AddSource(r.Name, r.Template); err != nil {
			log.Fatalf("add researchlink querier source: %v", err)
		}
	}

	db, err := bolthold.Open(*confDBPath, 0600, nil)
	if err != nil {
		log.Fatalf("error parsing path format template: %v", err)
	}
	defer db.Close()

	sseServ := sse.New()
	defer sseServ.Close()

	jobStream := sseServ.CreateStream("jobs")

	pushJob := func(job *Job) error {
		var buff bytes.Buffer
		if err := uiTempl.ExecuteTemplate(&buff, "release.html", job); err != nil {
			return fmt.Errorf("render jobs template: %w", err)
		}
		data := bytes.ReplaceAll(buff.Bytes(), []byte("\n"), []byte{})
		sseServ.Publish(jobStream.ID, &sse.Event{Data: data})
		return nil
	}

	jobTick := func() error {
		var job Job
		switch err := db.FindOne(&job, bolthold.Where("Status").Eq(StatusIncomplete)); {
		case errors.Is(err, bolthold.ErrNotFound):
			return nil
		case err != nil:
			return fmt.Errorf("find next job: %w", err)
		}

		if err := pushJob(&job); err != nil {
			log.Printf("push job: %v", err)
		}
		defer func() {
			_ = db.Update(job.ID, &job)
			_ = pushJob(&job)
		}()

		if err := processJob(context.Background(), mb, tg, pathFormat, researchLinkQuerier, &job, "", false); err != nil {
			return fmt.Errorf("process job: %w", err)
		}
		return nil
	}

	go func() {
		for {
			if err := jobTick(); err != nil {
				log.Printf("error in job: %v", err)
			}
			time.Sleep(2 * time.Second)
		}
	}()

	mux := muxpatterns.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		job := Job{SourcePath: path}
		if err := db.Insert(bolthold.NextSequence(), &job); err != nil {
			http.Error(w, "error saving job", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(muxpatterns.PathValue(r, "id"))
		confirm, _ := strconv.ParseBool(r.FormValue("confirm"))
		useMBID := filepath.Base(r.FormValue("mbid"))

		var job Job
		if err := db.Get(id, &job); err != nil {
			http.Error(w, "error getting job", http.StatusInternalServerError)
			return
		}
		if err := processJob(r.Context(), mb, tg, pathFormat, researchLinkQuerier, &job, useMBID, confirm); err != nil {
			log.Printf("error processing job %d: %v", id, err)
			http.Error(w, "error in job", http.StatusInternalServerError)
			return
		}
		if err := db.Update(id, &job); err != nil {
			http.Error(w, "save job", http.StatusInternalServerError)
			return
		}
		if err := uiTempl.ExecuteTemplate(w, "release.html", struct {
			*Job
			UseMBID string
		}{&job, useMBID}); err != nil {
			log.Printf("err in template: %v", err)
			return
		}
	})

	mux.HandleFunc("GET /dump", func(w http.ResponseWriter, r *http.Request) {
		var jobs []*Job
		if err := db.Find(&jobs, nil); err != nil {
			http.Error(w, fmt.Sprintf("error listing jobs: %v", err), http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			http.Error(w, "error encoding jobs", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		var jobs []*Job
		if err := db.Find(&jobs, nil); err != nil {
			http.Error(w, fmt.Sprintf("error listing jobs: %v", err), http.StatusInternalServerError)
			return
		}
		if err := uiTempl.ExecuteTemplate(w, "index.html", jobs); err != nil {
			log.Printf("err in template: %v", err)
			return
		}
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

	log.Printf("starting on %s", *confListenAddr)
	log.Panicln(http.ListenAndServe(*confListenAddr, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", "Basic")
		if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(*confAPIKey)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})))
}

type JobStatus string

const (
	StatusIncomplete JobStatus = ""
	StatusComplete   JobStatus = "complete"
	StatusNoMatch    JobStatus = "no-match"
	StatusError      JobStatus = "error"
)

type Job struct {
	ID                   uint64    `boltholdKey:"ID"`
	Status               JobStatus `boltholdIndex:"Status"`
	Info                 string
	SourcePath, DestPath string
	Score                float64
	MBID                 string
	Diff                 []tagmap.Diff
	ResearchLinks        []researchlink.SearchResult
}

func processJob(
	ctx context.Context, mb *musicbrainz.Client, tg tagcommon.Reader,
	pathFormat *texttemplate.Template, researchLinkQuerier *researchlink.Querier,
	job *Job,
	useMBID string, confirm bool,
) (err error) {
	job.Score = 0
	job.MBID = ""
	job.Diff = nil
	job.ResearchLinks = nil

	job.Info = ""
	defer func() {
		if err != nil {
			job.Status = StatusError
			job.Info = err.Error()
		}
	}()

	tagFiles, err := wrtag.ReadDir(tg, job.SourcePath)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", job.SourcePath, err)
	}
	defer func() {
		var fileErrs []error
		for _, f := range tagFiles {
			fileErrs = append(fileErrs, f.Close())
		}
		if err != nil {
			return
		}
		err = errors.Join(fileErrs...)
	}()

	searchFile := tagFiles[0]
	query := musicbrainz.ReleaseQuery{
		MBReleaseID:      searchFile.MBReleaseID(),
		MBArtistID:       first(searchFile.MBArtistID()),
		MBReleaseGroupID: searchFile.MBReleaseGroupID(),
		Release:          searchFile.Album(),
		Artist:           or(searchFile.AlbumArtist(), searchFile.Artist()),
		Date:             searchFile.Date(),
		Format:           searchFile.MediaFormat(),
		Label:            searchFile.Label(),
		CatalogueNum:     searchFile.CatalogueNum(),
		NumTracks:        len(tagFiles),
	}
	if useMBID != "" {
		query.MBReleaseID = useMBID
	}

	job.ResearchLinks, err = researchLinkQuerier.Search(searchFile)
	if err != nil {
		return fmt.Errorf("research querier search: %w", err)
	}

	release, err := mb.SearchRelease(ctx, query)
	if err != nil {
		return fmt.Errorf("search musicbrainz: %w", err)
	}

	job.MBID = release.ID
	job.Score, job.Diff = tagmap.DiffRelease(release, tagFiles)

	job.DestPath, err = wrtag.DestDir(pathFormat, *release)
	if err != nil {
		return fmt.Errorf("gen dest dir: %w", err)
	}

	if releaseTracks := musicbrainz.FlatTracks(release.Media); len(tagFiles) != len(releaseTracks) {
		return fmt.Errorf("%w: %d/%d", wrtag.ErrTrackCountMismatch, len(tagFiles), len(releaseTracks))
	}
	if !confirm && job.Score < 95 {
		job.Status = StatusNoMatch
		return nil
	}

	// write release to tags. files are saved by defered Close()
	tagmap.WriteRelease(release, tagFiles)

	job.Score, job.Diff = tagmap.DiffRelease(release, tagFiles)
	job.SourcePath = job.DestPath
	job.Status = StatusComplete

	if err := wrtag.MoveFiles(pathFormat, release, nil); err != nil {
		return fmt.Errorf("move files: %w", err)
	}

	return nil
}

//go:embed *.html *.ico
var ui embed.FS
var uiTempl = htmltemplate.Must(
	htmltemplate.
		New("template").
		Funcs(funcMap).
		ParseFS(ui, "*.html"),
)

var funcMap = htmltemplate.FuncMap{
	"now":   func() int64 { return time.Now().UnixMilli() },
	"file":  func(p string) string { ur, _ := url.Parse("file://"); ur.Path = p; return ur.String() },
	"url":   func(u string) htmltemplate.URL { return htmltemplate.URL(u) },
	"query": htmltemplate.URLQueryEscaper,
	"join":  func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0":  func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
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

func or[T comparable](items ...T) T {
	var zero T
	for _, i := range items {
		if i != zero {
			return i
		}
	}
	return zero
}
