package main

import (
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/wire"
)

// crawlTwistee runs in a goroutine, crawls the remote ip and updates the master
// list of currently active addresses
func crawlTwistee(tw *Twistee) {

	if config.debug {
		log.Printf("status - start crawl: twistee %s status: %v:%v lastcrawl: %s\n",
			net.JoinHostPort(tw.na.IP.String(),
				strconv.Itoa(int(tw.na.Port))),
			tw.status,
			tw.rating,
			time.Since(tw.crawlStart).String())
	}

	tw.crawlActive = true
	tw.crawlStart = time.Now()

	defer crawlEnd(tw)

	// connect to the remote ip and ask them for their addr list
	ras, err := crawlIP(tw)
	if err != nil {
		// update the fact that we have not connected to this twistee
		tw.lastTry = time.Now()
		tw.connectFails++
		// update the status of this failed twistee
		switch tw.status {
		case statusRG:
			if tw.rating += 25; tw.rating > 30 {
				tw.status = statusWG
			}
		case statusCG:
			if tw.rating += 25; tw.rating >= 50 {
				tw.status = statusWG
			}
		case statusWG:
			if tw.rating += 30; tw.rating >= 100 {
				tw.status = statusNG // not able to connect to this twistee so ignore
			}
		}
		// no more to do so return which will shutdown the goroutine & call
		// the deffered cleanup
		if config.verbose {
			log.Printf("debug - failed crawl: twistee %s failcount: %v newstatus: %v:%v\n",
				net.JoinHostPort(tw.na.IP.String(),
					strconv.Itoa(int(tw.na.Port))),
				tw.connectFails,
				tw.status,
				tw.rating)
		}
		return
	}

	// succesful connection and addresses received so mark status
	if tw.status != statusCG {
		tw.status = statusCG
	}
	tw.rating = 0
	tw.connectFails = 0
	tw.lastConnect = time.Now()
	tw.lastTry = time.Now()

	added := 0

	// loop through all the received network addresses and add to thelist if not present
	for _, na := range ras {
		// a new network address so add to the system
		if x := config.seeder.addNa(na); x == true {
			added++
		}
	}

	if config.verbose {
		log.Printf("status - crawl done: twistee: %s newstatus: %v.%v received_addr: %v added_addr: %v CrawlTime: %s\n",
			net.JoinHostPort(tw.na.IP.String(),
				strconv.Itoa(int(tw.na.Port))),
			tw.status,
			tw.rating,
			len(ras),
			added,
			time.Since(tw.crawlStart).String())
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
func crawlIP(tw *Twistee) ([]*wire.NetAddress, error) {

	ip := tw.na.IP.String()
	port := strconv.Itoa(int(tw.na.Port))
	// get correct formatting for ipv6 addresses
	dialString := net.JoinHostPort(ip, port)

	conn, err := net.DialTimeout("tcp", dialString, time.Second*10)
	if err != nil {
		if config.debug {
			log.Printf("error - Could not connect to %s - %v\n", ip, err)
		}
		return nil, err
	}

	defer conn.Close()
	if config.verbose {
		log.Printf("%s - Connected to remote address. Last connect was %v ago\n", ip, time.Since(tw.lastConnect).String())
	}

	// First command to remote end needs to be a version command
	// last parameter is lastblock
	msgver, err := wire.NewMsgVersionFromConn(conn, NOUNCE, 0)
	if err != nil {
		log.Printf("error - NewMsgVer from conn: %v\n", err)
		return nil, err
	}

	err = wire.WriteMessage(conn, msgver, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		log.Printf("error - %s:%s Write Message: %v\n", ip, port, err)
		return nil, err
	}

	// first message received should be version
	msg, _, err := wire.ReadMessage(conn, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		log.Printf("error - %s:%s Read Message after sending version: %v\n", ip, port, err)
		return nil, err
	}

	switch msg := msg.(type) {
	case *wire.MsgVersion:
		// The message is a pointer to a MsgVersion struct.
		if config.debug {
			log.Printf("%s - Remote version: %v\n", ip, msg.ProtocolVersion)
		}
	default:
		if config.debug {
			log.Printf("error: expected Version Message but received: %v\n", msg.Command())
		}
		return nil, errors.New("Error. Did not receive expected Version message from remote client")
	}

	// FIXME - update twistee client version with what they just said

	// send verack command
	msgverack := wire.NewMsgVerAck()

	err = wire.WriteMessage(conn, msgverack, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		log.Printf("error - %s:%s Writing Message Ver Ack failed: %v\n", ip, port, err)
		return nil, err
	}

	// second message received should be verack
	msg, _, err = wire.ReadMessage(conn, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		log.Printf("error - %s:%s Reading Message expected Ver Ack: %v\n", ip, port, err)
		return nil, err
	}

	switch msg := msg.(type) {
	case *wire.MsgVerAck:
		if config.debug {
			// The message is a pointer to a MsgVersion struct.
			log.Printf("%s - received Version Ack\n", ip)
		}
	default:
		if config.debug {
			log.Printf("error: expected Version Ack Message but received: %v\n", msg.Command())
		}
		return nil, errors.New("Error. Did not receive expected Version Ack message from remote client")
	}

	// send getaddr command
	msgGetAddr := wire.NewMsgGetAddr()

	err = wire.WriteMessage(conn, msgGetAddr, PVER, TWISTNET)
	if err != nil {
		// Log and handle the error
		log.Printf("error - %s:%s writing message Get Addr: %v\n", ip, port, err)
		return nil, err
	}

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
				return msg.AddrList, nil
				dowhile = false
			default:
				if config.debug {
					log.Printf("%s - ignoring message - %v\n", ip, msg.Command())
				}
			}
		}
	}

	// should never get here but need a return command
	return nil, nil
}

/*

 */
