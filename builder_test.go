package earnalliance_test

import (
	"os"
	"strings"
	"testing"

	ea "github.com/earn-alliance/earnalliance-go"
)

func TestBuildClient(t *testing.T) {
	t.Run("missing client ID", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "client id cannot be empty") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().WithClientID("")
	})

	t.Run("missing client secret", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "client secret cannot be empty") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().WithClientSecret("")
	})

	t.Run("missing game ID", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "game id cannot be empty") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().WithGameID("")
	})

	t.Run("missing dsn", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "dsn cannot be empty") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().WithDSN("")
	})

	t.Run("false dsn", func(t *testing.T) {
		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "failed to parse dsn") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().WithDSN("asdasd")
	})

	t.Run("missing any", func(t *testing.T) {
		clientID := os.Getenv("ALLIANCE_CLIENT_ID")
		clientSecret := os.Getenv("ALLIANCE_CLIENT_SECRET")
		gameID := os.Getenv("ALLIANCE_GAME_ID")

		if clientID != "" && clientSecret != "" && gameID != "" {
			t.SkipNow()
		}

		defer func() {
			err := recover()
			if err == nil {
				t.Fatal("panic was expected")
			}
			if !strings.Contains(err.(string), "missing required client options") {
				t.Fatal("unexpected panic", err.(string))
			}
		}()

		ea.NewClientBuilder().Build()
	})
}
