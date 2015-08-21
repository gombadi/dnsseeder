# dnsseeder
Go Language dns seeder for the Twister P2P network

This is a dns seeder for the [Twister P2P network](http://twister.net.co/)

It is based on the original twister-seeder https://github.com/miguelfreitas/twister-seeder

Also see the associated utility to display information about [non-standard ip addresses](https://github.com/gombadi/nonstd/)


> **NOTE:** This repository is under ongoing development. Stable releases have been tagged and should be used for production systems.


## Installing

Simply use go get to download the code:

    $ go get github.com/gombadi/dnsseeder

Dependencies are handled by the Go vendor directory.
Note: This means the codebase requires Go 1.5 or higher and use of GO15VENDOREXPERIMENT=1

There seem to be some issues with using go get so a process that works for me on Unix based systems is -

```
cd ${HOME}
mkdir -p go/src/github.com/gombadi
cd go/src/github.com/gombadi
git clone the repo into this directory
cd dnsseeder
go install

```
The binary will then be available in ${HOME}/go/bin


## Usage

    $ dnsseeder -h <domain respond to>

An easy way to run the program is with tmux or screen. This enables you to log out and leave the program running.

```

Command line Options:
-h hostname to serve 
-p port to listen on
-d Produce debug output
-v Produce verbose output
-v Produce stats output

```

An easy way to run the program is with the following script. Change to suit your system.

```

#!/bin/bash

LOGDIR=${HOME}/goseederlogs/

mkdir -p ${LOGDIR}

# pass through the logging level needed
if [ -z ${1} ]; then
        LOGLV="-v"
else
        LOGLV="${1}"
fi

cd
echo
echo "======= Run the Go Language dnsseed ======="
echo
${HOME}/go/bin/dnsseeder -h <host.to.serve> -p <port.to.listen.on> ${LOGLV} 2>&1 | tee ${LOGDIR}/$(date +%F-%s)-goseeder.log


```


## RUNNING AS NON-ROOT

Typically, you'll need root privileges to listen to port 53 (name service).

One solution is using an iptables rule (Linux only) to redirect it to
a non-privileged port:

$ iptables -t nat -A PREROUTING -p udp --dport 53 -j REDIRECT --to-port 5353

If properly configured, this will allow you to run dnsseeder in userspace, using
the -p 5353 option.


## License

For the DNS library license see https://github.com/miekg/dns

For the bitcoin library license see https://github.com/btcsuite/btcd


