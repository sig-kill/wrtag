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
	"io"
	"log"
	"net/http"
	"net/url"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/r3labs/sse/v2"
	"github.com/timshannon/bolthold"
	"go.senan.xyz/flagconf"
	"go.senan.xyz/wrtag"
	"go.senan.xyz/wrtag/cmd/internal/flagparse"
	"go.senan.xyz/wrtag/musicbrainz"
	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
	"go.senan.xyz/wrtag/tags/taglib"
	"golang.org/x/sync/errgroup"
)

var tg = &taglib.TagLib{}
var mb = musicbrainz.NewClient(http.DefaultClient)

func main() {
	var pathFormat pathformat.Format
	flag.Var(flagparse.PathFormat{&pathFormat}, "path-format", "path-format")
	var tagWeights tagmap.TagWeights
	flag.Var(flagparse.TagWeights{&tagWeights}, "tag-weight", "tag weight")
	var researchLinkQuerier researchlink.Querier
	flag.Var(flagparse.Querier{&researchLinkQuerier}, "research-link", "research link")
	var keepFiles = map[string]struct{}{}
	flag.Func("keep-file", "files to keep from source directories",
		func(s string) error { keepFiles[s] = struct{}{}; return nil })
	var notifs notifications.Notifications
	flag.Var(flagparse.Notifications{&notifs}, "notification-uri", "shoutrrr notification uri")
	configPath := flag.String("config-path", flagparse.DefaultConfigPath, "path config file")

	confListenAddr := flag.String("listen-addr", "", "listen addr")
	confPublicURL := flag.String("public-url", "", "public url")
	confAPIKey := flag.String("api-key", "", "api key")
	confDBPath := flag.String("db-path", "wrtag.db", "db path")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)

	if *confAPIKey == "" {
		log.Fatal("need api key")
	}

	db, err := bolthold.Open(*confDBPath, 0600, nil)
	if err != nil {
		log.Fatalf("error parsing path format template: %v", err)
	}
	defer db.Close()

	sseServ := sse.New()
	sseServ.AutoReplay = false
	defer sseServ.Close()

	jobStream := sseServ.CreateStream("jobs")

	eventAllJobs := "jobs"
	eventUpdateJob := func(id uint64) string { return fmt.Sprintf("job-%d", id) }
	emit := func(e string) {
		sseServ.Publish(jobStream.ID, &sse.Event{Event: []byte(e), Data: []byte{0}})
	}

	processJob := func(ctx context.Context, job *Job, yes bool) error {
		job.Error = ""
		job.Status = StatusComplete

		var err error
		job.SearchResult, err = wrtag.ProcessDir(ctx, mb, tg, &pathFormat, tagWeights, &researchLinkQuerier, keepFiles, wrtagOperation(job.Operation), job.SourcePath, job.UseMBID, yes)
		if err != nil {
			if errors.Is(err, wrtag.ErrScoreTooLow) {
				job.Error = string(JobErrorNeedsInput)
				notifs.Send(notifications.NeedsInput, jobNotificationMessage(*confPublicURL, job))
				return nil
			}
			job.Error = err.Error()
			return nil
		}

		job.DestPath, err = wrtag.DestDir(&pathFormat, job.SearchResult.Release)
		if err != nil {
			return fmt.Errorf("gen dest dir: %w", err)
		}

		notifs.Send(notifications.Complete, jobNotificationMessage(*confPublicURL, job))

		// either if this was a copy or move job, subsequent re-imports should just be a move so we can retag
		job.Operation = OperationMove
		job.SourcePath = job.DestPath

		return nil
	}

	var buffPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	respTmpl := func(w http.ResponseWriter, name string, data any) {
		buff := buffPool.Get().(*bytes.Buffer)
		defer buffPool.Put(buff)
		buff.Reset()

		if err := uiTmpl.ExecuteTemplate(w, name, data); err != nil {
			log.Printf("error in template: %v", err)
			return
		}
		if _, err := io.Copy(w, buff); err != nil {
			log.Printf("error copying template data: %v", err)
			return
		}
	}
	respErr := func(w http.ResponseWriter, code int, f string, a ...any) {
		w.WriteHeader(code)
		respTmpl(w, "error", fmt.Sprintf(f, a...))
	}

	mux := http.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		q := &bolthold.Query{}
		search := r.URL.Query().Get("search")
		if search != "" {
			q = q.And("SourcePath").MatchFunc(func(path string) (bool, error) {
				return strings.Contains(strings.ToLower(path), strings.ToLower(search)), nil
			})
		}
		q = q.SortBy("Time")
		q = q.Reverse()

		var d struct {
			Total  int
			Jobs   []*Job
			Search string
		}
		if err := db.Find(&d.Jobs, q); err != nil {
			respErr(w, http.StatusInternalServerError, fmt.Sprintf("error listing jobs: %v", err))
			return
		}
		d.Total, _ = db.Count(&Job{}, &bolthold.Query{})
		d.Search = search
		respTmpl(w, "jobs", d)
	})

	mux.HandleFunc("POST /jobs", func(w http.ResponseWriter, r *http.Request) {
		operation, err := parseOperation(r.FormValue("operation"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := r.FormValue("path")
		if path == "" {
			http.Error(w, "no path provided", http.StatusBadRequest)
			return
		}
		job := Job{SourcePath: path, Operation: operation, Time: time.Now()}
		if err := db.Insert(bolthold.NextSequence(), &job); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}
		respTmpl(w, "job-import", nil)
		emit(eventAllJobs)
	})

	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		var job Job
		if err := db.Get(uint64(id), &job); err != nil {
			respErr(w, http.StatusInternalServerError, "error getting job")
			return
		}
		respTmpl(w, "job", job)
	})

	mux.HandleFunc("PUT /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		confirm, _ := strconv.ParseBool(r.FormValue("confirm"))
		useMBID := r.FormValue("mbid")
		if strings.Contains(useMBID, "/") {
			useMBID = filepath.Base(useMBID) // accept release URL
		}

		var job Job
		if err := db.Get(uint64(id), &job); err != nil {
			respErr(w, http.StatusInternalServerError, "error getting job")
			return
		}
		job.UseMBID = useMBID
		if err := processJob(r.Context(), &job, confirm); err != nil {
			respErr(w, http.StatusInternalServerError, "error in job")
			return
		}
		if err := db.Update(uint64(id), &job); err != nil {
			respErr(w, http.StatusInternalServerError, "save job")
			return
		}
		respTmpl(w, "job", job)
		emit(eventAllJobs)
	})

	mux.HandleFunc("DELETE /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		if err := db.Delete(uint64(id), &Job{}); err != nil {
			respErr(w, http.StatusInternalServerError, "error getting job")
			return
		}
		emit(eventAllJobs)
	})

	mux.HandleFunc("GET /dump", func(w http.ResponseWriter, r *http.Request) {
		var jobs []*Job
		if err := db.Find(&jobs, nil); err != nil {
			respErr(w, http.StatusInternalServerError, fmt.Sprintf("error listing jobs: %v", err))
			return
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			respErr(w, http.StatusInternalServerError, "error encoding jobs")
			return
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		var d struct {
			Total  int
			Jobs   []*Job
			Search string
		}
		if err := db.Find(&d.Jobs, (&bolthold.Query{}).SortBy("Time").Reverse()); err != nil {
			respErr(w, http.StatusInternalServerError, fmt.Sprintf("error listing jobs: %v", err))
			return
		}
		d.Total = len(d.Jobs)
		respTmpl(w, "index", d)
	})

	mux.Handle("/", http.FileServer(http.FS(ui)))

	// external API
	mux.HandleFunc("POST /op/{operation}", func(w http.ResponseWriter, r *http.Request) {
		operation, err := parseOperation(r.PathValue("operation"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		path := r.FormValue("path")
		if path == "" {
			http.Error(w, "no path provided", http.StatusBadRequest)
			return
		}
		job := Job{SourcePath: path, Operation: operation, Time: time.Now()}
		if err := db.Insert(bolthold.NextSequence(), &job); err != nil {
			http.Error(w, fmt.Sprintf("error saving job: %v", err), http.StatusInternalServerError)
			return
		}
		emit(eventAllJobs)
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		defer logJob("http")()

		mw := authMiddleware(*confAPIKey)
		server := &http.Server{Addr: *confListenAddr, Handler: mw(mux)}
		errgrp.Go(func() error { <-ctx.Done(); return server.Shutdown(context.Background()) })
		errgrp.Go(func() error { <-ctx.Done(); sseServ.Close(); return nil })

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	errgrp.Go(func() error {
		defer logJob("process jobs")()

		tick := func(ctx context.Context) error {
			var job Job
			switch err := db.FindOne(&job, bolthold.Where("Status").Eq(StatusIncomplete)); {
			case errors.Is(err, bolthold.ErrNotFound):
				return nil
			case err != nil:
				return fmt.Errorf("find next job: %w", err)
			}
			emit(eventUpdateJob(job.ID))
			defer func() {
				_ = db.Update(job.ID, &job)
				emit(eventUpdateJob(job.ID))
			}()
			return processJob(ctx, &job, false)
		}

		ctxTick(ctx, 2*time.Second, func() {
			if err := tick(ctx); err != nil {
				log.Printf("error in job: %v", err)
			}
		})
		return nil
	})

	if err := errgrp.Wait(); err != nil {
		log.Panic(err)
	}
}

type JobStatus string

const (
	StatusIncomplete JobStatus = ""
	StatusComplete   JobStatus = "complete"
)

type JobError string

const (
	JobErrorNeedsInput JobError = "needs-input"
)

type Operation string

const (
	OperationCopy Operation = "copy"
	OperationMove Operation = "move"
)

func parseOperation(str string) (Operation, error) {
	switch str {
	case "copy":
		return OperationCopy, nil
	case "move":
		return OperationMove, nil
	default:
		return "", fmt.Errorf("invalid operation %q", str)
	}
}

func wrtagOperation(op Operation) wrtag.FileSystemOperation {
	switch op {
	case OperationCopy:
		return wrtag.Copy{}
	case OperationMove:
		return wrtag.Move{}
	default:
		panic(fmt.Errorf("unknown operation: %q", op))
	}
}

type Job struct {
	ID                   uint64    `boltholdKey:"ID"`
	Status               JobStatus `boltholdIndex:"Status"`
	Error                string
	Operation            Operation
	Time                 time.Time `boltholdIndex:"Time"`
	UseMBID              string
	SourcePath, DestPath string
	SearchResult         *wrtag.SearchResult
}

func jobNotificationMessage(publicURL string, job *Job) string {
	var status string
	if job.Error != "" {
		status = job.Error
	} else if job.Status != "" {
		status = string(job.Status)
	}

	url, _ := url.Parse(publicURL)
	url.Fragment = fmt.Sprint(job.ID)

	return fmt.Sprintf(`%s %s (%s) %s`,
		job.Operation, status, job.SourcePath, url)
}

//go:embed *.gohtml *.ico dist/*
var ui embed.FS
var uiTmpl = htmltemplate.Must(
	htmltemplate.
		New("template").
		Funcs(funcMap).
		ParseFS(ui, "*.gohtml"),
)

var funcMap = htmltemplate.FuncMap{
	"now":  func() int64 { return time.Now().UnixMilli() },
	"file": func(p string) string { ur, _ := url.Parse("file://"); ur.Path = p; return ur.String() },
	"url":  func(u string) htmltemplate.URL { return htmltemplate.URL(u) },
	"join": func(delim string, items []string) string { return strings.Join(items, delim) },
	"pad0": func(amount, n int) string { return fmt.Sprintf("%0*d", amount, n) },
}

func logJob(jobName string) func() {
	log.Printf("starting job %q", jobName)
	return func() { log.Printf("stopped job %q", jobName) }
}

func authMiddleware(apiKey string) func(next http.Handler) http.Handler {
	const cookieKey = "api-key"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("req for %s", r.URL)

			// exchange a valid basic basic auth request for a cookie that lasts 30 days
			if cookie, _ := r.Cookie(cookieKey); cookie != nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(apiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) == 1 {
				http.SetCookie(w, &http.Cookie{Name: cookieKey, Value: apiKey, HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, Path: "/", Expires: time.Now().Add(30 * 24 * time.Hour)})
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "unauthorised", http.StatusUnauthorized)
		})
	}
}

func ctxTick(ctx context.Context, interval time.Duration, f func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f()
		}
	}
}
