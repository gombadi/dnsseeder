/*

 */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

type dnsseeder struct {
	host    string
	port    string
	verbose bool
	debug   bool
	stats   bool
	seeder  *Seeder
}

var config dnsseeder

func main() {

	flag.StringVar(&config.host, "h", "", "DNS host to serve")
	flag.StringVar(&config.port, "p", "8053", "Port to listen on")
	flag.BoolVar(&config.verbose, "v", false, "Display verbose output")
	flag.BoolVar(&config.debug, "d", false, "Display debug output")
	flag.BoolVar(&config.stats, "s", false, "Display stats output")
	flag.Parse()

	if config.host == "" {
		log.Fatalf("error - no hostname provided\n")
	}

	if config.debug == true {
		config.verbose = true
		config.stats = true
	}
	if config.verbose == true {
		config.stats = true
	}

	log.Printf("Starting dnsseeder system for host %s.\n", config.host)

	// FIXME - setup/make the data structures in Seeder
	config.seeder = &Seeder{}
	config.seeder.theList = make(map[string]*Twistee)

	// start dns server
	dns.HandleFunc("nonstd."+config.host, handleDNSNon)
	dns.HandleFunc(config.host, handleDNSStd)
	go serve("udp", config.port)
	//go serve("tcp", config.port)

	// seed the seeder with some ip addresses
	initCrawlers()
	// start first crawl
	config.seeder.startCrawlers()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// extract good dns records from all twistees on regular basis
	dnsChan := time.NewTicker(time.Second * 57).C
	// used to start crawlers on a regular basis
	crawlChan := time.NewTicker(time.Second * 22).C
	// used to remove old statusNG twistees that have reached fail count
	purgeChan := time.NewTicker(time.Hour * 2).C

	dowhile := true
	for dowhile == true {
		select {
		case <-sig:
			dowhile = false
		case <-purgeChan:
			if config.debug {
				log.Printf("debug - purge old statusNG twistees timer triggered\n")
			}
			config.seeder.purgeNG()
		case <-dnsChan:
			if config.debug {
				log.Printf("debug - DNS - Updating latest ip addresses timer triggered\n")
			}
			config.seeder.loadDNS()
		case <-crawlChan:
			if config.debug {
				log.Printf("debug - start crawlers timer triggered\n")
			}
			config.seeder.startCrawlers()
		}
	}
	// FIXME - call dns server.Shutdown()
	fmt.Printf("\nProgram exiting. Bye\n")
}

/*

 */
