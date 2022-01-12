# hadrianus

![Hadrianus](hadrianus_logo.svg)

Block incoming graphite metrics if they come in too fast for downstream carbon-relay/carbon-cache to handle.

## Building

Hadrianus is written in Go so all you need is: `go build`

As a convenience there's a small `Makefile`. To build:

* `make all` <- Generates a Linux amd64 executable
* `make all-mac` <- Generates a Darwin amd64 executable

## Usage

### Basic usage

`hadrianus listeningport outport1...`

* `listeningport` is the port number for listening to incoming newline delimited graphite protocol messages.
* `outport1` denotes the first (out of possibly many) output ports for carbon-relay process instances. Metrics will be distributed to the destinations in a "round robin" fashion.

### Options

* `-cleanupmaxage` Maximum time in seconds since last message before metric path is removed from memory.
* `-cleanuptimegranularity` Seconds between memory cleanup events (default 86401).
* `-enablenewmetrics` Initially enable new metrics and block them later if needed.
* `-maxdrymessages` Maximum allowed consecutive identical values before marking metric as stale. Only meaningful if `-enablenewmetrics` is used.
* `-maxdrylimit` Maximum number of messages that dry threshold may be increased to.
* `-minimumtimeinterval` Minimum allowed time interval between incoming metrics in seconds. Lower values makes hadrianus more "generous" in how often applications may send a specific metric.
* `-mirrordestination` Secondary destination(s) to mirror traffic to.
* `-override` Filename for per-path override file that allows allowlisting.
* `-staleresendinterval` Time after which stale messages are resent in seconds.
* `-statstimegranularity` Time between statistics messages in seconds.
* `-tertiarydestination` Tertiary destination(s) to mirror traffic to.

## What

Hadrianus can reduce the total number of metrics, and save significant amounts of storage and network capacity by limiting:

* How often the value of a metric path is allowed to be sent.
* How many times the value of a metric path is allowed to be unchanged.

For example, setting a `timelimit` of 60 would result in a message only being transmitted once per minute, or more seldom.

These techniques can also increase the stability and reliability of an under dimensioned graphite/carbon system.

## Example commandline usage

### Typical usage

`hadrianus -minimumtimeinterval=14 2003 2103 2203 server01.iambk.com:2303`

Tell hadrianus to discard unique metric paths that arrive sooner than or equal to every 14 seconds. It will listen for plaintext graphite messages on port 2003. It will attempt to distribute the incoming messages to 127.0.0.1:2103, 127.0.0.1:2203 and server01.iambk.com:2303 in a round-robin fashion.

### Allowing a metric path to pass through unmodified

It's possible to allow a metric path to pass through without being touched by the blocking logic by adding a matching pattern in a separate configuration file. In this example the configuration file `allowlist.conf` is as follows:

```ini
[hadrianus]
pattern = ^server\.hadrianus\.
allowunmodified = true
```

Referring to the "allowlist" file is done on the commandline by using the `-override` flag as follows: `hadrianus -override=allowlist.conf -minimumtimeinterval=14 2003 2103 2203 server01.iambk.com:2303`

## Internal metrics

Hadrianus (currently) exposes metrics on the path `server.hadrianus.<servername>.*`.

Please note that the values of the metrics correspond to the `statstimegranularity` specified. For example: if `receivedMessage` has a value of 4.23 million, and the granularity of stats is 60 seconds, this will mean that the number of received messages per second is `4,230,000 / 60`, which equals `70,500` messages per second.

### discardedChattyMessage

The number of messages that have been discarded because they are coming in faster than is allowed by the `minimumtimeinterval` setting.

### discardedStaleMessage

The number of messages that have been discarded because they have been judged to be "stale" due to their values not changing often enough, or ever, as decided by the `maxdrymessages` setting.

### discardedStaleAndChattyMessage

The number of messages that have been discarded *both* because they have been judged to be stale and because they are coming in too fast.

### encounteredMetricPaths

The number of unique metric paths that have been encountered so far, during the running time of the service.

### staleMetricPaths

The number of unique metric paths that are judged to be "stale", as decided by the `maxdrymessages` setting. These messages will not be sent to the downstream metrics consumers.

### garbageCollectionPauseMs

The amount of time that has been spent on doing garbage collection.

### garbageCollections

The number of garbage collection operations that have been performed.

### receivedMessage

The number of messages that have been received from metrics producers.

### sentMessage

The number of messages that have been sent to the downstream metrics consumers after filtering and throttling operations have been applied.

### invalidMessage

The number of messages that have been received which do not correspond to valid Graphite wire protocol messages.

### allocatedMemoryMegabytes

The amount of memory allocated by the service in Megabytes.

### clientConnectionOpening

The number of opened TCP connections.

### clientConnectionClosing

The number of closed TCP connections.

### clientConnectionsActive

The number of currently active TCP connections.

### goroutines

The number of goroutines currently used. This will typically only change when the number of concurrent network connections changes.

### toOutPoolOverflows

The number of overflows when the output connection pool channel buffer is written to. If this goes up, it can indicate a severe performance issue.

### incomingMessageOverflows

The number of overflows when the incoming metrics producer channel buffer is being written to.

### toOutConnectionOverflows

The number of overflows when the output connection channel buffers are written to. If this goes up, it's possible that downstream metrics consumers cannot consume data fast enough.

### cleanupTimeMilli

The time in milliseconds that it took for the "cleanup" job to complete. This job will halt the central processing of data. If this takes a long time, it can stop the whole service and cause things to queue up.
