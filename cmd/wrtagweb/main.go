package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	texttemplate "text/template"

	"github.com/jba/muxpatterns"
	"github.com/peterbourgon/ff/v4"
	"github.com/r3labs/sse/v2"
	"go.etcd.io/bbolt"
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

	db, err := bbolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatalf("error parsing path format template: %v", err)
	}
	defer db.Close()

	odb, err := newObjectDB([]byte("b"), db)
	if err != nil {
		log.Panicf("error creating object db: %v", err)
	}

	jobQueue := make(chan uint64)

	sseServ := sse.New()
	defer sseServ.Close()

	const jobsStream = "jobs"
	sseServ.CreateStream(jobsStream)

	notifyClient := func() {
		jobs, err := odb.list()
		if err != nil {
			log.Printf("error listings jobs: %v", err)
			return
		}
		var buff bytes.Buffer
		if err := uiTempl.ExecuteTemplate(&buff, "jobs.html", jobs); err != nil {
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
			for id := range jobQueue {
				err := func() (err error) {
					job, err := odb.get(id)
					if err != nil {
						return fmt.Errorf("get job: %w", err)
					}

					notifyClient()
					defer func() {
						_ = odb.save(job)
						notifyClient()
					}()

					if err := wrtag.ProcessJob(context.Background(), mb, tg, pathFormat, searchLinkTemplates, job, "", false); err != nil {
						return fmt.Errorf("process job: %w", err)
					}
					return nil
				}()
				if err != nil {
					log.Printf("job %d: %v", id, err)
				}
			}
		}()
	}

	mux := muxpatterns.NewServeMux()
	mux.Handle("GET /events", sseServ)

	mux.HandleFunc("POST /copy", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("path")
		job := wrtag.Job{
			SourcePath: path,
		}
		if err := odb.save(&job); err != nil {
			http.Error(w, "error saving job", http.StatusInternalServerError)
			return
		}
		jobQueue <- job.ID
	})

	mux.HandleFunc("POST /job/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(muxpatterns.PathValue(r, "id"))
		confirm, _ := strconv.ParseBool(r.FormValue("confirm"))
		mbid := filepath.Base(r.FormValue("mbid"))
		job, err := odb.get(uint64(id))
		if err != nil {
			http.Error(w, "error getting job", http.StatusInternalServerError)
			return
		}
		if err := wrtag.ProcessJob(r.Context(), mb, tg, pathFormat, searchLinkTemplates, job, mbid, confirm); err != nil {
			log.Printf("error processing job %d: %v", id, err)
			http.Error(w, "error in job", http.StatusInternalServerError)
			return
		}
		if err := odb.save(job); err != nil {
			http.Error(w, "save job", http.StatusInternalServerError)
			return
		}
		if err := uiTempl.ExecuteTemplate(w, "release.html", struct {
			*wrtag.Job
			UseMBID string
		}{job, mbid}); err != nil {
			log.Printf("err in template: %v", err)
			return
		}
	})

	mux.HandleFunc("GET /dump", func(w http.ResponseWriter, r *http.Request) {
		jobs, err := odb.list()
		if err != nil {
			http.Error(w, "error listing jobs", http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(jobs); err != nil {
			http.Error(w, "error encoding jobs", http.StatusInternalServerError)
			return
		}
	})

	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		jobs, err := odb.list()
		if err != nil {
			http.Error(w, "error listing jobs", http.StatusInternalServerError)
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
		if _, key, _ := r.BasicAuth(); subtle.ConstantTimeCompare([]byte(key), []byte(*confAPIKey)) != 1 {
			w.Header().Set("WWW-Authenticate", "Basic")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})))
}

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

type objectDB struct {
	bucket []byte
	bolt   *bbolt.DB
}

func newObjectDB(bucket []byte, bolt *bbolt.DB) (objectDB, error) {
	err := bolt.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	})
	if err != nil {
		return objectDB{}, err
	}
	return objectDB{bucket, bolt}, nil
}

func (o *objectDB) get(id uint64) (*wrtag.Job, error) {
	var j wrtag.Job
	return &j, o.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(o.bucket)
		data := b.Get(itob(id))
		return json.NewDecoder(bytes.NewReader(data)).Decode(&j)
	})
}

func (o *objectDB) save(j *wrtag.Job) error {
	return o.bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(o.bucket)
		if j.ID == 0 {
			var err error
			j.ID, err = b.NextSequence()
			if err != nil {
				return err
			}
		}
		var buff bytes.Buffer
		_ = json.NewEncoder(&buff).Encode(j)
		return b.Put(itob(j.ID), buff.Bytes())
	})
}

func (o *objectDB) list() ([]*wrtag.Job, error) {
	var r []*wrtag.Job
	return r, o.bolt.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(o.bucket)
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var j wrtag.Job
			_ = json.NewDecoder(bytes.NewReader(v)).Decode(&j)
			r = append(r, &j)
		}
		return nil
	})
}

func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func btoi(v []byte) uint64 {
	return binary.BigEndian.Uint64(v)
}
