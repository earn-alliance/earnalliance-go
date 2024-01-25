<p align="center">
  <a href="https://www.earnalliance.com?utm_source=github&utm_medium=logo" target="_blank">
    <img src="https://www.earnalliance.com/new/svgs/ea_logo.svg" alt="Earn Alliance" width="280">
  </a>
</p>

![GitHub Tag](https://img.shields.io/github/v/tag/earn-alliance/earnalliance-go)
[![Go Reference](https://pkg.go.dev/badge/github.com/earn-alliance/earnalliance-go.svg)](https://pkg.go.dev/github.com/earn-alliance/earnalliance-go)
![Test](https://github.com/earn-alliance/earnalliance-go/workflows/Tests/badge.svg)
[![Discord](https://img.shields.io/discord/926167446648397836)](http://discord.gg/2VqABVytBZ)

# Official Earn Alliance SDKs for Go

This is the Earn Alliance SDK for Go.

## Links

- [![Discord](https://img.shields.io/discord/926167446648397836)](http://discord.gg/2VqABVytBZ)
- [![Twitter Follow](https://img.shields.io/twitter/follow/earnalliance?label=Earn%20Alliance&style=social)](https://twitter.com/intent/follow?screen_name=earnalliance)

## Contents

- [Supported Platforms](#supported-platforms)
- [Installation and Usage](#installation-and-usage)

## Supported Platforms

We currently support the Go language in this package.

## Installation and Usage

To install the SDK, get the package via:

```sh
go get github.com/earn-alliance/earnalliance-go
```

### Initialize

Build the client via the ClientBuilder. The example below shows only the required options.
Check the ClientBuilder documentation to see the complete list.

```go
import ea "github.com/earn-alliance/earnalliance-go"

client := ea.NewClientBuilder().
    WithClientID("[client id]").
    WithClientSecret("[client secret]").
    WithGameID("[game id]").
    WithDSN("[dsn]"). // Optional
    Build()
// Stops the goroutines running in the background.
defer client.Close()
```

The client configuration can also be read from environment variables if not
provided as an option.

```go
// When the NewClientBuilder function is called, it will look
// for the environment variables `ALLIANCE_CLIENT_ID`, `ALLIANCE_CLIENT_SECRET`,
// `ALLIANCE_GAME_ID` and optionally `ALLIANCE_DSN`. The builder will use these to set the values.
client := ea.NewClientBuilder().Build()
```

### Set User Identifiers

Whenever a new user identifier is set, or a new user is registered, you can add or update the identifiers associated with the internal user id.

This is used to tell us the user has installed the app and enrich their information when more game platform accounts or social accounts are added to help us map the user to the game events.

```go
// This shows all of our currently supported platforms, but you only need to
// provide the identifiers that are relevant for your game.
client.SetIdentifiers("[internal user id]", &ea.Identifiers{
    AppleID: ea.IdentifierFrom("..."),
    DiscordID: ea.IdentifierFrom("..."),
    Email: ea.IdentifierFrom("..."),
    EpicGamesID: ea.IdentifierFrom("..."),
    WalletAddress: nil, // This will be ignored, same thing as not setting this value.
})
```

Note that if you pass a nil pointer in `ea.Identifiers`, that identifier will simply be ignored.
If however, you pass an empty string, then that identifier will be removed from the user's account.

```go
// You can remove a previously set identifier like this.
client.Identify("[internal user id]", &ea.Identifiers(
    AppleID: ea.RemoveIdentifier(), // This will be removed from the user's account.
))
```

### Track User Start Session

Sends standard TRACK event for launching a game. This lets us know that the user
has launched the game and is ready to start a challenge.

```go
client.StartGame("[internal user id]")
```

### Track Events

Tracking events that happens in a game. Tracked events are batched together and sent after 30 seconds interval, or when a batch size of 100 events have 
accumulated, whichever comes first. Both the interval and the batch size are
configurable in the client options.

The name of the events can be almost anything, but we recommend sticking to
common terms as shown in the following examples.

```go
// An event without any specific value, commonly used for counting event
// instances, i.e. "Kill X Zombies".
client.Track("[internal user id]", "KILL_ZOMBIE", nil, nil)

// An event with an associated value, commonly used for accumulating or
// checking min / max values, i.e. "Score a total of X" or "Achieve a
// highscore of at least X".
score := 100
client.Track("[internal user id]", "SCORE", &score, nil)

// The client can track events for multiple users in parallel.
client.Track("[internal user id]", "DAMAGE_DONE", 500, nil)
client.Track("[another user id]", "DAMAGE_TAKEN", 500, nil)

// Additional traits can be added to the event, which can be used to
// create more detailed challenges, i.e. "Kill X monsters with a knife".
client.Track("[internal user id]", "KILL", ea.Traits{ "weapon": "knife", "mob": "zombie" })
```

### Group Tracked Events

You can group events together, i.e. a game round or a match, whichever makes
sense for your game. This allows for natural challenge scopes like "Kill X players
in a match".

```go
// If the first parameter is empty, the library will generate a fresh UUID.
// And this ID will be the Group ID of events tracked for this round.
round := client.StartRound("", nil)
round.Track("[internal user id]", "KILL_ZOMBIE")

// You can set the round's ID yourself when you first create it.
round := client.StartRound("[my-custom-id]", nil)

// Additional traits can be set when starting the round, which will be added
// to all events that are tracked for the specific round.
round := client.StartRound("", ea.Traits{ "map": "nuclear_wasteland" })
round.Track("[internal user id]", "KILL_ZOMBIE")
```

### Flush event queue

For events that have higher priority (i.e. `SetIdentifiers`), instead of
initiating a scheduled batch, they trigger the use of an event queue flush.
This means that as long as the client has not been flushed prior to the event,
the event will be sent to the API right away.

Once the queue has been has been flushed, the client enters a cooldown period
(10 seconds by default), during which any subsequent calls to flush will wait for
the cooldown to finish, before another Flush is ran. These calls also will not
do anything if another Flush call before them started the waiter for the cooldown.
Note that any normal procedures, like the queue size reaching the batch limit,
will still send the events to the API.

The `Flush` function can also be called manually on the client instance, but
it is still restricted by the same cooldown mechanic.

```go
client.Flush()
```
