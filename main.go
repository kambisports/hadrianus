package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	OutgoingChannelSize      = 65536
	IncomingChannelSize      = 65536
	PoolChannelSize          = 65536
	StatsChannelSize         = 32768
	NanosecondsInMillisecond = 1000000
	BytesInMegabyte          = 1048576

	StatsTimeGranularity        = 60
	CleanupTimeGranularity      = 601
	CleanupMaxAge               = 3600
	MinimumTimeInterval         = 14
	MaxConsecutiveDryMessages   = 120 // If more than this number of messages with the same value have been sent, mark as "stale"
	IsNewMetricEnabledByDefault = false
	StaleResendInterval         = 0

	BlockOnChannelBufferFull = true
	TcpNoDelay               = false // Disable delay of sending successive small packets
)

// Commandline flag variable definitions
var (
	isNewMetricEnabledByDefault = flag.Bool("enablenewmetrics", IsNewMetricEnabledByDefault, "initially enable new metrics")
	minimumTimeInterval         = flag.Int64("minimumtimeinterval", MinimumTimeInterval, "minimum allowed time interval between incoming metrics in seconds")
	statsTimeGranularity        = flag.Int64("statstimegranularity", StatsTimeGranularity, "time between statistics messages in seconds")
	maxConsecutiveDryMessages   = flag.Uint64("maxdrymessages", MaxConsecutiveDryMessages, "maximum allowed consecutive identical values before marking metric as stale")
	staleResendInterval         = flag.Int64("staleresendinterval", StaleResendInterval, "time after which stale messages are resent in seconds")
	mirrorDestination           = flag.String("mirrordestination", "", "secondary destinations to mirror traffic to")
	tertiaryDestination         = flag.String("tertiarydestination", "", "tertiary destinations to mirror traffic to")
	cleanupTimeGranularity      = flag.Int64("cleanuptimegranularity", CleanupTimeGranularity, "seconds between cleanup events")
	cleanupMaxAge               = flag.Int64("cleanupmaxage", CleanupMaxAge, "maximum time in seconds since last message")
	override                    = flag.String("override", "", "filename for override file")
)

// Internal hadrianus metrics paths
var (
	pathDiscardedChattyMessage         string
	pathDiscardedStaleAndChattyMessage string
	pathDiscardedStaleMessage          string
	pathEncounteredMetricPaths         string
	pathGarbageCollectionPauseMs       string
	pathGarbageCollections             string
	pathSentMessage                    string
	pathStaleMetricPaths               string
	pathInvalidMessage                 string
	pathReceivedMessage                string
	pathAllocatedMemoryMegabytes       string
	pathClientConnectionOpening        string
	pathClientConnectionClosing        string
	pathClientConnectionsActive        string
	pathGoroutines                     string
	pathToOutPoolOverflows             string
	pathIncomingMessageOverflows       string
	pathToOutConnectionOverflows       string
	pathCleanupTimeMilli               string
)

var timeToCleanup = false

type StatUpdateType int

const (
	IncrementCounter StatUpdateType = iota
	IncreaseCounter
	SetCounterValue
	IncrementGauge
	DecrementGauge
	SetGaugeValue
	Generate
)

type metricMessage struct {
	metricPath string
	value      float64
	timestamp  int64
}

type metricData struct {
	unchangedCounter uint64
	lastValue        float64
	lastSentOut      int64
	lastTimestamp    int64
	outputActive     bool
	allowUnmodified  bool // Pass metric through as-is, no matter what?
}

