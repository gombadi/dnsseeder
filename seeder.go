package main

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/btcsuite/btcd/wire"
)

const (

	// NOUNCE is used to check if we connect to ourselves
	// as we don't listen we can use a fixed value
	nounce  = 0x0539a019ca550825
	minPort = 0
	maxPort = 65535

	crawlDelay = 22 // seconds between start crawlwer ticks
	auditDelay = 22 // minutes between audit channel ticks
	dnsDelay   = 57 // seconds between updates to active dns record list

	maxFails = 58 // max number of connect fails before we delete a twistee. Just over 24 hours(checked every 33 minutes)

	maxTo = 250 // max seconds (4min 10 sec) for all comms to twistee to complete before we timeout
)

const (
	dnsInvalid  = iota //
	dnsV4Std           // ip v4 using network standard port
	dnsV4Non           // ip v4 using network non standard port
	dnsV6Std           // ipv6 using network standard port
	dnsV6Non           // ipv6 using network non standard port
	maxDNSTypes        // used in main to allocate slice
)

const (
	// twistee status
	statusRG       = iota // reported good status. A remote twistee has reported this ip but we have not connected
	statusCG              // confirmed good. We have connected to the twistee and received addresses
	statusWG              // was good. Twistee was confirmed good but now having problems
	statusNG              // no good. Will be removed from theList after 24 hours to redure bouncing ip addresses
	maxStatusTypes        // used in main to allocate slice
)

type dnsseeder struct {
	net     *network            // network struct with config options for this network
	uptime  time.Time           // as the name says
	theList map[string]*twistee // the list of current clients
	mtx     sync.RWMutex
}

// initCrawlers needs to be run before the startCrawlers so it can get
// a list of current ip addresses from the other seeders and therefore
// start the crawl process
func (s *dnsseeder) initCrawlers() {

	// get a list of permenant seeders
	seeders := s.net.seeders

	for _, aseeder := range seeders {
		c := 0

		newRRs, err := net.LookupHost(aseeder)
		if err != nil {
			log.Printf("status - unable to do initial lookup to seeder %s %v\n", aseeder, err)
			continue
		}

		for _, ip := range newRRs {
			if newIP := net.ParseIP(ip); newIP != nil {
				// 1 at the end is the services flag
				if x := config.seeder.addNa(wire.NewNetAddressIPPort(newIP, s.net.port, 1)); x == true {
					c++
				}
			}
		}
		if config.verbose {
			log.Printf("status - completed import of %v addresses from %s\n", c, aseeder)
		}
	}
}

// startCrawlers is called on a time basis to start maxcrawlers new
// goroutines if there are spare goroutine slots available
func (s *dnsseeder) startCrawlers() {

	tcount := len(s.theList)
	if tcount == 0 {
		if config.debug {
			log.Printf("debug - startCrawlers fail: no twistees available\n")
		}
		return
	}

	// struct to hold config options for each status
	var crawlers = []struct {
		desc       string
		status     uint32
		maxCount   uint32 // max goroutines to start for this status type
		totalCount uint32 // stats count of this type
		started    uint32 // count of goroutines started for this type
		delay      int64  // number of second since last try
	}{
		{"statusRG", statusRG, s.net.maxStart[statusRG], 0, 0, s.net.delay[statusRG]},
		{"statusCG", statusCG, s.net.maxStart[statusCG], 0, 0, s.net.delay[statusCG]},
		{"statusWG", statusWG, s.net.maxStart[statusWG], 0, 0, s.net.delay[statusWG]},
		{"statusNG", statusNG, s.net.maxStart[statusNG], 0, 0, s.net.delay[statusNG]},
	}

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	// step through each of the status types RG, CG, WG, NG
	for _, c := range crawlers {

		// range on a map will not return items in the same order each time
		// so this is a random'ish selection
		for _, tw := range s.theList {

			if tw.status != c.status {
				continue
			}

			// stats count
			c.totalCount++

			if tw.crawlActive == true {
				continue
			}

			if c.started >= c.maxCount {
				continue
			}

			if (time.Now().Unix() - c.delay) <= tw.lastTry.Unix() {
				continue
			}

			// all looks good so start a go routine to crawl the remote twistee
			go crawlTwistee(s, tw)
			c.started++
		}

		log.Printf("stats - started crawler: %s total: %v started: %v\n", c.desc, c.totalCount, c.started)

		// update the global stats in another goroutine to free the main goroutine
		// for other work
		go updateTwCounts(c.status, c.totalCount, c.started)
	}

	log.Printf("stats - crawlers started. total twistees: %d\n", tcount)

	// returns and read lock released
}

// isDup will return true or false depending if the ip exists in theList
func (s *dnsseeder) isDup(ipport string) bool {
	s.mtx.RLock()
	_, dup := s.theList[ipport]
	s.mtx.RUnlock()
	return dup
}

// isNaDup returns true if this wire.NetAddress is already known to us
func (s *dnsseeder) isNaDup(na *wire.NetAddress) bool {
	return s.isDup(net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port))))
}

