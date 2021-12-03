package main

/*
Break out channel writes into separate functions to simplify handling of channel buffers.
*/

func writeStats(statsCounterChannel chan statUpdate, update statUpdate) {
	select {
	case statsCounterChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathStatsOverflows, 0})
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
		/*
			// It *seems* like the process is likely to hang if this is blocking. Let's disable that for now.
			if BlockOnChannelBufferFull {
				outgoingToPoolChannel <- update
			}
		*/
	}
}

func writeIncomingMessage(incomingMessageChannel chan metricMessage, update metricMessage, statsCounterChannel chan statUpdate) {
	select {
	case incomingMessageChannel <- update:
	default:
		writeStats(statsCounterChannel, statUpdate{IncrementCounter, pathIncomingMessageOverflows, 0})
		/*
			// It *seems* like there's a big increase in sockets in CLOSE_WAIT state when it crashes.
			// If we stop blocking on incoming messages, does this stop happening?
			if BlockOnChannelBufferFull {
				incomingMessageChannel <- update
			}
		*/
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
