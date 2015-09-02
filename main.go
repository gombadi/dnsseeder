/*

 */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

// twCounts holds various statistics about the running system
type twCounts struct {
	TwStatus  []uint32
	TwStarts  []uint32
	DNSCounts []uint32
	mtx       sync.RWMutex
}

// configData holds information on the application
type configData struct {
	host    string
	port    string
	http    string
	version string
	verbose bool
	debug   bool
	stats   bool
	seeder  *dnsseeder
}

var config configData
var counts twCounts
var nwname string

func main() {

	// FIXME - update with git hash during build
	config.version = "0.6.0"

	// initialize the stats counters
	counts.TwStatus = make([]uint32, maxStatusTypes)
	counts.TwStarts = make([]uint32, maxStatusTypes)
	counts.DNSCounts = make([]uint32, maxDNSTypes)

	flag.StringVar(&nwname, "net", "", "Preconfigured Network config")
	flag.StringVar(&config.host, "h", "", "DNS host to serve")
	flag.StringVar(&config.port, "p", "8053", "DNS Port to listen on")
	flag.StringVar(&config.http, "w", "", "Web Port to listen on. No port specified & no web server running")
	flag.BoolVar(&config.verbose, "v", false, "Display verbose output")
	flag.BoolVar(&config.debug, "d", false, "Display debug output")
	flag.BoolVar(&config.stats, "s", false, "Display stats output")
	flag.Parse()

	if config.host == "" {
		fmt.Printf("error - no hostname provided\n")
		os.Exit(1)
	}

	// configure the network options so we can start crawling
	thenet := selectNetwork(nwname)
	if thenet == nil {
		fmt.Printf("Error - No valid network specified. Please add -net=<network> from one of the following:\n")
		for _, n := range getNetworkNames() {
			fmt.Printf("%s\n", n)
		}
		os.Exit(1)
	}

	// init the seeder
	config.seeder = &dnsseeder{}
	config.seeder.theList = make(map[string]*twistee)
	config.seeder.uptime = time.Now()
	config.seeder.net = thenet

	if config.debug == true {
		config.verbose = true
		config.stats = true
	}
	if config.verbose == true {
		config.stats = true
	}

	log.Printf("Starting dnsseeder system for host %s.\n", config.host)

	if config.verbose == false {
		log.Printf("status - Running in quiet mode with limited output produced\n")
	} else {
		log.Printf("status - system is configured for %s\n", config.seeder.net.name)
	}

	// start the web interface if we want it running
	if config.http != "" {
		go startHTTP(config.http)
	}
	// start dns server
	dns.HandleFunc("nonstd."+config.host, handleDNSNon)
	dns.HandleFunc(config.host, handleDNSStd)
	go serve("udp", config.port)
	//go serve("tcp", config.port)

	// seed the seeder with some ip addresses
	config.seeder.initCrawlers()
	// start first crawl
	config.seeder.startCrawlers()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// extract good dns records from all twistees on regular basis
	dnsChan := time.NewTicker(time.Second * dnsDelay).C
	// used to start crawlers on a regular basis
	crawlChan := time.NewTicker(time.Second * crawlDelay).C
	// used to remove old statusNG twistees that have reached fail count
	auditChan := time.NewTicker(time.Minute * auditDelay).C

	dowhile := true
	for dowhile == true {
		select {
		case <-sig:
			dowhile = false
		case <-auditChan:
			if config.debug {
				log.Printf("debug - Audit twistees timer triggered\n")
			}
			config.seeder.auditClients()
		case <-dnsChan:
			if config.debug {
				log.Printf("debug - DNS - Updating latest ip addresses timer triggered\n")
			}
			config.seeder.loadDNS()
		case <-crawlChan:
			if config.debug {
				log.Printf("debug - Start crawlers timer triggered\n")
			}
			config.seeder.startCrawlers()
		}
	}
	// FIXME - call dns server.Shutdown()
	fmt.Printf("\nProgram exiting. Bye\n")
}

// updateTwCounts runs in a goroutine and updates the global stats with the lates
// counts from a startCrawlers run
func updateTwCounts(status, total, started uint32) {
	// update the stats counters
	counts.mtx.Lock()
	counts.TwStatus[status] = total
	counts.TwStarts[status] = started
	counts.mtx.Unlock()
}

// updateDNSCounts runs in a goroutine and updates the global stats for the number of DNS requests
func updateDNSCounts(dnsType uint32) {
	counts.mtx.Lock()
	counts.DNSCounts[dnsType]++
	counts.mtx.Unlock()
}

/*

 */
