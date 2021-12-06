package main

// Break out channel writes into separate functions.
// This is done toto simplify handling of channel buffers.

func writeStats(statsCounterChannel chan statUpdate, update statUpdate) {
	select {
	case statsCounterChannel <- update:
	default:
		if BlockOnChannelBufferFull {
			statsCounterChannel <- update
		}
	}
}

func writeToOutPool(outgoingToPoolChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case outgoingToPoolChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathToOutPoolOverflows, 0})
		if BlockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		}
	}
}

func writeIncomingMessage(incomingMessageChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case incomingMessageChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathIncomingMessageOverflows, 0})
		if BlockOnChannelBufferFull {
			incomingMessageChannel <- update
		}
	}
}

func writeToOutConnection(outgoingToPoolChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case outgoingToPoolChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathToOutConnectionOverflows, 0})
		if BlockOnChannelBufferFull {
			outgoingToPoolChannel <- update
		}
	}
}
