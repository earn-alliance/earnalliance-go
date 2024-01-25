package earnalliance

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type (
	// Client is the gateway to the Earn Alliance API.
	// It is concurrency safe after CreateClient is called.
	// It contains an event queue which sends batchSize (can be set via options)
	// events to the API. This queue is LIFO.
	Client struct {
		// Initialization args
		batchSize     int
		gameID        string
		clientID      string
		clientSecret  string
		dsn           string
		errorChan     chan error
		httpClient    *http.Client
		flushInterval time.Duration
		flushCooldown time.Duration

		// Runtime fields
		flushLock        sync.Mutex
		lastFlush        time.Time
		flushWaiting     *time.Timer
		stopBatchHandler chan struct{}

		queueLock       sync.Mutex
		eventQueue      []event
		identifierQueue []identifier
	}

	// Round is a nice way of grouping some events together.
	// It sets the GroupID of the events submitted to it.
	// Its ID is a random UUID.
	// You can also create the Round with some traits
	// and these traits will be copied to the events.
	Round struct {
		id     string
		traits Traits
		c      *Client
	}

	event struct {
		UserID string `json:"userId"`
		// ISO format timestamp
		Time    string `json:"time"`
		Event   string `json:"event"`
		GroupID string `json:"groupId"`
		Traits  Traits `json:"traits,omitempty"`
		Value   *int   `json:"value,omitempty"`
	}

	identifier struct {
		UserID string `json:"userId"`
		Identifiers
	}

	// Identifiers contains the current identifiers supported by Earn Alliance.
	// The pointers provide an easy way of omitting values. Because iIf the value
	// is set to an empty string, that identifier will be removed from the user.
	// We provide the generic helper function PointerFrom() to get the
	// pointer to any value.
	Identifiers struct {
		AppleID       *string `json:"appleId,omitempty"`
		DiscordID     *string `json:"discordId,omitempty"`
		Email         *string `json:"email,omitempty"`
		EpicGamesID   *string `json:"epicGamesId,omitempty"`
		SteamID       *string `json:"steamId,omitempty"`
		TwitterIdD    *string `json:"twitterId,omitempty"`
		WalletAddress *string `json:"walletAddress,omitempty"`
	}

	// Traits is a JSON object.
	Traits map[string]any
)

const (
	defaultMaxRetryAttempts = 5
	defaultBatchSize        = 100
	defaultFlushInterval    = 30 * time.Second
	defaultFlushCooldown    = 10 * time.Second
	defaultDSN              = "https://events.earnalliance.com/v2/custom-events"

	startGameEvent = "START_GAME"
)

// Flush flushes the event queue.
// We can only rely on this returning an error or not in case #1 below.
// Other cases are asynchronous and won't return an error.
// Three cases can happen:
// 1. The cooldown period has passed:
// - Then we simply send the events to the API.
// - This case will return an error if the HTTP request fails after max retries.
// 2. The cooldown period is active & Flush hasn't been called during this period yet:
// - Then we start a goroutine that will send the events to the API once cooldown is over.
// - Basically this goroutine is async and the function will return nil immediately.
// 3. The cooldown is active & Flush has been called during this period:
// - Then this simply returns nil and the events will be sent by the goroutine
// that was created in case #2.
func (c *Client) Flush() error {
	c.flushLock.Lock()
	if time.Since(c.lastFlush) >= c.flushCooldown {
		c.lastFlush = time.Now()
		c.flushLock.Unlock()
		return c.process()
	}

	// If there is already a goroutine waiting to flush
	if c.flushWaiting != nil {
		c.flushLock.Unlock()
		return nil
	} else {
		// Create a goroutine that will flush when the cooldown is done
		leftover := c.flushCooldown - time.Since(c.lastFlush)
		c.flushWaiting = time.AfterFunc(leftover, func() {
			c.flushLock.Lock()
			c.lastFlush = time.Now()
			c.flushWaiting = nil
			c.flushLock.Unlock()
			c.doProcess()
		})
		c.flushLock.Unlock()
		return nil
	}
}

// Track submits an event to the event queue. If the event queue
// hits the batch size limit, then Flush will be called.
func (c *Client) Track(userID string, eventName string, value *int, traits Traits) {
	c.appendEvent(&event{
		Value:  value,
		UserID: userID,
		Traits: traits,
		Event:  eventName,
		Time:   time.Now().Format(time.RFC3339),
	})
}

// StartGame submits an event with the name "START_GAME" and without any traits or value
// to the event queue. If the event queue hits the batch size limit, then Flush will be called.
func (c *Client) StartGame(userID string) {
	c.appendEvent(&event{
		UserID: userID,
		Event:  startGameEvent,
		Time:   time.Now().Format(time.RFC3339),
	})
}

// StartRound creates a new Round with the given traits. These traits
// can be overwritten for specific events via passing in traits with the
// same keys when submitting an event.
// If id is an empty string we will generate a fresh UUID, and the events
// sent to this round will have their GroupID set to this UUID.
func (c *Client) StartRound(id string, traits Traits) *Round {
	if id == "" {
		id = uuid.NewString()
	}

	return &Round{
		c:      c,
		id:     id,
		traits: traits,
	}
}