// addNa validates and adds a network address to theList
func (s *dnsseeder) addNa(nNa *wire.NetAddress) bool {

	// as this is run in many different goroutines then they may all try and
	// add new addresses so do a final check
	if s.isFull() {
		return false
	}

	if dup := s.isNaDup(nNa); dup == true {
		return false
	}
	if nNa.Port <= minPort || nNa.Port >= maxPort {
		return false
	}

	// if the reported timestamp suggests the netaddress has not been seen in the last 24 hours
	// then ignore this netaddress
	if (time.Now().Add(-(time.Hour * 24))).After(nNa.Timestamp) {
		return false
	}

	nt := twistee{
		na:          nNa,
		lastConnect: time.Now(),
		version:     0,
		status:      statusRG,
		statusTime:  time.Now(),
		dnsType:     dnsV4Std,
	}

	// select the dns type based on the remote address type and port
	if x := nt.na.IP.To4(); x == nil {
		// not ipv4
		if nNa.Port != s.net.port {
			nt.dnsType = dnsV6Non

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)

		} else {
			nt.dnsType = dnsV6Std
		}
	} else {
		// ipv4
		if nNa.Port != s.net.port {
			nt.dnsType = dnsV4Non

			// force ipv4 address into a 4 byte buffer
			nt.na.IP = nt.na.IP.To4()

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)
		}
	}

	// generate the key and add to theList
	k := net.JoinHostPort(nNa.IP.String(), strconv.Itoa(int(nNa.Port)))
	s.mtx.Lock()
	// final check to make sure another crawl & goroutine has not already added this client
	if _, dup := s.theList[k]; dup == false {
		s.theList[k] = &nt
	}
	s.mtx.Unlock()

	return true
}

// getNonStdIP is given an IP address and a port and returns a fake IP address
// that is encoded with the original IP and port number. Remote clients can match
// the two and work out the real IP and port from the two IP addresses.
func getNonStdIP(rip net.IP, port uint16) net.IP {

	b := []byte{0x0, 0x0, 0x0, 0x0}
	crcAddr := crc16(rip)
	b[0] = byte(crcAddr >> 8)
	b[1] = byte((crcAddr & 0xff))
	b[2] = byte(port >> 8)
	b[3] = byte(port & 0xff)

	encip := net.IPv4(b[0], b[1], b[2], b[3])
	if config.debug {
		log.Printf("debug - encode nonstd - realip: %s port: %v encip: %s crc: %x\n", rip.String(), port, encip.String(), crcAddr)
	}

	return encip
}

// crc16 produces a crc16 from a byte slice
func crc16(bs []byte) uint16 {
	var x, crc uint16
	crc = 0xffff

	for _, v := range bs {
		x = crc>>8 ^ uint16(v)
		x ^= x >> 4
		crc = (crc << 8) ^ (x << 12) ^ (x << 5) ^ x
	}
	return crc
}

func (s *dnsseeder) auditClients() {

	c := 0

	// set this early so for this audit run all NG clients will be purged
	// and space will be made for new, possible CG clients
	iAmFull := s.isFull()

	// cgGoal is 75% of the max statusCG clients we can crawl with the current network delay & maxStart settings.
	// This allows us to cycle statusCG users to keep the list fresh
	cgGoal := int(float64(float64(s.net.delay[statusCG]/crawlDelay)*float64(s.net.maxStart[statusCG])) * 0.75)
	cgCount := 0

	log.Printf("status - Audit start. statusCG Goal: %v System Uptime: %s\n", cgGoal, time.Since(s.uptime).String())

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for k, tw := range s.theList {

		if tw.crawlActive == true {
			if time.Now().Unix()-tw.crawlStart.Unix() >= 300 {
				log.Printf("warning - long running crawl > 5 minutes ====\n- %s status:rating:fails %v:%v:%v crawl start: %s last status: %s\n====\n",
					k,
					tw.status,
					tw.rating,
					tw.connectFails,
					tw.crawlStart.String(),
					tw.statusStr)
			}
		}

		// Audit task is to remove clients that we have not been able to connect to
		if tw.status == statusNG && tw.connectFails > maxFails {
			if config.verbose {
				log.Printf("status - purging twistee %s after %v failed connections\n", k, tw.connectFails)
			}

			c++
			// remove the map entry and mark the old twistee as
			// nil so garbage collector will remove it
			s.theList[k] = nil
			delete(s.theList, k)
		}

		// If seeder is full then remove old NG clients and fill up with possible new CG clients
		if tw.status == statusNG && iAmFull {
			if config.verbose {
				log.Printf("status - seeder full purging twistee %s\n", k)
			}

			c++
			// remove the map entry and mark the old twistee as
			// nil so garbage collector will remove it
			s.theList[k] = nil
			delete(s.theList, k)
		}

		// check if we need to purge statusCG to freshen the list
		if tw.status == statusCG {
			if cgCount++; cgCount > cgGoal {
				// we have enough statusCG clients so purge remaining to cycle through the list
				if config.verbose {
					log.Printf("status - seeder cycle statusCG - purging client %s\n", k)
				}

				c++
				// remove the map entry and mark the old twistee as
				// nil so garbage collector will remove it
				s.theList[k] = nil
				delete(s.theList, k)
			}

		}

	}
	if config.verbose {
		log.Printf("status - Audit complete. %v twistees purged\n", c)
	}

}

// teatload loads the dns records with time based test data
func (s *dnsseeder) loadDNS() {
	updateDNS(s)
}

// isFull returns true if the number of remote clients is more than we want to store
func (s *dnsseeder) isFull() bool {
	if len(s.theList) > s.net.maxSize {
		return true
	}
	return false
}

/*

 */
