package main

import (
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/wire"
)

type CrawlError struct {
	errLoc string
	Err    error
}

// Error returns a formatted error about a crawl
func (e *CrawlError) Error() string {
	return "err: " + e.errLoc + ": " + e.Err.Error()
}

// crawlTwistee runs in a goroutine, crawls the remote ip and updates the master
// list of currently active addresses
func crawlTwistee(tw *Twistee) {

	tw.crawlActive = true
	tw.crawlStart = time.Now()

	defer crawlEnd(tw)

	if config.debug {
		log.Printf("status - start crawl: twistee %s status: %v:%v lastcrawl: %s\n",
			net.JoinHostPort(tw.na.IP.String(),
				strconv.Itoa(int(tw.na.Port))),
			tw.status,
			tw.rating,
			time.Since(tw.crawlStart).String())
	}

	// connect to the remote ip and ask them for their addr list
	ras, e := crawlIP(tw)
	if e != nil {
		// update the fact that we have not connected to this twistee
		tw.lastTry = time.Now()
		tw.connectFails++
		tw.statusStr = e.Error()

		// update the status of this failed twistee
		switch tw.status {
		case statusRG:
			if tw.rating += 25; tw.rating > 30 {
				tw.status = statusWG
				tw.statusTime = time.Now()
			}
		case statusCG:
			if tw.rating += 25; tw.rating >= 50 {
				tw.status = statusWG
				tw.statusTime = time.Now()
			}
		case statusWG:
			if tw.rating += 30; tw.rating >= 100 {
				tw.status = statusNG // not able to connect to this twistee so ignore
				tw.statusTime = time.Now()
			}
		}
		// no more to do so return which will shutdown the goroutine & call
		// the deffered cleanup
		if config.verbose {
			log.Printf("debug - failed crawl: twistee %s s:r:f: %v:%v:%v %s\n",
				net.JoinHostPort(tw.na.IP.String(),
					strconv.Itoa(int(tw.na.Port))),
				tw.status,
				tw.rating,
				tw.connectFails,
				tw.statusStr)
		}
		return
	}

	// succesful connection and addresses received so mark status
	if tw.status != statusCG {
		tw.status = statusCG
		tw.statusTime = time.Now()
	}
	cs := tw.lastConnect
	tw.rating = 0
	tw.connectFails = 0
	tw.lastConnect = time.Now()
	tw.lastTry = time.Now()
	tw.statusStr = "ok: received remote address list"

	added := 0

	// loop through all the received network addresses and add to thelist if not present
	for _, na := range ras {
		// a new network address so add to the system
		if x := config.seeder.addNa(na); x == true {
			added++
		}
	}

	if config.verbose {
		log.Printf("status - crawl done: twistee: %s s:r:f: %v:%v:%v addr: %v:%v CrawlTime: %s Last connect: %v ago\n",
			net.JoinHostPort(tw.na.IP.String(),
				strconv.Itoa(int(tw.na.Port))),
			tw.status,
			tw.rating,
			tw.connectFails,
			len(ras),
			added,
			time.Since(tw.crawlStart).String(),
			time.Since(cs).String())
	}

	// goroutine ends. deffered cleanup runs
}

// crawlEnd is a deffered func to update theList after a crawl is all done
func crawlEnd(tw *Twistee) {
	tw.crawlActive = false
	// FIXME - scan for long term crawl active twistees. Dial timeout is 10 seconds
	// so should be done in under 5 minutes
}

// crawlIP retrievs a slice of ip addresses from a client
func crawlIP(tw *Twistee) ([]*wire.NetAddress, *CrawlError) {

	ip := tw.na.IP.String()
	port := strconv.Itoa(int(tw.na.Port))
	// get correct formatting for ipv6 addresses
	dialString := net.JoinHostPort(ip, port)

	conn, err := net.DialTimeout("tcp", dialString, time.Second*10)
	if err != nil {
		if config.debug {
			log.Printf("error - Could not connect to %s - %v\n", ip, err)
		}
		return nil, &CrawlError{"", err}
	}

	defer conn.Close()
	if config.debug {
		log.Printf("%s - Connected to remote address. Last connect was %v ago\n", ip, time.Since(tw.lastConnect).String())
	}

	// set a deadline for all comms to be done by. After this all i/o will error
	conn.SetDeadline(time.Now().Add(time.Second * MAXTO))

	// First command to remote end needs to be a version command
	// last parameter is lastblock
	msgver, err := wire.NewMsgVersionFromConn(conn, NOUNCE, 0)
	if err != nil {
		return nil, &CrawlError{"Create NewMsgVersionFromConn", err}
	}

	err = wire.WriteMessage(conn, msgver, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		return nil, &CrawlError{"Write Version Message", err}
	}

	// first message received should be version
	msg, _, err := wire.ReadMessage(conn, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		return nil, &CrawlError{"Read message after sending Version", err}
	}

	switch msg := msg.(type) {
	case *wire.MsgVersion:
		// The message is a pointer to a MsgVersion struct.
		if config.debug {
			log.Printf("%s - Remote version: %v\n", ip, msg.ProtocolVersion)
		}
	default:
		return nil, &CrawlError{"Did not receive expected Version message from remote client", errors.New("")}
	}

	// FIXME - update twistee client version with what they just said

	// send verack command
	msgverack := wire.NewMsgVerAck()

	err = wire.WriteMessage(conn, msgverack, PVER, TWISTNET)
	if err != nil {
		return nil, &CrawlError{"writing message VerAck", err}
	}

	// second message received should be verack
	msg, _, err = wire.ReadMessage(conn, PVER, TWISTNET)
	if err != nil {
		return nil, &CrawlError{"reading expected Ver Ack from remote client", err}
	}

	switch msg.(type) {
	case *wire.MsgVerAck:
		if config.debug {
			log.Printf("%s - received Version Ack\n", ip)
		}
	default:
		return nil, &CrawlError{"Did not receive expected Ver Ack message from remote client", errors.New("")}
	}

	// send getaddr command
	msgGetAddr := wire.NewMsgGetAddr()

	err = wire.WriteMessage(conn, msgGetAddr, PVER, TWISTNET)
	if err != nil {
		return nil, &CrawlError{"writing Addr message to remote client", err}
	}

	c := 0
	dowhile := true
	for dowhile == true {

		// Using the Bitcoin lib for the Twister Net means it does not understand some
		// of the commands and will error. We can ignore these as we are only
		// interested in the addr message and its content.
		msgaddr, _, _ := wire.ReadMessage(conn, PVER, TWISTNET)
		if msgaddr != nil {
			switch msg := msgaddr.(type) {
			case *wire.MsgAddr:
				// received the addrt message so return the result
				if config.debug {
					log.Printf("%s - received valid addr message\n", ip)
				}
				dowhile = false
				return msg.AddrList, nil
			default:
				if config.debug {
					log.Printf("%s - ignoring message - %v\n", ip, msg.Command())
				}
			}
		}
		// if we get more than 25 messages before the addr we asked for then give up on this client
		if c++; c >= 25 {
			dowhile = false
		}
	}

	// received too many messages before requested Addr
	return nil, &CrawlError{"message loop - did not receive remote addresses in first 25 messages from remote client", errors.New("")}
}

/*

 */
