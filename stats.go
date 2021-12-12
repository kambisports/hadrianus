package main

import (
	"log"
	"os"
	"time"
)

// Enums for counters
type CounterId int

const (
	CleanupTimeMilli CounterId = iota
	ClientConnectionClosing
	ClientConnectionOpening
	DiscardedChattyMessage
	DiscardedStaleAndChattyMessage
	DiscardedStaleMessage
	DroppedIncomingMessages
	DroppedOutPool
	DroppedOutConnection
	GarbageCollectionPauseMs
	GarbageCollections
	IncomingMessageOverflows
	InvalidMessage
	ReceivedMessage
	SentMessage
	ToOutConnectionOverflows
	ToOutPoolOverflows
)

var counterData [ToOutPoolOverflows + 1]int64
var oldCounterData [ToOutPoolOverflows + 1]int64
var counterPath [ToOutPoolOverflows + 1]string

// Enums for gauges
type GaugeId int

const (
	AllocatedMemoryMegabytes GaugeId = iota
	ClientConnectionsActive
	EncounteredMetricPaths
	Goroutines
	StaleMetricPaths
)

var gaugeData [StaleMetricPaths + 1]int64
var gaugePath [StaleMetricPaths + 1]string

var timesStatsGenerated int64

func generateInternalStats(incomingMessageChannel chan metricMessage) {
	timeStamp := time.Now().Unix()
	for key, value := range counterData {
		var metricValue float64
		oldValue := oldCounterData[key]

		if timesStatsGenerated < 1 {
			metricValue = float64(value)
		} else if oldValue > 0 {
			metricValue = float64(value - oldValue)
		} else {
			metricValue = 0
		}

		// Send metrics message with statistics using the graphite connection
		writeIncomingMessage(incomingMessageChannel, metricMessage{metricPath: baseMetricsPath + counterPath[key], value: float64(metricValue), timestamp: timeStamp})
		oldCounterData[key] = value
	}
	for key, value := range gaugeData {
		// Send metrics message with statistics using the graphite connection
		writeIncomingMessage(incomingMessageChannel, metricMessage{metricPath: baseMetricsPath + gaugePath[key], value: float64(value), timestamp: timeStamp})
	}
	timesStatsGenerated++
}

func initializeInternalMetricsPaths() {
	// Extract hostname for naming internal hadrianus metrics
	hostname, err := os.Hostname()
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}
	baseMetricsPath = `server.hadrianus.` + hostname + `.`

	counterPath = [ToOutPoolOverflows + 1]string{
		`cleanupTimeMilli`,
		`clientConnectionClosing`,
		`clientConnectionOpening`,
		`discardedChattyMessage`,
		`discardedStaleAndChattyMessage`,
		`discardedStaleMessage`,
		`droppedIncomingMessages`,
		`droppedOutPool`,
		`droppedOutConnection`,
		`garbageCollectionPauseMs`,
		`garbageCollections`,
		`incomingMessageOverflows`,
		`invalidMessage`,
		`receivedMessage`,
		`sentMessage`,
		`toOutConnectionOverflows`,
		`toOutPoolOverflows`,
	}
	gaugePath = [StaleMetricPaths + 1]string{
		`allocatedMemoryMegabytes`,
		`clientConnectionsActive`,
		`encounteredMetricPaths`,
		`goroutines`,
		`staleMetricPaths`,
	}
}
