package notifications

import (
	"errors"
	"fmt"
	"log"

	"github.com/containrrr/shoutrrr"
)

var ErrUnknownEvent = errors.New("unknown event")

type Event string

const (
	Complete   Event = "complete"
	NeedsInput Event = "needs-input"
)

type Notifications struct {
	mappings map[Event][]string
}

func (n *Notifications) AddURI(event Event, uri string) error {
	if n.mappings == nil {
		n.mappings = map[Event][]string{}
	}
	switch event {
	case Complete, NeedsInput:
		n.mappings[event] = append(n.mappings[event], uri)
		return nil
	}
	return fmt.Errorf("%w: %q", ErrUnknownEvent, event)
}

// Send a simple string for now, later fancy diffs and things
func (n *Notifications) Send(event Event, message string) {
	for _, uri := range n.mappings[event] {
		log.Printf("sending %s to %s", event, uri)
		go func() {
			if err := shoutrrr.Send(uri, message); err != nil {
				log.Printf("error sending message for %s: %v", event, err)
			}
		}()
	}
}
