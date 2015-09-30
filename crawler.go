package main

import (
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/wire"
)

type crawlError struct {
	errLoc string
	Err    error
}

// Error returns a formatted error about a crawl
func (e *crawlError) Error() string {
	return "err: " + e.errLoc + ": " + e.Err.Error()
}

// crawlNode runs in a goroutine, crawls the remote ip and updates the master
// list of currently active addresses
func crawlNode(rc chan *result, s *dnsseeder, nd *node) {

	// connect to the remote ip and ask them for their addr list
	rna, e := crawlIP(s, nd)

	res := &result{
		nas:  rna,
		msg:  e,
		node: net.JoinHostPort(nd.na.IP.String(), strconv.Itoa(int(nd.na.Port))),
	}

	if e != nil {
		res.success = true
	}

	// all done so push the result back to the seeder.
	//This will block until the seeder reads the result
	rc <- res

	// goroutine will end and be cleaned up
}

// crawlIP retrievs a slice of ip addresses from a client
func crawlIP(s *dnsseeder, nd *node) ([]*wire.NetAddress, *crawlError) {

	ip := nd.na.IP.String()
	port := strconv.Itoa(int(nd.na.Port))

	// get correct formatting for ipv6 addresses
	dialString := net.JoinHostPort(ip, port)

	if config.debug {
		log.Printf("%s - debug - start crawl: node %s status: %v:%v lastcrawl: %s\n",
			s.name,
			dialString,
			nd.status,
			nd.rating,
			time.Since(nd.crawlStart).String())
	}

	conn, err := net.DialTimeout("tcp", dialString, time.Second*10)
	if err != nil {
		if config.debug {
			log.Printf("%s - debug - Could not connect to %s - %v\n", s.name, ip, err)
		}
		return nil, &crawlError{"", err}
	}

	defer conn.Close()
	if config.debug {
		log.Printf("%s - debug - Connected to remote address: %s Last connect was %v ago\n", s.name, ip, time.Since(nd.lastConnect).String())
	}

	// set a deadline for all comms to be done by. After this all i/o will error
	conn.SetDeadline(time.Now().Add(time.Second * maxTo))

	// First command to remote end needs to be a version command
	// last parameter is lastblock
	msgver, err := wire.NewMsgVersionFromConn(conn, nounce, 0)
	if err != nil {
		return nil, &crawlError{"Create NewMsgVersionFromConn", err}
	}

	err = wire.WriteMessage(conn, msgver, s.pver, s.id)
	if err != nil {
		// Log and handle the error
		return nil, &crawlError{"Write Version Message", err}
	}

	// first message received should be version
	msg, _, err := wire.ReadMessage(conn, s.pver, s.id)
	if err != nil {
		// Log and handle the error
		return nil, &crawlError{"Read message after sending Version", err}
	}

	switch msg := msg.(type) {
	case *wire.MsgVersion:
		// The message is a pointer to a MsgVersion struct.
		if config.debug {
			log.Printf("%s - debug - %s - Remote version: %v\n", s.name, ip, msg.ProtocolVersion)
		}
		// fill the node struct with the remote details
		nd.version = msg.ProtocolVersion
		nd.services = msg.Services
		nd.lastBlock = msg.LastBlock
		if nd.strVersion != msg.UserAgent {
			// if the srtVersion is already the same then don't overwrite it.
			// saves the GC having to cleanup a perfectly good string
			nd.strVersion = msg.UserAgent
		}
	default:
		return nil, &crawlError{"Did not receive expected Version message from remote client", errors.New("")}
	}

	// send verack command
	msgverack := wire.NewMsgVerAck()

	err = wire.WriteMessage(conn, msgverack, s.pver, s.id)
	if err != nil {
		return nil, &crawlError{"writing message VerAck", err}
	}

	// second message received should be verack
	msg, _, err = wire.ReadMessage(conn, s.pver, s.id)
	if err != nil {
		return nil, &crawlError{"reading expected Ver Ack from remote client", err}
	}

	switch msg.(type) {
	case *wire.MsgVerAck:
		if config.debug {
			log.Printf("%s - debug - %s - received Version Ack\n", s.name, ip)
		}
	default:
		return nil, &crawlError{"Did not receive expected Ver Ack message from remote client", errors.New("")}
	}

	// if we get this far and if the seeder is full then don't ask for addresses. This will reduce bandwith usage while still
	// confirming that we can connect to the remote node
	if s.isFull() {
		return nil, nil
	}
	// send getaddr command
	msgGetAddr := wire.NewMsgGetAddr()

	err = wire.WriteMessage(conn, msgGetAddr, s.pver, s.id)
	if err != nil {
		return nil, &crawlError{"writing Addr message to remote client", err}
	}

	c := 0
	dowhile := true
	for dowhile == true {

		// Using the Bitcoin lib for the some networks means it does not understand some
		// of the commands and will error. We can ignore these as we are only
		// interested in the addr message and its content.
		msgaddr, _, _ := wire.ReadMessage(conn, s.pver, s.id)
		if msgaddr != nil {
			switch msg := msgaddr.(type) {
			case *wire.MsgAddr:
				// received the addr message so return the result
				if config.debug {
					log.Printf("%s - debug - %s - received valid addr message\n", s.name, ip)
				}
				dowhile = false
				return msg.AddrList, nil
			default:
				if config.debug {
					log.Printf("%s - debug - %s - ignoring message - %v\n", s.name, ip, msg.Command())
				}
			}
		}
		// if we get more than 25 messages before the addr we asked for then give up on this client
		if c++; c >= 25 {
			dowhile = false
		}
	}

	// received too many messages before requested Addr
	return nil, &crawlError{"message loop - did not receive remote addresses in first 25 messages from remote client", errors.New("")}
}

/*

 */
