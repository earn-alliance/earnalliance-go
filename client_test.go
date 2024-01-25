package earnalliance

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlush(t *testing.T) {
	t.Run("single flush call, will skip processing due to queue being empty", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		err := client.Flush()
		require.Nil(t, err)
	})

	t.Run("back to back flush calls with one start game, one will fail one will start waiter", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.StartGame("asd")

		err := client.Flush()
		require.NotNil(t, err)
		err = client.Flush()
		require.Nil(t, err)
		require.NotNil(t, client.flushWaiting)
		time.Sleep(5 * time.Second)
		require.Nil(t, client.flushWaiting)
	})

	t.Run("3 back to back flush calls with one start game, one will fail one will start waiter one will do nothing", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.StartGame("asd")

		err := client.Flush()
		require.NotNil(t, err)

		err = client.Flush()
		require.Nil(t, err)
		err = client.Flush()
		require.Nil(t, err)

		require.NotNil(t, client.flushWaiting)
		time.Sleep(5 * time.Second)
		require.Nil(t, client.flushWaiting)
	})

	t.Run("single flush call with one event", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.Track("asd", "DEATH", nil, nil)

		err := client.Flush()
		require.NotNil(t, err)
	})

	t.Run("flush call, then insert identifier so that another flush is called", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.Track("asd", "DEATH", nil, nil)

		err := client.Flush()
		require.NotNil(t, err)
		client.SetIdentifiers("asd", &Identifiers{
			WalletAddress: IdentifierFrom("yoyo"),
		})
		require.NotNil(t, client.flushWaiting)
		time.Sleep(5 * time.Second)
		require.Nil(t, client.flushWaiting)
	})
}

func TestTrack(t *testing.T) {
	t.Run("single track", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.Track("asd", "kill", PointerFrom(1), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
	})

	t.Run("single start game", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.StartGame("asd")

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, startGameEvent)
		require.Nil(t, e.Value)
		require.Nil(t, e.Traits)
	})

	t.Run("multiple tracks", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		client.Track("asd", "kill", PointerFrom(1), nil)
		client.Track("asd2", "kill2", PointerFrom(2), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)

		e = &client.eventQueue[1]
		require.Equal(t, e.UserID, "asd2")
		require.Equal(t, e.Event, "kill2")
		require.Equal(t, *e.Value, 2)

		client.Track("asd3", "kill3", PointerFrom(3), nil)

		e = &client.eventQueue[2]
		require.Equal(t, e.UserID, "asd3")
		require.Equal(t, e.Event, "kill3")
		require.Equal(t, *e.Value, 3)
	})

	t.Run("one track with 1 batch size", func(t *testing.T) {
		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := false

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					result = true
					fmt.Println(e)
					return
				case <-tick.C:
					result = false
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			WithBatchSize(1).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		// This will error due to Flush being called because of the 1 batch size
		client.Track("asd", "kill", PointerFrom(1), nil)

		wg.Wait()

		// Event should be removed now
		require.Len(t, client.eventQueue, 0)

		if !result {
			t.Fatal("test expired without error being returned")
		}
	})
}

func TestSetIdentifiers(t *testing.T) {
	t.Run("one identity", func(t *testing.T) {
		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := false

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = true
					return
				case <-tick.C:
					result = false
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		// This will error due to Flush being called because of the 1 batch size
		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: IdentifierFrom("yope"),
		})

		wg.Wait()

		// Queue should be empty now
		require.Len(t, client.identifierQueue, 0)

		if !result {
			t.Fatal("test expired without error being returned")
		}
	})

	t.Run("two identities", func(t *testing.T) {
		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := false

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = true
					return
				case <-tick.C:
					result = false
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		// This will error due to Flush being called because of the 1 batch size
		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: IdentifierFrom("yope"),
		})

		// This will start the waiter
		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: IdentifierFrom("yope"),
		})

		require.NotNil(t, client.flushWaiting)

		wg.Wait()

		// Queue might still have the 2nd identifier, don't check length here

		if !result {
			t.Fatal("test expired without error being returned")
		}
	})

	t.Run("valid client test", func(t *testing.T) {
		clientID := os.Getenv("ALLIANCE_CLIENT_ID")
		clientSecret := os.Getenv("ALLIANCE_CLIENT_SECRET")
		gameID := os.Getenv("ALLIANCE_GAME_ID")

		if clientID == "" || clientSecret == "" || gameID == "" {
			t.SkipNow()
		}

		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := true

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = false
					return
				case <-tick.C:
					result = true
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithFlushCooldown(5 * time.Second).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: IdentifierFrom("yope"),
		})

		require.Nil(t, client.flushWaiting)

		wg.Wait()

		require.Len(t, client.identifierQueue, 0)

		if !result {
			t.Fatal("valid client test failed with error")
		}
	})
}

