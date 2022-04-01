// Copyright (c) 2013-2018 The btcsuite developers
// Copyright (c) 2016-2018 The Decred developers
// Copyright (c) 2021 Jeremy Rand
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file of btcd.

package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/go-socks/socks"
)

func createDial(cfg *configData) error {
	// This function is loosely copied from config.go from btcd.

	funcName := "createDial"

	// Setup dial function depending on the
	// specified options.  The default is to use the standard
	// net.DialTimeout function.  When a
	// proxy is specified, the dial function is set to the proxy specific
	// dial function.
	cfg.dial = net.DialTimeout
	if cfg.proxy != "" {
		_, _, err := net.SplitHostPort(cfg.proxy)
		if err != nil {
			str := "%s: Proxy address '%s' is invalid: %v"
			err := fmt.Errorf(str, funcName, cfg.proxy, err)
			return err
		}

		proxy := &socks.Proxy{
			Addr:         cfg.proxy,
			Username:     cfg.proxyUser,
			Password:     cfg.proxyPass,
			TorIsolation: cfg.torIsolation,
		}
		cfg.dial = proxy.DialTimeout
	}

	return nil
}

// newNetAddress attempts to extract the IP address and port from the passed
// net.Addr interface and create a bitcoin NetAddress structure using that
// information.  Copied from peer.go from btcd.  TODO: Just use btcd's peer package instead.
func newNetAddress(addr net.Addr, services wire.ServiceFlag) (*wire.NetAddress, error) {
	// addr will be a net.TCPAddr when not using a proxy.
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		ip := tcpAddr.IP
		port := uint16(tcpAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// addr will be a socks.ProxiedAddr when using a proxy.
	if proxiedAddr, ok := addr.(*socks.ProxiedAddr); ok {
		ip := net.ParseIP(proxiedAddr.Host)
		if ip == nil {
			ip = net.ParseIP("0.0.0.0")
		}
		port := uint16(proxiedAddr.Port)
		na := wire.NewNetAddressIPPort(ip, port, services)
		return na, nil
	}

	// For the most part, addr should be one of the two above cases, but
	// to be safe, fall back to trying to parse the information from the
	// address string as a last resort.
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, err
	}
	na := wire.NewNetAddressIPPort(ip, uint16(port), services)
	return na, nil
}
