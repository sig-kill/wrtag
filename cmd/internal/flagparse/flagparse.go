package flagparse

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

const name = "wrtag"

func init() {
	flag.CommandLine.Init(name, flag.ExitOnError)
}

var (
	userConfig, _     = os.UserConfigDir()
	DefaultConfigPath = filepath.Join(userConfig, name, "config")
)

type PathFormat struct{ *pathformat.Format }

func (pf PathFormat) String() string         { return "" }
func (pf PathFormat) Set(value string) error { return pf.Parse(value) }

var _ flag.Value = PathFormat{}

type Querier struct{ *researchlink.Querier }

func (p Querier) String() string { return "" }
func (q Querier) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	err := q.AddSource(name, value)
	return err
}

var _ flag.Value = Querier{}

type Notifications struct{ *notifications.Notifications }

func (n Notifications) String() string { return "" }
func (n Notifications) Set(value string) error {
	eventsRaw, uri, ok := strings.Cut(value, " ")
	if !ok {
		return fmt.Errorf("invalid notification uri format. expected eg \"ev1,ev2 uri\"")
	}
	var lineErrs []error
	for _, ev := range strings.Split(eventsRaw, ",") {
		ev, uri = strings.TrimSpace(ev), strings.TrimSpace(uri)
		err := n.AddURI(notifications.Event(ev), uri)
		lineErrs = append(lineErrs, err)
	}
	return errors.Join(lineErrs...)
}

var _ flag.Value = Notifications{}

type TagWeights struct{ *tagmap.TagWeights }

func (tw TagWeights) String() string { return "" }
func (tw TagWeights) Set(value string) error {
	const sep = " "
	i := strings.LastIndex(value, sep)
	if i < 0 {
		return fmt.Errorf("invalid tag weight format. expected eg \"tag name 0.5\"")
	}
	tag := strings.TrimSpace(value[:i])
	weightStr := strings.TrimSpace(value[i+len(sep):])
	weight, err := strconv.ParseFloat(weightStr, 64)
	if err != nil {
		return fmt.Errorf("parse weight: %w", err)
	}
	if *tw.TagWeights == nil {
		*tw.TagWeights = tagmap.TagWeights{}
	}
	(*tw.TagWeights)[tag] = weight
	return nil
}

var _ flag.Value = TagWeights{}
