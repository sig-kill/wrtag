package flagcommon

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"go.senan.xyz/wrtag/notifications"
	"go.senan.xyz/wrtag/pathformat"
	"go.senan.xyz/wrtag/researchlink"
	"go.senan.xyz/wrtag/tagmap"
)

var _ flag.Value = pathFormatParser{}
var _ flag.Value = querierParser{}
var _ flag.Value = notificationsParser{}
var _ flag.Value = tagWeightsParser{}

type pathFormatParser struct{ *pathformat.Format }

func (pf pathFormatParser) String() string         { return "" }
func (pf pathFormatParser) Set(value string) error { return pf.Parse(value) }

type querierParser struct{ *researchlink.Querier }

func (p querierParser) String() string { return "" }
func (q querierParser) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	err := q.AddSource(name, value)
	return err
}

type notificationsParser struct{ *notifications.Notifications }

func (n notificationsParser) String() string { return "" }
func (n notificationsParser) Set(value string) error {
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

type tagWeightsParser struct{ *tagmap.TagWeights }

func (tw tagWeightsParser) String() string { return "" }
func (tw tagWeightsParser) Set(value string) error {
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
