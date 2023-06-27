package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/miekg/dns"
)

type HOSTS struct {
	domain string
	ip     string
}

type DNSSERVER struct {
	server string
	port   int
}

func check_ping(ip string) (icmp_reachable bool) {

	pinger, err := ping.NewPinger(ip)
	if err != nil {
		log.Fatal(err)
	}

	pinger.Count = 5
	pinger.Timeout = time.Second * 1

	err = pinger.Run()
	if err != nil {
		log.Fatal(err)
	}

	stats := pinger.Statistics()
	if stats.PacketsSent == stats.PacketsRecv {
		return true
	}
	return false
}

func main() {
	domain := flag.String("d", "", "domain name")
	udpServer := flag.String("u", "", "dns server (use /etc/resolv.conf if not specified)")
	udpPort := flag.Int("p", 53, "dns udp port")

	flag.Parse()

	if *domain == "" && flag.NArg() > 0 {
		*domain = flag.Arg(0)
	}

	if *domain == "" {
		fmt.Println("No domain name specified.")
		return
	}

	var dns_server string
	if *udpServer != "" {
		dns_server = *udpServer
	} else {
		config, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
		dns_server = config.Servers[0]
	}

	var port int
	if *udpPort != 0 {
		port = *udpPort
	}

	dnsServer := DNSSERVER{
		server: dns_server,
		port:   port,
	}

	dnsc := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(*domain), dns.TypeA)
	m.RecursionDesired = true

	r, _, err := dnsc.Exchange(m, net.JoinHostPort(dnsServer.server, strconv.Itoa(dnsServer.port)))
	if r == nil {
		log.Fatalf("*** error: %s\n", err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		log.Fatalf(" *** invalid answer name %s after MX query for %s\n", os.Args[1], os.Args[1])
	}
	hosts := []HOSTS{}
	for _, a := range r.Answer {
		host := HOSTS{
			domain: *domain,
			ip:     a.(*dns.A).A.String(),
		}
		// check icmp ping reachable
		if check_ping(host.ip) {
			hosts = append(hosts, host)
		}
	}

	for _, host := range hosts {
		fmt.Printf("%v\t%v\n", host.domain, host.ip)
	}

}
