package server

//
import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	eventv1 "sg-emulator/gen/event/v1"
)

const (
	// DefaultSubscriberBufferSize is the channel buffer size per subscriber.
	DefaultSubscriberBufferSize = 256

	// DefaultIngestBufferSize is the buffer size for the EventBus ingest channel.
	DefaultIngestBufferSize = 10240
)

// EventFilter determines whether an event should be delivered to a subscriber.
type EventFilter struct {
	// EventTypes to receive. Empty means all types.
	EventTypes map[eventv1.EventType]struct{}
	// AccountIDs to filter on. Empty means all accounts.
	// An event matches if any involved account is in this set.
	AccountIDs map[string]struct{}
}

// Matches returns true if the event passes this filter.
func (f *EventFilter) Matches(event *eventv1.Event) bool {
	// Check event type filter
	if len(f.EventTypes) > 0 {
		if _, ok := f.EventTypes[event.GetType()]; !ok {
			return false
		}
	}

	// Check account ID filter
	if len(f.AccountIDs) > 0 {
		involved := eventInvolvedAccounts(event)
		match := false
		for _, accID := range involved {
			if _, ok := f.AccountIDs[accID]; ok {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}

	return true
}

// Subscription represents an active event subscription for a single client.
type Subscription struct {
	AccountID string
	Events    chan *eventv1.Event
	filter    *EventFilter
	done      chan struct{}
}

// Done returns a channel that is closed when the subscription is terminated.
func (s *Subscription) Done() <-chan struct{} {
	return s.done
}

// EventBus manages event subscriptions for a single VirtualApp node.
// Events are received on an ingest channel and dispatched by a single goroutine
// to matching subscribers and registered EventTransport channels.
type EventBus struct {
	ingest      chan *eventv1.Event
	subscribers map[string]*Subscription // keyed by account ID
	transports  []chan<- EventDelivery
	mu          sync.RWMutex
	logger      *slog.Logger
	stopCh      chan struct{}
	stopped     chan struct{}
	started     atomic.Bool
}

// NewEventBus creates a new EventBus with a buffered ingest channel.
func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		ingest:      make(chan *eventv1.Event, DefaultIngestBufferSize),
		subscribers: make(map[string]*Subscription),
		transports:  make([]chan<- EventDelivery, 0),
		logger:      logger,
		stopCh:      make(chan struct{}),
		stopped:     make(chan struct{}),
	}
}

// RegisterTransport adds a transport's event delivery channel.
// Must be called before Run.
func (eb *EventBus) RegisterTransport(ch chan<- EventDelivery) {
	eb.transports = append(eb.transports, ch)
}

// Send places an event onto the ingest channel for asynchronous dispatch.
// Non-blocking: if the ingest buffer is full the event is dropped and logged.
func (eb *EventBus) Send(event *eventv1.Event) {
	select {
	case eb.ingest <- event:
	default:
		eb.logger.Warn("EventBus ingest buffer full, dropping event",
			"event_type", event.GetType(),
			"buffer_size", DefaultIngestBufferSize,
		)
	}
}

// Run starts the dispatcher goroutine. It reads events from the ingest channel,
// matches them against subscriber filters, and delivers EventDelivery messages
// to registered transport channels. Blocks until ctx is cancelled or Stop is called.
func (eb *EventBus) Run(ctx context.Context) {
	eb.started.Store(true)
	defer close(eb.stopped)

	for {
		select {
		case event, ok := <-eb.ingest:
			if !ok {
				return
			}
			eb.dispatch(event)
		case <-ctx.Done():
			// Drain remaining events before exiting
			for {
				select {
				case event, ok := <-eb.ingest:
					if !ok {
						return
					}
					eb.dispatch(event)
				default:
					return
				}
			}
		case <-eb.stopCh:
			return
		}
	}
}

// dispatch sends a single event to all matching subscribers via their
// transport's event delivery channel.
func (eb *EventBus) dispatch(event *eventv1.Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for accountID, sub := range eb.subscribers {
		if !sub.filter.Matches(event) {
			continue
		}

		delivery := EventDelivery{
			Event:     event,
			AccountID: accountID,
		}

		// Deliver to all registered transports
		for _, ch := range eb.transports {
			select {
			case ch <- delivery:
			default:
				eb.logger.Warn("Transport event channel full, dropping delivery",
					"account_id", accountID,
					"event_type", event.GetType(),
				)
			}
		}
	}
}

// Stop signals the dispatcher to stop and waits for it to finish.
// Safe to call even if Run was never started.
func (eb *EventBus) Stop() {
	select {
	case <-eb.stopCh:
		// already stopped
	default:
		close(eb.stopCh)
	}
	if eb.started.Load() {
		<-eb.stopped
	}
}

// Subscribe registers a new subscription for the given account ID.
// Returns an error if the account already has an active subscription on this node.
func (eb *EventBus) Subscribe(accountID string, filter *EventFilter) (*Subscription, error) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if _, exists := eb.subscribers[accountID]; exists {
		return nil, fmt.Errorf("account %s already has an active subscription on this node", accountID)
	}

	sub := &Subscription{
		AccountID: accountID,
		Events:    make(chan *eventv1.Event, DefaultSubscriberBufferSize),
		filter:    filter,
		done:      make(chan struct{}),
	}

	eb.subscribers[accountID] = sub
	eb.logger.Info("Subscriber added", "account_id", accountID)
	return sub, nil
}

