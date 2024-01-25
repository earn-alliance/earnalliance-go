package earnalliance

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// ClientBuilder builds a new client. It is not concurrency safe.
// It will panic if required options are missing, or if any
// invalid options are passed in.
type ClientBuilder struct {
	c *Client
}

func createRetryableClient(maxAttempts int) *http.Client {
	rc := retryablehttp.NewClient()
	rc.Logger = nil
	rc.RetryMax = maxAttempts
	return rc.StandardClient()
}

// NewClientBuilder creates a new ClientBuilder. It also sets the default values
// in the underlying client. And looks for the environment variables:
// ALLIANCE_CLIENT_ID, ALLIANCE_CLIENT_SECRET, ALLIANCE_GAME_ID and ALLIANCE_DSN.
func NewClientBuilder() *ClientBuilder {
	clientID := strings.TrimSpace(os.Getenv("ALLIANCE_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("ALLIANCE_CLIENT_SECRET"))
	gameID := strings.TrimSpace(os.Getenv("ALLIANCE_GAME_ID"))
	dsn := strings.TrimSpace(os.Getenv("ALLIANCE_DSN"))
	if dsn == "" {
		dsn = defaultDSN
	}

	cb := &ClientBuilder{
		c: &Client{
			dsn:              dsn,
			gameID:           gameID,
			clientID:         clientID,
			clientSecret:     clientSecret,
			batchSize:        defaultBatchSize,
			stopBatchHandler: make(chan struct{}),
			flushInterval:    defaultFlushInterval,
			flushCooldown:    defaultFlushCooldown,
			httpClient:       createRetryableClient(defaultMaxRetryAttempts),
		},
	}

	return cb
}

// WithMaxRetryAttempts sets the maximum number of retries before
// an HTTP request is considered as failed and the library returns an error.
// Default: 5
// This is optional.
func (cb *ClientBuilder) WithMaxRetryAttempts(maxAttempts int) *ClientBuilder {
	if maxAttempts < 1 {
		panic("max retry attempts must be at least 1")
	}

	cb.c.httpClient = createRetryableClient(maxAttempts)
	return cb
}

// WithFlushInterval sets the flush interval which is the time between
// flushes without the event queue hitting the batch size.
// Default: 30 seconds
// This is optional.
func (cb *ClientBuilder) WithFlushInterval(interval time.Duration) *ClientBuilder {
	if interval < 0 {
		panic("flush interval must be at least 0")
	}

	cb.c.flushInterval = interval
	return cb
}

// WithFlushCooldown sets the flush cooldown which is the minimum
// required time between Flush() calls. During this cooldown period,
// a Flush() call will start a timer in a goroutine that will
// flush afterwards.
// Default: 10 seconds
// This is optional.
func (cb *ClientBuilder) WithFlushCooldown(cooldown time.Duration) *ClientBuilder {
	if cooldown < 0 {
		panic("flush cooldown must be at least 0")
	}

	cb.c.flushCooldown = cooldown
	return cb
}

// WithBatchSize sets the batch size which is the maximum size
// of the event queue where it will be flushed automatically if reached.
// Default: 100
// This is optional.
func (cb *ClientBuilder) WithBatchSize(batchSize int) *ClientBuilder {
	if batchSize < 1 {
		panic("batch size must be at least 1")
	}

	cb.c.batchSize = batchSize
	return cb
}

// WithErrorChannel sets the error channel where the asynchronous Flush calls
// will send their errors to. Multiple errors may be sent at once.
// Default: N/A
// This is optional.
func (cb *ClientBuilder) WithErrorChannel(ch chan error) *ClientBuilder {
	cb.c.errorChan = ch
	return cb
}

// WithDSN sets the DSN (the URL that requests are sent to) for the Earn Alliance API.
// Default: https://events.earnalliance.com/v2/custom-events
// This is optional.
func (cb *ClientBuilder) WithDSN(dsn string) *ClientBuilder {
	if dsn == "" {
		panic("dsn cannot be empty")
	}

	dsn = strings.TrimSuffix(dsn, "/")

	if _, err := url.ParseRequestURI(dsn); err != nil {
		panic("failed to parse dsn: " + err.Error())
	}

	cb.c.dsn = dsn
	return cb
}

// WithClientID sets the Earn Alliance client ID.
// Default: N/A
// This is required.
func (cb *ClientBuilder) WithClientID(clientID string) *ClientBuilder {
	if clientID == "" {
		panic("client id cannot be empty")
	}

	cb.c.clientID = clientID
	return cb
}

// WithClientSecret sets the Earn Alliance client secret.
// Default: N/A
// This is required.
func (cb *ClientBuilder) WithClientSecret(clientSecret string) *ClientBuilder {
	if clientSecret == "" {
		panic("client secret cannot be empty")
	}

	cb.c.clientSecret = clientSecret
	return cb
}

// WithGameID sets the Earn Alliance game ID.
// Default: N/A
// This is required.
func (cb *ClientBuilder) WithGameID(gameID string) *ClientBuilder {
	if gameID == "" {
		panic("game id cannot be empty")
	}

	cb.c.gameID = gameID
	return cb
}

// Build returns the client that was created via the builder and starts
// the internal batch processing goroutine.
// Ensure that you have the ClientID, ClientSecret and GameID set before
// you call this.
func (cb *ClientBuilder) Build() *Client {
	c := cb.c

	if c.clientID == "" || c.clientSecret == "" || c.gameID == "" || c.dsn == "" {
		panic("missing required client options")
	}

	go c.handleBatch()

	return c
}
