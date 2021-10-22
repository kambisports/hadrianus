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
* `-cleanuptimegranularity` Seconds between memory cleanup events (default 601)
* `-enablenewmetrics` Initially enable new metrics and block them later if needed.
* `-maxdrymessages` Maximum allowed consecutive identical values before marking metric as stale.
* `-minimumtimeinterval` Minimum allowed time interval between incoming metrics in seconds. Lower values makes hadrianus more "generous" in how often applications may send a specific metric.
* `-mirrordestination` Secondary destination(s) to mirror traffic to.
* `-override` Filename for per-path override file that allows allowlisting.
* `-staleresendinterval` Time after which stale messages are resent in seconds.
* `-statstimegranularity` Time between statistics messages in seconds.
* `-tertiarydestination` Tertiary destination(s) to mirror traffic to.

## What

A newline delimited graphite message works like this: `metric_path value timestamp\n`. The number of messages can be limited per metric path for a time period. For example, setting a `timelimit` of 60 would result in a message only being transmitted once per minute, or more seldom.

This can be useful to increase stability and reliability if you have applications producing more messages than the graphite/carbon system can handle.

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
