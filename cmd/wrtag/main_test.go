package main

import (
	"embed"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/fileutil"
)

//go:embed testdata/responses
var responses embed.FS

func init() {
	mb.MBClient.RateLimit = 0
	mb.CAAClient.RateLimit = 0
	mb.MBClient.HTTPClient = clientutil.FSClient(responses, "testdata/responses/musicbrainz")
	mb.CAAClient.HTTPClient = clientutil.FSClient(responses, "testdata/responses/coverartarchive")

	// panic if someone tries to use the default client/transport
	http.DefaultClient.Transport = panicTransport
	http.DefaultTransport = panicTransport
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtag":     func() int { main(); return 0 },
		"tag-write": func() int { mainTagWrite(); return 0 },
		"tag-check": func() int { mainTagCheck(); return 0 },
		"find":      func() int { mainFind(); return 0 },
		"touch":     func() int { mainTouch(); return 0 },
		"mime":      func() int { mainMIME(); return 0 },
		"mod-time":  func() int { mainModTime(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

func mainTagWrite() {
	flag.Parse()

	pat := flag.Arg(0)
	paths := parsePattern(pat)
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	pairs := flag.Args()[1:]
	if len(pairs)%2 != 0 {
		log.Fatalf("invalid field/value pairs")
	}

	for _, p := range paths {
		if err := ensureFlac(p); err != nil {
			log.Fatalf("ensure flac: %v", err)
		}
		f, err := tg.Read(p)
		if err != nil {
			log.Fatalf("open tag file: %v", err)
		}

		for i := 0; i < len(pairs)-1; i += 2 {
			field, jsonValue := pairs[i], pairs[i+1]

			method := reflect.ValueOf(f).MethodByName("Write" + field)
			dest := reflect.New(method.Type().In(0))
			if err := json.Unmarshal([]byte(jsonValue), dest.Interface()); err != nil {
				log.Fatalf("unmarshal json to arg: %v", err)
			}
			method.Call([]reflect.Value{dest.Elem()})
		}

		f.Close()
	}
}

func mainTagCheck() {
	flag.Parse()

	pat := flag.Arg(0)
	paths := parsePattern(pat)
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	pairs := flag.Args()[1:]
	if len(pairs)%2 != 0 {
		log.Fatalf("invalid field/value pairs")
	}

	for _, p := range paths {
		f, err := tg.Read(p)
		if err != nil {
			log.Fatalf("open tag file: %v", err)
		}

		for i := 0; i < len(pairs)-1; i += 2 {
			field, jsonValue := pairs[i], pairs[i+1]

			method := reflect.ValueOf(f).MethodByName(field)
			dest := reflect.New(method.Type().Out(0))
			if err := json.Unmarshal([]byte(jsonValue), dest.Interface()); err != nil {
				log.Fatalf("unmarshal json to arg: %v", err)
			}
			result := method.Call(nil)
			exp, act := dest.Elem().Interface(), result[0].Interface()
			if !reflect.DeepEqual(exp, act) {
				log.Fatalf("exp %q got %q", exp, act)
			}
		}

		f.Close()
	}
}

func mainFind() {
	flag.Parse()

	paths := flag.Args()
	sort.Strings(paths)

	for _, p := range paths {
		err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		})
		if err != nil {
			log.Fatal(err)
		}
	}
}

func mainTouch() {
	flag.Parse()

	for _, p := range flag.Args() {
		if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
			log.Fatalf("mkdirall: %v", err)
		}
		if _, err := os.Create(p); err != nil {
			log.Fatalf("err creating: %v", err)
		}
	}
}

func mainMIME() {
	flag.Parse()

	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatalf("error reading: %v", err)
	}

	mime := http.DetectContentType(data)
	fmt.Println(mime)
}

func mainModTime() {
	flag.Parse()

	paths := parsePattern(flag.Arg(0))
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			log.Fatalf("error stating: %v", err)
		}
		fmt.Println(info.ModTime().UnixNano())
	}
}

var panicTransport = clientutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
	panic("panic transport used")
})

//go:embed testdata/empty.flac
var emptyFlac []byte

func ensureFlac(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("make parents: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("open and trunc file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(emptyFlac); err != nil {
		return fmt.Errorf("write empty file: %w", err)
	}
	return nil
}

func parsePattern(pat string) []string {
	// assume the file exists if the pattern doesn't look like a glob
	if fileutil.GlobEscape(pat) == pat {
		return []string{pat}
	}
	paths, _ := filepath.Glob(pat)
	return paths
}