// Unsubscribe removes the subscription for the given account ID.
func (eb *EventBus) Unsubscribe(accountID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if sub, exists := eb.subscribers[accountID]; exists {
		close(sub.done)
		delete(eb.subscribers, accountID)
		eb.logger.Info("Subscriber removed", "account_id", accountID)
	}
}

// SubscriberCount returns the number of active subscribers.
func (eb *EventBus) SubscriberCount() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.subscribers)
}

// eventInvolvedAccounts extracts all account IDs involved in an event.
func eventInvolvedAccounts(event *eventv1.Event) []string {
	switch d := event.GetData().(type) {
	case *eventv1.Event_Transfer:
		return []string{d.Transfer.GetFrom(), d.Transfer.GetTo()}
	case *eventv1.Event_Mint:
		return []string{d.Mint.GetTo()}
	case *eventv1.Event_MintToken:
		accs := []string{d.MintToken.GetAccountId()}
		if cb := d.MintToken.GetClawbackAddress(); cb != "" {
			accs = append(accs, cb)
		}
		return accs
	case *eventv1.Event_TransferToken:
		return []string{d.TransferToken.GetFrom(), d.TransferToken.GetTo()}
	case *eventv1.Event_AuthorizeTokenTransfer:
		return []string{d.AuthorizeTokenTransfer.GetAccountId()}
	case *eventv1.Event_UnauthorizeTokenTransfer:
		return []string{d.UnauthorizeTokenTransfer.GetAccountId()}
	case *eventv1.Event_BurnToken:
		return []string{d.BurnToken.GetAccountId()}
	case *eventv1.Event_ClawbackToken:
		return []string{d.ClawbackToken.GetFrom(), d.ClawbackToken.GetTo()}
	default:
		return nil
	}
}

// BuildEvent constructs a proto Event from a request payload that was successfully processed.
// Returns nil if the request type does not map to an event.
func BuildEvent(requestPayload any) *eventv1.Event {
	now := time.Now().Unix()

	switch req := requestPayload.(type) {
	case *transferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_Transfer{
				Transfer: &eventv1.TransferEventData{
					From:   req.From,
					To:     req.To,
					Amount: float64(req.Amount),
				},
			},
		}
	case *mintEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_MINT,
			Timestamp: now,
			Data: &eventv1.Event_Mint{
				Mint: &eventv1.MintEventData{
					To:     req.To,
					Amount: float64(req.Amount),
				},
			},
		}
	case *mintTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_MINT_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_MintToken{
				MintToken: &eventv1.MintTokenEventData{
					AccountId:       req.AccountID,
					TokenId:         req.TokenID,
					TokenValue:      req.TokenValue,
					ClawbackAddress: req.ClawbackAddress,
				},
			},
		}
	case *transferTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_TRANSFER_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_TransferToken{
				TransferToken: &eventv1.TransferTokenEventData{
					From:    req.From,
					To:      req.To,
					TokenId: req.TokenID,
				},
			},
		}
	case *authorizeTokenTransferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_AUTHORIZE_TOKEN_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_AuthorizeTokenTransfer{
				AuthorizeTokenTransfer: &eventv1.AuthorizeTokenTransferEventData{
					AccountId: req.AccountID,
					TokenId:   req.TokenID,
				},
			},
		}
	case *unauthorizeTokenTransferEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_UNAUTHORIZE_TOKEN_TRANSFER,
			Timestamp: now,
			Data: &eventv1.Event_UnauthorizeTokenTransfer{
				UnauthorizeTokenTransfer: &eventv1.UnauthorizeTokenTransferEventData{
					AccountId: req.AccountID,
					TokenId:   req.TokenID,
				},
			},
		}
	case *burnTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_BURN_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_BurnToken{
				BurnToken: &eventv1.BurnTokenEventData{
					AccountId: req.AccountID,
					TokenId:   req.TokenID,
				},
			},
		}
	case *clawbackTokenEventInfo:
		return &eventv1.Event{
			Type:      eventv1.EventType_EVENT_TYPE_CLAWBACK_TOKEN,
			Timestamp: now,
			Data: &eventv1.Event_ClawbackToken{
				ClawbackToken: &eventv1.ClawbackTokenEventData{
					From:    req.From,
					To:      req.To,
					TokenId: req.TokenID,
				},
			},
		}
	default:
		return nil
	}
}

// Event info types used to construct events from domain request payloads.

type transferEventInfo struct {
	From   string
	To     string
	Amount float64
}

type mintEventInfo struct {
	To     string
	Amount float64
}

type mintTokenEventInfo struct {
	AccountID       string
	TokenID         string
	TokenValue      string
	ClawbackAddress string
}

type transferTokenEventInfo struct {
	From    string
	To      string
	TokenID string
}

type authorizeTokenTransferEventInfo struct {
	AccountID string
	TokenID   string
}

type unauthorizeTokenTransferEventInfo struct {
	AccountID string
	TokenID   string
}

type burnTokenEventInfo struct {
	AccountID string
	TokenID   string
}

type clawbackTokenEventInfo struct {
	From    string
	To      string
	TokenID string
}
