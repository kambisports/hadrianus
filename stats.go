package main

import (
	"log"
	"os"
	"time"
)

type statUpdate struct {
	updateType StatUpdateType
	metricPath string
	value      int64
}

func statsListener(statsCounterChannel chan statUpdate, incomingMessageChannel chan metricMessage) {
	counterStats := make(map[string]int64)
	oldCounterStats := make(map[string]int64)
	gaugeStats := make(map[string]int64)
	for {
		metricUpdateMessage := <-statsCounterChannel

		if metricUpdateMessage.updateType == IncrementCounter {
			counterStats[metricUpdateMessage.metricPath]++
		} else if metricUpdateMessage.updateType == IncreaseCounter {
			counterStats[metricUpdateMessage.metricPath] += metricUpdateMessage.value
		} else if metricUpdateMessage.updateType == SetCounterValue {
			counterStats[metricUpdateMessage.metricPath] = metricUpdateMessage.value
		} else if metricUpdateMessage.updateType == IncrementGauge {
			gaugeStats[metricUpdateMessage.metricPath]++
		} else if metricUpdateMessage.updateType == DecrementGauge {
			gaugeStats[metricUpdateMessage.metricPath]--
		} else if metricUpdateMessage.updateType == SetGaugeValue {
			gaugeStats[metricUpdateMessage.metricPath] = metricUpdateMessage.value
		} else if metricUpdateMessage.updateType == Generate {
			timeStamp := time.Now().Unix()
			for key, value := range counterStats {
				if oldValue, ok := oldCounterStats[key]; ok {
					if oldValue > 0 {
						// Send metrics message with statistics using the graphite connection
						writeIncomingMessage(incomingMessageChannel, metricMessage{metricPath: key, value: float64(value - oldValue), timestamp: timeStamp}, statsCounterChannel)
					}
				} else {
					// No previous value to compare with. Just output current value
					writeIncomingMessage(incomingMessageChannel, metricMessage{metricPath: key, value: float64(value), timestamp: timeStamp}, statsCounterChannel)
				}
				oldCounterStats[key] = value
			}
			for key, value := range gaugeStats {
				// Send metrics message with statistics using the graphite connection
				writeIncomingMessage(incomingMessageChannel, metricMessage{metricPath: key, value: float64(value), timestamp: timeStamp}, statsCounterChannel)
			}
		}
	}
}

func initializeInternalMetricsPaths() {
	// Extract hostname for naming internal hadrianus metrics
	hostname, err := os.Hostname()
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}
	pathDiscardedChattyMessage = `server.hadrianus.` + hostname + `.discardedChattyMessage`
	pathDiscardedStaleAndChattyMessage = `server.hadrianus.` + hostname + `.discardedStaleAndChattyMessage`
	pathDiscardedStaleMessage = `server.hadrianus.` + hostname + `.discardedStaleMessage`
	pathEncounteredMetricPaths = `server.hadrianus.` + hostname + `.encounteredMetricPaths`
	pathGarbageCollectionPauseMs = `server.hadrianus.` + hostname + `.garbageCollectionPauseMs`
	pathGarbageCollections = `server.hadrianus.` + hostname + `.garbageCollections`
	pathSentMessage = `server.hadrianus.` + hostname + `.sentMessage`
	pathStaleMetricPaths = `server.hadrianus.` + hostname + `.staleMetricPaths`
	pathInvalidMessage = `server.hadrianus.` + hostname + `.invalidMessage`
	pathReceivedMessage = `server.hadrianus.` + hostname + `.receivedMessage`
	pathAllocatedMemoryMegabytes = `server.hadrianus.` + hostname + `.allocatedMemoryMegabytes`
	pathClientConnectionOpening = `server.hadrianus.` + hostname + `.clientConnectionOpening`
	pathClientConnectionClosing = `server.hadrianus.` + hostname + `.clientConnectionClosing`
	pathClientConnectionsActive = `server.hadrianus.` + hostname + `.clientConnectionsActive`
	pathGoroutines = `server.hadrianus.` + hostname + `.goroutines`
	pathStatsOverflows = `server.hadrianus.` + hostname + `.statsOverflows`
	pathToOutPoolOverflows = `server.hadrianus.` + hostname + `.toOutPoolOverflows`
	pathIncomingMessageOverflows = `server.hadrianus.` + hostname + `.incomingMessageOverflows`
	pathToOutConnectionOverflows = `server.hadrianus.` + hostname + `.toOutConnectionOverflows`
	pathCleanupTimeMilli = `server.hadrianus.` + hostname + `.cleanupTimeMilli`
}
