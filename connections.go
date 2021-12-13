package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func handleIncomingConnection(connection net.Conn, incomingMessageChannel chan metricMessage) {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	counterData[ClientConnectionOpening]++
	gaugeData[ClientConnectionsActive]++
	for {
		netData, err := reader.ReadString('\n')
		if err != nil {
			break // Break out of for loop and close connection
		}

		// Parse incoming message and forward it, if the message format is valid
		incomingMessage, err := parseGraphiteMessage(strings.TrimSpace(string(netData)))
		counterData[ReceivedMessage]++

		if err != nil {
			counterData[InvalidMessage]++
		} else {
			writeIncomingMessage(incomingMessageChannel, incomingMessage)
		}
	}
	counterData[ClientConnectionClosing]++
	gaugeData[ClientConnectionsActive]--
}

func createIncomingConnections(incomingPort string, incomingMessageChannel chan metricMessage) {
	listen, err := net.Listen("tcp4", ":"+incomingPort)
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}
	defer listen.Close()

	for {
		connection, err := listen.Accept()

		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		go handleIncomingConnection(connection, incomingMessageChannel)
	}
}

func createOutgoingConnection(outgoingHostPort string, outgoingMessageChannel chan metricMessage) {
	for {
		addr, _ := net.ResolveTCPAddr("tcp", outgoingHostPort)
		connection, err := net.DialTCP("tcp", nil, addr)
		err2 := connection.SetNoDelay(TcpNoDelay)
		if err2 != nil {
			log.Println("Attempt to set \"NoDelay\" failed:", err.Error())
			os.Exit(1)
		}

		if err != nil {
			log.Println("Failed to connect to", outgoingHostPort+":", err.Error())
			os.Exit(1)
		} else {
			for {
				outMessage := <-outgoingMessageChannel
				text := fmt.Sprintln(outMessage.metricPath, outMessage.value, outMessage.timestamp)
				_, err = connection.Write([]byte(text))
				if err != nil {
					log.Println("Write to output to", outgoingHostPort, "failed:", err.Error())
					os.Exit(1)
				}
			}
		}
	}
}

func handleOutgoingPool(outgoingToPoolChannel chan metricMessage, outgoingHostPort [][]string) {
	var outgoingMessageChannel [][]chan metricMessage
	messagesSent := 0
	numberOfPools := len(outgoingHostPort)
	var numberOutConnections []int
	for currentPool := 0; currentPool < numberOfPools; currentPool++ {
		numberOutConnections = append(numberOutConnections, len(outgoingHostPort[currentPool]))
		// Create pool of outgoing connections
		var emptySlice []chan metricMessage
		outgoingMessageChannel = append(outgoingMessageChannel, emptySlice)
		for connectionInPool := 0; connectionInPool < numberOutConnections[currentPool]; connectionInPool++ {
			outgoingMessageChannel[currentPool] = append(outgoingMessageChannel[currentPool], make(chan metricMessage, OutgoingChannelSize))
			go createOutgoingConnection(outgoingHostPort[currentPool][connectionInPool], outgoingMessageChannel[currentPool][connectionInPool])
		}
	}

	for {
		fromConnection := <-outgoingToPoolChannel
		for currentPool := 0; currentPool < numberOfPools; currentPool++ {
			writeToOutConnection(outgoingMessageChannel[currentPool][messagesSent%numberOutConnections[currentPool]], fromConnection)
		}
		messagesSent++
	}
}

// mungeClusterNodesDestinations processes a list of addresses for cluster nodes, for outputting to several different graphite clusters
// Examples of valid output addresses:
// * 1234 (translates to 127.0.0.1:1234)
// * :4565 (translates to 127.0.0.1:4565)
// * 23.41.31.1:4565
// * sillyhostname23.sillyhostnamesrus.com:4565
func mungeClusterNodesDestinations(outgoingDestination []string) error {
	hostPortPattern := regexp.MustCompile(`^(?:([a-z0-9][a-z0-9.-]*)?:)?(\d+)$`)
	for outgoingIndex, outNode := range outgoingDestination {
		nodePatternCapture := hostPortPattern.FindStringSubmatch(strings.ToLower(outNode))

		if nodePatternCapture != nil {
			var hostname string
			if nodePatternCapture[1] == "" { // A blank hostname will imply the local host
				hostname = "127.0.0.1"
			} else {
				hostname = nodePatternCapture[1]
			}

			// Verify that TCP port is within a valid interval
			portNumberInteger, _ := strconv.ParseInt(nodePatternCapture[2], 10, 64)
			var portNumber string
			if portNumberInteger > 65535 {
				return errors.New("Port number too big: \"" + nodePatternCapture[2] + "\"")
			}
			portNumber = nodePatternCapture[2]

			// Verify that the hostname can be resolved
			if _, err := net.LookupIP(hostname); err == nil {
				outgoingDestination[outgoingIndex] = hostname + ":" + portNumber
			} else {
				return errors.New("Invalid hostname: \"" + hostname + "\"")
			}
		} else {
			return errors.New("Invalid host:port supplied: \"" + outNode + "\"")
		}
	}
	return nil
}