func main() {

	optind := permutateArgs(os.Args) // sort flags to front of args list
	nonFlagArgument := os.Args[optind:]
	flag.Parse()

	if len(nonFlagArgument) < 2 {
		fmt.Println("Usage: hadrianus listeningport outport1...")
		return
	}

	incomingPort := nonFlagArgument[0]

	primaryMetricsOutput := nonFlagArgument[1:]

	var outgoingHostPort [][]string

	// Process and sanity check output cluster arguments
	err := mungeClusterNodesDestinations(primaryMetricsOutput)
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}
	outgoingHostPort = append(outgoingHostPort, primaryMetricsOutput)

	// Process and sanity check mirror output cluster arguments
	mirrorNode := strings.Split(*mirrorDestination, " ")
	if *mirrorDestination != "" {
		err := mungeClusterNodesDestinations(mirrorNode)
		if err != nil {
			log.Println(err)
			os.Exit(1)
			return
		}
		outgoingHostPort = append(outgoingHostPort, mirrorNode)
	}

	// Process and sanity check tertiary output cluster arguments
	tertiaryNode := strings.Split(*tertiaryDestination, " ")
	if *tertiaryDestination != "" {
		err := mungeClusterNodesDestinations(tertiaryNode)
		if err != nil {
			log.Println(err)
			os.Exit(1)
			return
		}
		outgoingHostPort = append(outgoingHostPort, tertiaryNode)
	}

	// Process and sanity check override file argument
	var storageSchema map[string]overrideData

	if len(*override) > 0 {
		storageSchema = getStorageSchemaFromFile(*override)
	}

	// Automatically whitelist internal hadrianus metrics
	if len(storageSchema) == 0 {
		storageSchema = make(map[string]overrideData)
	}
	internalHadrianusPattern, _ := regexp.Compile(`^server\.hadrianus\.`)
	storageSchema[`hadrianus`] = overrideData{pattern: internalHadrianusPattern, allowUnmodifiedActive: true, allowUnmodified: true}

	initializeInternalMetricsPaths()

	statsCounterChannel := make(chan statUpdate, StatsChannelSize)
	incomingMessageChannel := make(chan metricMessage, IncomingChannelSize)
	outgoingToPoolChannel := make(chan metricMessage, PoolChannelSize)

	// Create listener for stats channel
	go statsListener(statsCounterChannel, incomingMessageChannel)

	// Create listening socket
	go createIncomingConnections(incomingPort, incomingMessageChannel, statsCounterChannel)

	// Create outgoing pool
	go handleOutgoingPool(outgoingToPoolChannel, outgoingHostPort, statsCounterChannel)

	metric := make(map[string]*metricData)

	// Trigger periodic stats generation
	go func() {
		d := time.Duration(*statsTimeGranularity) * time.Second
		var garbageCollectionStats debug.GCStats
		var memoryStats runtime.MemStats
		for range time.Tick(d) {
			// Update GC stats
			debug.ReadGCStats(&garbageCollectionStats)
			writeStats(statsCounterChannel, statUpdate{SetCounterValue, pathGarbageCollections, int64(garbageCollectionStats.NumGC)})
			writeStats(statsCounterChannel, statUpdate{SetCounterValue, pathGarbageCollectionPauseMs, int64(garbageCollectionStats.PauseTotal / NanosecondsInMillisecond)})

			// Update memory stats
			runtime.ReadMemStats(&memoryStats)
			writeStats(statsCounterChannel, statUpdate{SetGaugeValue, pathAllocatedMemoryMegabytes, int64(memoryStats.Alloc / BytesInMegabyte)})

			// Update goroutine stats
			writeStats(statsCounterChannel, statUpdate{SetGaugeValue, pathGoroutines, int64(runtime.NumGoroutine())})

			// Trigger generation of stats for counters
			writeStats(statsCounterChannel, statUpdate{updateType: Generate})
		}
	}()

	// Trigger periodic cleanup
	go func() {
		d := time.Duration(*cleanupTimeGranularity) * time.Second
		for range time.Tick(d) {
			timeToCleanup = true
		}
	}()

	// Main loop
	for {
		fromConnection := <-incomingMessageChannel

		var instance *metricData
		var found bool

		// Check if data for the metric path needs to be created
		if instance, found = metric[fromConnection.metricPath]; !found {
			writeStats(statsCounterChannel, statUpdate{IncrementGauge, pathEncounteredMetricPaths, 0})

			// Initialize data for newly discovered metric
			instance = &metricData{
				outputActive:     *isNewMetricEnabledByDefault,
				unchangedCounter: 0,
				lastValue:        fromConnection.value,
				lastSentOut:      fromConnection.timestamp - *minimumTimeInterval,
				lastTimestamp:    fromConnection.timestamp,
				allowUnmodified:  false,
			}
			if !*isNewMetricEnabledByDefault {
				writeStats(statsCounterChannel, statUpdate{IncrementGauge, pathStaleMetricPaths, 0})
			}

			// Check if the newly discovered metric path matches patterns in the override file
			for _, value := range storageSchema {
				if value.pattern.Match([]byte(fromConnection.metricPath)) {
					if value.retentionActive {
						// Do nothing. Not yet implemented.
					}
					if value.maxDryMessagesThresholdActive {
						// Do nothing. Not yet implemented.
					}
					if value.allowUnmodifiedActive {
						instance.allowUnmodified = value.allowUnmodified
					}
					break // Stop trying to match against more patterns
				}
			}
			metric[fromConnection.metricPath] = instance
		}

		// If metric path is allowUnmodified, send it out, no matter what.
		if instance.allowUnmodified {
			writeToOutPool(outgoingToPoolChannel, fromConnection, statsCounterChannel)
			instance.lastSentOut = fromConnection.timestamp
			writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathSentMessage, 0})
		} else {

			// Check that the metric value hasn't gone stale
			if fromConnection.value == instance.lastValue {
				instance.unchangedCounter++
				if instance.outputActive && instance.unchangedCounter >= *maxConsecutiveDryMessages {
					instance.outputActive = false
					writeStats(statsCounterChannel, statUpdate{IncrementGauge, pathStaleMetricPaths, 0})
				}
			} else {
				instance.unchangedCounter = 0
				if !instance.outputActive {
					instance.outputActive = true
					writeStats(statsCounterChannel, statUpdate{DecrementGauge, pathStaleMetricPaths, 0})
					// Send out previous "silenced" metric to make data nicer
					writeToOutPool(outgoingToPoolChannel, metricMessage{fromConnection.metricPath, instance.lastValue, instance.lastTimestamp}, statsCounterChannel)
				}
			}

			// Check that the metric doesn't come in too often
			chatty := fromConnection.timestamp < (instance.lastSentOut + *minimumTimeInterval)

			// Allow resending of stale metric periodically to keep it "alive"
			timeToResendStaleMessage := *staleResendInterval > 0 && fromConnection.timestamp > (instance.lastSentOut+*staleResendInterval)

			// Send out metric if not stale or not chatty
			if timeToResendStaleMessage || instance.outputActive && !chatty {
				writeToOutPool(outgoingToPoolChannel, fromConnection, statsCounterChannel)
				instance.lastSentOut = fromConnection.timestamp
				writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathSentMessage, 0})
			} else if !instance.outputActive && chatty {
				writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathDiscardedStaleAndChattyMessage, 0})
			} else if !instance.outputActive && !chatty {
				writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathDiscardedStaleMessage, 0})
			} else if instance.outputActive && chatty {
				writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathDiscardedChattyMessage, 0})
			}
		}
		instance.lastValue = fromConnection.value
		instance.lastTimestamp = fromConnection.timestamp

		if timeToCleanup {
			timeToCleanup = false // Reset the cleanup indicator
			timeNow := time.Now().Unix()
			beginTime := time.Now().UnixMilli()
			for metricPath, metricData := range metric {
				if timeNow >= (metricData.lastTimestamp + *cleanupMaxAge) {
					// If a disabled metric is removed, decrement the number of stale
					// metrics paths since the path doesn't exist in memory anymore
					if !metricData.outputActive {
						writeStats(statsCounterChannel, statUpdate{DecrementGauge, pathStaleMetricPaths, 0})
					}
					delete(metric, metricPath)
					writeStats(statsCounterChannel, statUpdate{DecrementGauge, pathEncounteredMetricPaths, 0})
				}
			}
			endTime := time.Now().UnixMilli()
			writeStats(statsCounterChannel, statUpdate{IncreaseCounter, pathCleanupTimeMilli, endTime - beginTime})
		}
	}
}