func TestRound(t *testing.T) {
	t.Run("single track", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		r := client.StartRound("", nil)
		require.NotEmpty(t, r.id)
		r.Track("asd", "kill", PointerFrom(1), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
		require.Equal(t, r.id, e.GroupID)
	})

	t.Run("single track with custom id", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		r := client.StartRound("custom-id", nil)
		require.Equal(t, "custom-id", r.id)
		r.Track("asd", "kill", PointerFrom(1), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
		require.Equal(t, r.id, e.GroupID)
	})

	t.Run("multiple tracks", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		r := client.StartRound("", nil)
		require.NotEmpty(t, r.id)

		r.Track("asd", "kill", PointerFrom(1), nil)
		r.Track("asd2", "kill2", PointerFrom(2), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
		require.Equal(t, r.id, e.GroupID)

		e = &client.eventQueue[1]
		require.Equal(t, e.UserID, "asd2")
		require.Equal(t, e.Event, "kill2")
		require.Equal(t, *e.Value, 2)
		require.Equal(t, r.id, e.GroupID)

		r.Track("asd3", "kill3", PointerFrom(3), nil)

		e = &client.eventQueue[2]
		require.Equal(t, e.UserID, "asd3")
		require.Equal(t, e.Event, "kill3")
		require.Equal(t, *e.Value, 3)
		require.Equal(t, r.id, e.GroupID)
	})

	t.Run("single track with traits", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		r := client.StartRound("", Traits{"map": "nuclear_wasteland"})
		require.NotEmpty(t, r.id)
		r.Track("asd", "kill", PointerFrom(1), nil)

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
		require.Equal(t, r.id, e.GroupID)
		require.Equal(t, e.Traits["map"], "nuclear_wasteland")
	})

	t.Run("single track with traits that collide", func(t *testing.T) {
		client := NewClientBuilder().
			WithClientID("a").
			WithClientSecret("b").
			WithGameID("c").
			WithFlushCooldown(5 * time.Second).
			Build()
		defer client.Close()

		r := client.StartRound("", Traits{"map": "nuclear_wasteland"})
		require.NotEmpty(t, r.id)

		r.Track("asd", "kill", PointerFrom(1), Traits{"map": "nuclear_utopia"})

		e := &client.eventQueue[0]
		require.Equal(t, e.UserID, "asd")
		require.Equal(t, e.Event, "kill")
		require.Equal(t, *e.Value, 1)
		require.Equal(t, r.id, e.GroupID)
		require.Equal(t, e.Traits["map"], "nuclear_utopia")
	})
}

func TestSign(t *testing.T) {
	client := NewClientBuilder().
		WithClientID("a").
		WithClientSecret("b").
		WithGameID("c").
		WithFlushCooldown(5 * time.Second).
		Build()
	defer client.Close()

	s, err := client.sign([]byte("hello"), "123")
	require.Nil(t, err)

	// a123hello signed with "b"
	require.Equal(t, "8713437da9757423053fb5beb3e58794321ac22055d5c8fbf0cd0c6b9f5675e5", s)

	s, err = client.sign([]byte("yoyoyoyo"), "123")
	require.Nil(t, err)

	// a123yoyoyoyo signed with "b"
	require.Equal(t, "d930ed77d3ad0682c396f8e9cb12f4d2761083a68378f6b1f4f8b93d16a0b0e7", s)

	client.clientID = "foh"
	client.clientSecret = "rah"

	s, err = client.sign([]byte("hundred"), "123")
	require.Nil(t, err)

	// foh123hundred signed with "rah"
	require.Equal(t, "8462555d220af5dff2922abb6c50dbfe36a87918361dbbdb5572bcf637185d92", s)
}

