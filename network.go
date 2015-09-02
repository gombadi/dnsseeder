package main

import (
	"github.com/btcsuite/btcd/wire"
)

// network struct holds config details for the network the seeder is using
type network struct {
	id          wire.BitcoinNet // Magic number - Unique ID for this network. Sent in header of all messages
	maxSize     int             // max number of clients before we start restricting new entries
	port        uint16          // default network port this network uses
	pver        uint32          // minimum block height for the network
	ttl         uint32          // DNS TTL to use for this network
	name        string          // Short name for the network
	description string          // Long description for the network
	seeders     []string        // slice of seeders to pull ip addresses when starting this seeder
	maxStart    []uint32        // max number of goroutines to start each run for each status type
	delay       []int64         // number of seconds to wait before we connect to a known client for each status
}

// getNetworkNames returns a slice of the networks that have been configured
func getNetworkNames() []string {
	return []string{"twister", "bitcoin", "bitcoin-testnet"}
}

// selectNetwork will return a network struct for a given network
func selectNetwork(name string) *network {
	switch name {
	case "twister":
		return &network{
			id:          0xd2bbdaf0,
			port:        28333,
			pver:        60000,
			ttl:         600,
			maxSize:     1000,
			name:        "TwisterNet",
			description: "Twister P2P Net",
			seeders:     []string{"seed2.twister.net.co", "seed.twister.net.co", "seed3.twister.net.co"},
			maxStart:    []uint32{15, 15, 15, 30},
			delay:       []int64{184, 678, 237, 1876},
		}
	case "bitcoin":
		return &network{
			id:          0xd9b4bef9,
			port:        8333,
			pver:        70001,
			ttl:         900,
			maxSize:     1250,
			name:        "BitcoinMainNet",
			description: "Bitcoin Main Net",
			seeders:     []string{"dnsseed.bluematt.me", "bitseed.xf2.org", "dnsseed.bitcoin.dashjr.org", "seed.bitcoin.sipa.be"},
			maxStart:    []uint32{20, 20, 20, 30},
			delay:       []int64{210, 789, 234, 1876},
		}
	case "bitcoin-testnet":
		return &network{
			id:          0xdab5bffa,
			port:        18333,
			pver:        70001,
			ttl:         300,
			maxSize:     250,
			name:        "BitcoinTestNet",
			description: "Bitcoin Test Net",
			seeders:     []string{"testnet-seed.alexykot.me", "testnet-seed.bitcoin.petertodd.org", "testnet-seed.bluematt.me", "testnet-seed.bitcoin.schildbach.de"},
			maxStart:    []uint32{15, 15, 15, 30},
			delay:       []int64{184, 678, 237, 1876},
		}
	default:
		return nil
	}
}

/*

 */
