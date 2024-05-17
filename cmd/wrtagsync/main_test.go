package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/clientutil"
)

func init() {
	mb.MBClient.RateLimit = 0
	mb.CAAClient.RateLimit = 0

	// panic if someone tries to use the default client/transport
	http.DefaultClient.Transport = panicTransport
	http.DefaultTransport = panicTransport
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtagsync": func() int { main(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

var panicTransport = clientutil.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
	panic("panic transport used")
})