func TestCompleteValid(t *testing.T) {
	t.Run("test some tracks", func(t *testing.T) {
		clientID := os.Getenv("ALLIANCE_CLIENT_ID")
		clientSecret := os.Getenv("ALLIANCE_CLIENT_SECRET")
		gameID := os.Getenv("ALLIANCE_GAME_ID")

		if clientID == "" || clientSecret == "" || gameID == "" {
			t.SkipNow()
		}

		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := true

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = false
					return
				case <-tick.C:
					result = true
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithFlushCooldown(5 * time.Second).
			WithBatchSize(2).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: IdentifierFrom("yope"),
		})

		require.Nil(t, client.flushWaiting)

		client.Track("asd", "KILL", PointerFrom(1), nil)
		// This will start the waiter
		client.Flush()

		require.NotNil(t, client.flushWaiting)

		// This will call process() because of full queue
		client.Track("asd", "KILL", PointerFrom(2), nil)

		wg.Wait()

		require.Len(t, client.eventQueue, 0)
		require.Len(t, client.identifierQueue, 0)

		if !result {
			t.Fatal("valid client complete test failed with error")
		}
	})

	t.Run("test round tracking", func(t *testing.T) {
		clientID := os.Getenv("ALLIANCE_CLIENT_ID")
		clientSecret := os.Getenv("ALLIANCE_CLIENT_SECRET")
		gameID := os.Getenv("ALLIANCE_GAME_ID")

		if clientID == "" || clientSecret == "" || gameID == "" {
			t.SkipNow()
		}

		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := true

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = false
					return
				case <-tick.C:
					result = true
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithFlushCooldown(5 * time.Second).
			WithBatchSize(3).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		// 1st event
		client.StartGame("asd-test-1")

		r := client.StartRound("round-id-1", Traits{"test": "yeap"})
		// 2nd event
		r.Track("asd-test-1", "WIN", PointerFrom(10), Traits{"round": "flawless"})
		// 3rd event, this calls process
		r.Track("asd-test-1", "DIE", nil, Traits{"test": "yup"})

		wg.Wait()

		require.Len(t, client.eventQueue, 0)
		require.Len(t, client.identifierQueue, 0)

		if !result {
			t.Fatal("valid client round track test failed with error")
		}
	})

	t.Run("test identifiers", func(t *testing.T) {
		clientID := os.Getenv("ALLIANCE_CLIENT_ID")
		clientSecret := os.Getenv("ALLIANCE_CLIENT_SECRET")
		gameID := os.Getenv("ALLIANCE_GAME_ID")

		if clientID == "" || clientSecret == "" || gameID == "" {
			t.SkipNow()
		}

		errChan := make(chan error)

		var wg sync.WaitGroup
		wg.Add(1)

		// Cancel test in 5 seconds
		tick := time.NewTicker(5 * time.Second)

		result := true

		go func() {
			defer wg.Done()

			for {
				select {
				case e := <-errChan:
					fmt.Println(e)
					result = false
					return
				case <-tick.C:
					result = true
					return
				}
			}
		}()

		client := NewClientBuilder().
			WithFlushCooldown(3 * time.Second).
			WithBatchSize(2).
			WithErrorChannel(errChan).
			Build()
		defer client.Close()

		client.SetIdentifiers("asd", &Identifiers{
			AppleID:   IdentifierFrom("yope"),
			DiscordID: IdentifierFrom("yope"),
		})

		client.SetIdentifiers("asd", &Identifiers{
			TwitterId: IdentifierFrom("asd"),
		})

		client.SetIdentifiers("asd", &Identifiers{
			DiscordID: RemoveIdentifier(),
		})

		require.NotNil(t, client.flushWaiting)

		wg.Wait()

		require.Len(t, client.eventQueue, 0)
		require.Len(t, client.identifierQueue, 0)

		if !result {
			t.Fatal("valid client identifiers test failed with error")
		}
	})
}

func TestCombineTraits(t *testing.T) {
	testCases := []struct {
		name     string
		a, b     Traits
		expected Traits
	}{
		{
			name:     "both nil",
			expected: Traits{},
		},
		{
			name:     "only a nil",
			b:        Traits{},
			expected: Traits{},
		},
		{
			name:     "only b nil",
			a:        Traits{},
			expected: Traits{},
		},
		{
			name:     "both empty",
			a:        Traits{},
			b:        Traits{},
			expected: Traits{},
		},
		{
			name:     "a has value",
			a:        Traits{"a": "a"},
			b:        Traits{},
			expected: Traits{"a": "a"},
		},
		{
			name:     "b has value",
			a:        Traits{},
			b:        Traits{"b": "b"},
			expected: Traits{"b": "b"},
		},
		{
			name:     "a has 2 vaues",
			a:        Traits{"a1": "a", "a2": "a"},
			expected: Traits{"a1": "a", "a2": "a"},
		},
		{
			name:     "b has same value as a",
			a:        Traits{"c": "c"},
			b:        Traits{"c": "d"},
			expected: Traits{"c": "d"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, combineTraits(tc.a, tc.b))
		})
	}
}

func TestIdentifiersJSON(t *testing.T) {
	testCases := []struct {
		name     string
		i        *Identifiers
		expected string
	}{
		{
			name:     "empty",
			i:        &Identifiers{},
			expected: "{}",
		},
		{
			name: "apple id nil",
			i: &Identifiers{
				AppleID: nil,
			},
			expected: "{}",
		},
		{
			name: "apple id empty string",
			i: &Identifiers{
				AppleID: IdentifierFrom(""),
			},
			expected: `{"appleId":null}`,
		},
		{
			name: "apple id normal string",
			i: &Identifiers{
				AppleID: IdentifierFrom("normal"),
			},
			expected: `{"appleId":"normal"}`,
		},
		{
			name: "apple and twitter id empty string",
			i: &Identifiers{
				AppleID:   IdentifierFrom(""),
				TwitterId: IdentifierFrom(""),
			},
			expected: `{"appleId":null,"twitterId":null}`,
		},
		{
			name: "apple and twitter id normal strings",
			i: &Identifiers{
				AppleID:   IdentifierFrom("a"),
				TwitterId: IdentifierFrom("b"),
			},
			expected: `{"appleId":"a","twitterId":"b"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := json.Marshal(tc.i)
			require.Nil(t, err)
			require.Equal(t, tc.expected, string(s))
		})
	}
}
