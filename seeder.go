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

	// Twister Magic number to make it incompatible with the Bitcoin network
	TWISTNET = 0xd2bbdaf0
	// nounce is used to check if we connect to ourselves
	// as we don't listen we can use a fixed value
	NOUNCE  = 0x0539a019ca550825
	PVER    = 70003
	MINPORT = 0
	MAXPORT = 65535

	TWSTDPORT = 28333 // standard port twister listens on

	MAXFAILS = 48 // max number of connect fails before we delete a twistee. Just over 24 hours(checked every 33 minutes)

	MAXTO = 250 // max seconds (4min 10 sec) for all comms to twistee to complete before we timeout

	// DNS Type. Is this twistee using v4/v6 and standard or non standard ports
	DNSV4STD = 1
	DNSV4NON = 2
	DNSV6STD = 3
	DNSV6NON = 4

	// twistee status
	statusRG = 1 // reported good status. A remote twistee has reported this ip but we have not connected
	statusCG = 2 // confirmed good. We have connected to the twistee and received addresses
	statusWG = 3 // was good. Twistee was confirmed good but now having problems
	statusNG = 4 // no good. Will be removed from theList after 24 hours to redure bouncing ip addresses

)

type Seeder struct {
	uptime  time.Time
	theList map[string]*Twistee
	mtx     sync.RWMutex
}

// Twistee struct contains details on one twister client
type Twistee struct {
	na               *wire.NetAddress
	lastConnect      time.Time
	lastTry          time.Time
	crawlStart       time.Time
	statusTime       time.Time
	crawlActive      bool
	connectFails     uint32
	clientVersion    int32
	clientSubVersion string
	statusStr        string
	status           uint32 // rg,cg,wg,ng
	rating           uint32 // if it reaches 100 then we ban them
	nonstdIP         net.IP
	dnsType          uint32
}

// initCrawlers needs to be run before the startCrawlers so it can get
// a list of current ip addresses from the other seeders and therefore
// start the crawl process
func initCrawlers() {

	seeders := []string{"seed2.twister.net.co", "seed3.twister.net.co", "seed.twister.net.co"}

	for _, seeder := range seeders {
		c := 0

		newRRs, err := net.LookupHost(seeder)
		if err != nil {
			log.Printf("status - unable to do initial lookup to seeder %s %v\n", seeder, err)
			continue
		}

		for _, ip := range newRRs {
			if newIP := net.ParseIP(ip); newIP != nil {
				// 1 at the end is the services flag
				if x := config.seeder.addNa(wire.NewNetAddressIPPort(newIP, 28333, 1)); x == true {
					c++
				}
			}
		}
		if config.verbose {
			log.Printf("status - completed import of %v addresses from %s\n", c, seeder)
		}
	}
}

// startCrawlers is called on a time basis to start maxcrawlers new
// goroutines if there are spare goroutine slots available
func (s *Seeder) startCrawlers() {

	tcount := len(s.theList)
	if tcount == 0 {
		if config.debug {
			log.Printf("debug - startCrawlers fail: no twistees available\n", tcount)
		}
		return
	}

	// select the twistees to crawl up to max goroutines.

	// struct to hold config options for each status
	var crawlers = []struct {
		desc       string
		status     uint32
		maxCount   uint32 // max goroutines to start for this status type
		totalCount uint32 // stats count of this type
		started    uint32 // count of goroutines started for this type
		delay      int64  // number of second since last try
	}{
		{"statusRG", statusRG, 10, 0, 0, 184},
		{"statusCG", statusCG, 10, 0, 0, 325},
		{"statusWG", statusWG, 10, 0, 0, 237},
		{"statusNG", statusNG, 20, 0, 0, 1876},
	}

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	for _, c := range crawlers {

		// range on a map will not return items in the same order each time
		// not the best method to randomly pick twistees to crawl. FIXME
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

			// all looks go so start a go routine to crawl the remote twistee
			go crawlTwistee(tw)
			c.started++
		}

		if config.verbose || config.stats {
			log.Printf("stats - started crawler: %s total: %v started: %v\n", c.desc, c.totalCount, c.started)
		}
	}

	if config.verbose || config.stats {
		log.Printf("stats - completed starting crawlers. total twistees: %d\n", tcount)
	}

	// returns and read lock released
}

// isDup will return true or false depending if the ip exists in theList
func (s *Seeder) isDup(ipport string) bool {
	s.mtx.RLock()
	_, dup := s.theList[ipport]
	s.mtx.RUnlock()
	return dup
}

// isNaDup returns true if this wire.NetAddress is already known to us
func (s *Seeder) isNaDup(na *wire.NetAddress) bool {
	return s.isDup(net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port))))
}

// addNa validates and adds a network address to theList
func (s *Seeder) addNa(nNa *wire.NetAddress) bool {

	if dup := s.isNaDup(nNa); dup == true {
		return false
	}
	if nNa.Port <= MINPORT || nNa.Port >= MAXPORT {
		return false
	}

	// if the reported timestamp suggests the netaddress has not been seen in the last 24 hours
	// then ignore this netaddress
	if (time.Now().Add(-(time.Hour * 24))).After(nNa.Timestamp) {
		return false
	}

	nt := Twistee{
		na:            nNa,
		lastConnect:   time.Now(),
		clientVersion: 0, // FIXME - need to get from the crawl somehow
		status:        statusRG,
		statusTime:    time.Now(),
		dnsType:       DNSV4STD,
	}

	// select the dns type based on the remote address type and port
	if x := nt.na.IP.To4(); x == nil {
		// not ipv4
		if nNa.Port != TWSTDPORT {
			nt.dnsType = DNSV6NON

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)

		} else {
			nt.dnsType = DNSV6STD
		}
	} else {
		// ipv4
		if nNa.Port != TWSTDPORT {
			nt.dnsType = DNSV4NON

			// force ipv4 address into a 4 byte buffer
			nt.na.IP = nt.na.IP.To4()

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)
		}
	}

	// generate the key and add to theList
	k := net.JoinHostPort(nNa.IP.String(), strconv.Itoa(int(nNa.Port)))
	s.mtx.Lock()
	// final check to make sure another twistee & goroutine has not already added this twistee
	// FIXME migrate to use channels
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
		log.Printf("debug encode nonstd - realip: %s port: %v encip: %s crc: %x\n", rip.String(), port, encip.String(), crcAddr)
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

func (s *Seeder) auditTwistees() {

	c := 0
	log.Printf("status - Audit start. System Uptime: %s\n", time.Since(s.uptime).String())

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
		if tw.status == statusRG || tw.status == statusWG {
			if time.Now().Unix()-tw.statusTime.Unix() >= 900 {
				log.Printf("warning - unchanged status > 15 minutes ====\n- %s status:rating:fails %v:%v:%v last status change: %s last status: %s\n====\n",
					k,
					tw.status,
					tw.rating,
					tw.connectFails,
					tw.statusTime.String(),
					tw.statusStr)
			}
		}

		// last audit task is to remove twistees that we can not connect to
		if tw.status == statusNG && tw.connectFails > MAXFAILS {
			if config.verbose {
				log.Printf("status - purging twistee %s after %v failed connections\n", k, tw.connectFails)
			}

			c++
			// remove the map entry and mark the old twistee as
			// nil so garbage collector will remove it
			s.theList[k] = nil
			delete(s.theList, k)
		}

	}
	if config.verbose {
		log.Printf("status - Audit complete. %v twistees purged\n", c)
	}

}

// teatload loads the dns records with time based test data
func (s *Seeder) loadDNS() {

	updateDNS(s)
}

/*

 */
