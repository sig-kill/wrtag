package main

import (
	"crypto/rand"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/clientutil"
	"go.senan.xyz/wrtag/fileutil"
	"go.senan.xyz/wrtag/tags"
)

//go:embed testdata/responses
var responses embed.FS

func init() {
	mb.MBClient.RateLimit = 0
	mb.CAAClient.RateLimit = 0
	mb.MBClient.HTTPClient = clientutil.FSClient(responses, "testdata/responses/musicbrainz")
	mb.CAAClient.HTTPClient = clientutil.FSClient(responses, "testdata/responses/coverartarchive")

	mb.MBClient.HTTPClient = clientutil.Wrap(mb.MBClient.HTTPClient,
		clientutil.WithLogging(),
	)

	// panic if someone tries to use the default client/transport
	http.DefaultClient.Transport = panicTransport
	http.DefaultTransport = panicTransport
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtag":    func() int { main(); return 0 },
		"tag":      func() int { mainTag(); return 0 },
		"find":     func() int { mainFind(); return 0 },
		"touch":    func() int { mainTouch(); return 0 },
		"mime":     func() int { mainMIME(); return 0 },
		"mod-time": func() int { mainModTime(); return 0 },
		"rand":     func() int { mainRand(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
		Condition: func(cond string) (bool, error) {
			switch cond {
			case "ci":
				v, _ := strconv.ParseBool(os.Getenv("CI"))
				return v, nil
			}
			return false, fmt.Errorf("unknown cond")
		},
	})
}

func mainTag() {
	flag.Parse()

	op := flag.Arg(0)
	switch op {
	case "write", "check":
	default:
		log.Fatalf("bad op %s", op)
	}

	pat := flag.Arg(1)
	paths := parsePattern(pat)
	if len(paths) == 0 {
		log.Fatalf("no paths to match pattern")
	}

	pairs := parseTagLine(flag.Args()[2:])

	var exit int
	for _, p := range paths {
		switch op {
		case "write":
			if err := ensureFlac(p); err != nil {
				log.Fatalf("ensure flac: %v", err)
			}
		}

		f, err := tags.Read(p)
		if err != nil {
			log.Fatalf("open tag file: %v", err)
		}

		for t, vs := range pairs {
			switch op {
			case "write":
				f.Write(t, vs...)
			case "check":
				if got := f.ReadMulti(t); !slices.Equal(vs, got) {
					log.Printf("%s exp %q got %q", p, vs, got)
					exit = 1
				}
			}
		}

		if err := f.Save(); err != nil {
			log.Fatalf("write tag file: %v", err)
		}
		f.Close()
	}

	os.Exit(exit)
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

	pat := flag.Arg(0)
	paths := parsePattern(pat)
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

func mainRand() {
	flag.Parse()

	path, sizeStr := flag.Arg(0), flag.Arg(1)
	if path == "" || sizeStr == "" {
		log.Fatalf("bad args")
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("error creating: %v", err)
	}
	defer f.Close()

	size, _ := strconv.Atoi(sizeStr)
	_, _ = io.Copy(f, io.LimitReader(rand.Reader, int64(size)))
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

func parseTagLine(vs []string) map[string][]string {
	r := make(map[string][]string)
	var k string
	for _, v := range vs {
		if v == "/" {
			k = ""
			continue
		}
		if k == "" {
			k = v
			continue
		}
		r[k] = append(r[k], v)
	}
	return r
}
