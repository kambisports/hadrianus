package main

// Break out channel writes into separate functions.
// This is done to simplify handling of channel buffers.

func writeToOutPool(outgoingToPoolChannel chan metricMessage, update metricMessage) {
	select {
	case outgoingToPoolChannel <- update:

		// Restore normal blocking behavior
		outPoolOverflows = 0
		blockOnChannelBufferFull = BlockOnChannelBufferFullDefault
		channelBufferMetricsEnabled = true // Resume buffer overflow message generation
	default:
		if channelBufferMetricsEnabled {
			counterData[ToOutPoolOverflows]++
		}
		if blockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		} else {
			counterData[DroppedOutPool]++
		}

		// If number of overflows of the "out pool" exceeds threshold, stop blocking and discard data
		outPoolOverflows++
		if outPoolOverflows > OverflowsThreshold {
			blockOnChannelBufferFull = false
			channelBufferMetricsEnabled = false // Stop buffer overflow messages temporarily to lessen load
		}
	}
}

func writeIncomingMessage(incomingMessageChannel chan metricMessage, update metricMessage) {
	select {
	case incomingMessageChannel <- update:
	default:
		if channelBufferMetricsEnabled {
			counterData[IncomingMessageOverflows]++
		}
		if blockOnChannelBufferFull {
			incomingMessageChannel <- update
		} else {
			counterData[DroppedIncomingMessages]++
		}
	}
}

func writeToOutConnection(outgoingToPoolChannel chan metricMessage, update metricMessage) {
	select {
	case outgoingToPoolChannel <- update:
	default:
		if channelBufferMetricsEnabled {
			counterData[ToOutConnectionOverflows]++
		}
		if blockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		} else {
			counterData[DroppedOutConnection]++
		}
	}
}
