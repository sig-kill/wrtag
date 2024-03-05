package conf

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.senan.xyz/flagconf"
)

const name = "wrtag"

var (
	PathFormat       string
	ResearchLinks    ResearchLinkConf
	NotificationURIs = make(NotificationURIsConf)
)

func Parse() {
	flag.CommandLine.Init(name, flag.ExitOnError)
	flag.StringVar(&PathFormat, "path-format", "", "path format")
	flag.Var(&ResearchLinks, "research-link", "research link")
	flag.Var(NotificationURIs, "notification-uri", "shoutrrr notification uri")

	configPath := flag.String("config-path", defaultConfigPath, "path config file")

	flag.Parse()
	flagconf.ParseEnv()
	flagconf.ParseConfig(*configPath)
}

var (
	userConfig, _     = os.UserConfigDir()
	defaultConfigPath = filepath.Join(userConfig, name, "config")
)

type ResearchLinkConf []ResearchLinkTemplate
type ResearchLinkTemplate struct {
	Name     string
	Template string
}

func (sls ResearchLinkConf) String() string {
	var names []string
	for _, sl := range sls {
		names = append(names, sl.Name)
	}
	return strings.Join(names, ", ")
}

func (sls *ResearchLinkConf) Set(value string) error {
	name, value, _ := strings.Cut(value, " ")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	*sls = append(*sls, ResearchLinkTemplate{Name: name, Template: value})
	return nil
}

type Event string

const (
	EventComplete   Event = "complete"
	EventNeedsInput Event = "needs-input"
)

type NotificationURIsConf map[Event][]string

func (nur NotificationURIsConf) String() string {
	var buff strings.Builder
	for k, v := range nur {
		fmt.Fprintf(&buff, "\n%s -> %s", k, strings.Join(v, ", "))
	}
	return buff.String()

}

func (nur NotificationURIsConf) Set(value string) error {
	eventsRaw, uri, ok := strings.Cut(value, " ")
	if !ok {
		return fmt.Errorf("invalid notification uri format. expected \"ev1,ev2 uri\"")
	}
	for _, ev := range strings.Split(eventsRaw, ",") {
		ev, uri = strings.TrimSpace(ev), strings.TrimSpace(uri)
		switch ev := Event(ev); ev {
		case EventComplete, EventNeedsInput:
			nur[ev] = append(nur[ev], uri)
		default:
			return fmt.Errorf("unknown event type %q", ev)
		}
	}
	return nil
}