// Track submits an event to the event queue with its GroupID set to the round's ID.
// The traits are combined with the round's traits. The traits passed to this function
// will overwrite the round's traits for this event when they are combined.
// If the event queue hits the batch size limit, then Flush will be called.
func (r *Round) Track(userID string, eventName string, value *int, traits Traits) {
	r.c.appendEvent(&event{
		GroupID: r.id,
		Value:   value,
		UserID:  userID,
		Event:   eventName,
		Traits:  combineTraits(r.traits, traits),
		Time:    time.Now().Format(time.RFC3339),
	})
}

// SetIdentifiers submits an identifier to the event queue.
// This will call Flush no matter what, but whether it will be sent immediately
// depends on if the cooldown period is active.
// However if the event queue hits the batch size limit,
// the events will be sent to the API.
func (c *Client) SetIdentifiers(userID string, is *Identifiers) {
	if is == nil {
		is = &Identifiers{}
	}

	c.appendIdentifier(&identifier{
		Identifiers: *is,
		UserID:      userID,
	})
	if err := c.Flush(); err != nil && c.errorChan != nil {
		c.errorChan <- err
	}
}

// Close closes the open goroutines. It is not concurrency safe.
func (c *Client) Close() {
	c.queueLock.Lock()
	if c.flushWaiting != nil {
		c.flushWaiting.Stop()
	}
	c.queueLock.Unlock()
	c.stopBatchHandler <- struct{}{}
}

func (c *Client) appendEvent(e *event) {
	c.queueLock.Lock()
	c.eventQueue = append(c.eventQueue, *e)
	queueSize := c.queueSize()
	c.queueLock.Unlock()

	if queueSize >= c.batchSize {
		c.doProcess()
	}
}

func (c *Client) appendIdentifier(i *identifier) {
	c.queueLock.Lock()
	c.identifierQueue = append(c.identifierQueue, *i)
	queueSize := c.queueSize()
	c.queueLock.Unlock()

	if queueSize >= c.batchSize {
		c.doProcess()
	}
}

func (c *Client) queueSize() int {
	return len(c.eventQueue) + len(c.identifierQueue)
}

func (c *Client) sign(msg []byte, timestamp string) (string, error) {
	h := hmac.New(sha256.New, []byte(c.clientSecret))

	body := fmt.Sprintf("%s%s%s", c.clientID, timestamp, msg)

	if _, err := h.Write([]byte(body)); err != nil {
		return "", fmt.Errorf("failed to write hmac body: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Client) handleBatch() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopBatchHandler:
			return
		case <-ticker.C:
			if err := c.Flush(); err != nil && c.errorChan != nil {
				c.errorChan <- err
			}
		}
	}
}

func (c *Client) doProcess() {
	if err := c.process(); err != nil && c.errorChan != nil {
		c.errorChan <- err
	}
}

func (c *Client) process() error {
	c.queueLock.Lock()

	events := make([]event, 0, c.batchSize)
	identifiers := make([]identifier, 0, c.batchSize)

	i := 0
	for ; i < c.batchSize && i < len(c.identifierQueue); i++ {
		identifiers = append(identifiers, c.identifierQueue[i])
	}
	if i < len(c.identifierQueue) {
		c.identifierQueue = c.identifierQueue[i:]
	} else {
		c.identifierQueue = c.identifierQueue[:0]
	}

	j := 0
	for ; i < c.batchSize && j < len(c.eventQueue); j++ {
		events = append(events, c.eventQueue[j])
		i++
	}
	if j < len(c.eventQueue) {
		c.eventQueue = c.eventQueue[j:]
	} else {
		c.eventQueue = c.eventQueue[:0]
	}

	c.queueLock.Unlock()

	// Skip processing if queue empty
	if len(events) == 0 && len(identifiers) == 0 {
		return nil
	}

	payload := map[string]any{
		"gameId":      c.gameID,
		"events":      events,
		"identifiers": identifiers,
	}

	m, err := json.Marshal(&payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	return c.send(m)
}

func (c *Client) send(msg []byte) error {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	signature, err := c.sign(msg, timestamp)
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	req, err := http.NewRequest("POST", c.dsn, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("x-timestamp", timestamp)
	req.Header.Set("x-signature", signature)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to do request: %w", err)
	}
	defer res.Body.Close()

	var m map[string]any
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return fmt.Errorf("failed to decode response body: %w", err)
	}

	if b, ok := m["message"]; ok {
		if s, ok := b.(string); ok {
			if s == "OK" {
				return nil
			}
		}
	}

	if b, ok := m["error"]; ok {
		if s, ok := b.(string); ok {
			return fmt.Errorf("server returned error: %s", s)
		}
	}

	return fmt.Errorf("unexpected response from server: %v", m)
}

func PointerFrom[T any](v T) *T {
	return &v
}

func combineTraits(a, b Traits) Traits {
	n := make(Traits, len(a)+len(b))
	for k := range a {
		n[k] = a[k]
	}
	for k := range b {
		n[k] = b[k]
	}
	return n
}