func parseGraphiteMessage(graphiteMessage string) (metricMessage, error) {
	var outputMessage metricMessage
	var err error
	splitString := strings.Split(graphiteMessage, " ")
	if len(splitString) != 3 {
		return outputMessage, errors.New("Wrong number of fields in graphite message: " + graphiteMessage)
	}
	if len(splitString[0]) < 1 {
		return outputMessage, errors.New("Length of metric_path too short (0) in graphite message: " + graphiteMessage)
	}
	outputMessage.metricPath = splitString[0]
	if splitString[1] == "NaN" {
		return outputMessage, errors.New("Invalid value field in graphite message: " + graphiteMessage)
	}
	if splitString[1] == "-Inf" {
		return outputMessage, errors.New("Invalid value field in graphite message: " + graphiteMessage)
	}
	if outputMessage.value, err = strconv.ParseFloat(splitString[1], 64); err != nil {
		return outputMessage, errors.New("Invalid value field in graphite message: " + graphiteMessage)
	}
	if outputMessage.timestamp, err = strconv.ParseInt(splitString[2], 10, 64); err != nil {
		return outputMessage, errors.New("Invalid timestamp field in graphite message: " + graphiteMessage)
	}
	if outputMessage.timestamp == -1 {
		outputMessage.timestamp = time.Now().Unix()
	}
	return outputMessage, err
}

// permutateArgs permutates args such that options are in front,
// leaving the program name untouched. permutateArgs returns the
// index of the first non-option after permutation.
func permutateArgs(args []string) int {
	args = args[1:]
	var flags []string
	var arguments []string
	optind := 0
	for i := range args {
		if args[i][0] == '-' {
			flags = append(flags, args[i])
			optind++
		} else {
			arguments = append(arguments, args[i])
		}
	}

	for i := range args {
		if i < optind {
			args[i] = flags[i]
		} else {
			args[i] = arguments[i-optind]
		}
	}

	args = append(flags, arguments...)
	return optind + 1
}
