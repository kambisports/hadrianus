package main

// Break out channel writes into separate functions.
// This is done to simplify handling of channel buffers.

func writeStats(statsCounterChannel chan statUpdate, update statUpdate) {
	select {
	case statsCounterChannel <- update:
	default:
		if blockOnChannelBufferFull {
			statsCounterChannel <- update
		}
	}
}

func writeToOutPool(outgoingToPoolChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case outgoingToPoolChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathToOutPoolOverflows, 0})
		if blockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		}
	}
}

func writeIncomingMessage(incomingMessageChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case incomingMessageChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathIncomingMessageOverflows, 0})
		if blockOnChannelBufferFull {
			// Blocking on incoming data could be a stability problem.
			// Since most of the incoming data is filtered away, dropping these messages would have the least impact.
			incomingMessageChannel <- update
		}
	}
}

func writeToOutConnection(outgoingToPoolChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case outgoingToPoolChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathToOutConnectionOverflows, 0})
		if blockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		}
	}
}
