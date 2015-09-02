/*
This application provides a DNS seeder service to network based on Bitcoin technology.
For example -
http://twister.net.co/
https://bitcoin.org/


This application crawls the Network for active clients and records their ip address and port. It then replies to DNS queries with this information.

Features:
- Preconfigured support for Twister & Bitcoin networks. use -net <network> to load config data.
- supports ipv4 & ipv6 addresses
- revisits clients on a configurable time basis to make sure they are still available
- Low memory & cpu requirements

*/
package main
