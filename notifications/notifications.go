package notifications

import (
	"errors"
	"fmt"
	"log"

	"github.com/containrrr/shoutrrr"
	shoutrrrtypes "github.com/containrrr/shoutrrr/pkg/types"
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
	uris := n.mappings[event]
	if len(uris) == 0 {
		return
	}

	sender, err := shoutrrr.CreateSender(uris...)
	if err != nil {
		log.Printf("create sender: %v", err)
		return
	}

	params := &shoutrrrtypes.Params{}
	params.SetTitle("wrtag")

	if err := errors.Join(sender.Send(message, params)...); err != nil {
		log.Printf("error sending notifications: %v", err)
	}
}
