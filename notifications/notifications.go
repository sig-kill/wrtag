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
	Complete     Event = "complete"
	NeedsInput   Event = "needs-input"
	SyncComplete Event = "sync-complete"
	SyncError    Event = "sync-error"
)

func (e Event) IsValid() bool {
	switch e {
	case Complete, NeedsInput, SyncComplete, SyncError:
		return true
	}
	return false
}

type Notifications struct {
	mappings map[Event][]string
}

func (n *Notifications) AddURI(event Event, uri string) error {
	if n.mappings == nil {
		n.mappings = map[Event][]string{}
	}
	if !event.IsValid() {
		return fmt.Errorf("%w: %q", ErrUnknownEvent, event)
	}
	n.mappings[event] = append(n.mappings[event], uri)
	return nil
}

// Send a simple string for now, maybe later message could instead be be a type which
// implements a notifications.Bodyer or something so that notifiers can send rich notifications.
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
