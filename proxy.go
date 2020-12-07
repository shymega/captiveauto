package main

import (
	"context"
	"log"
	"net"
	"os/exec"
	"regexp"

	socks5 "github.com/armon/go-socks5"
)

type UpstreamResolver struct {
	r *net.Resolver
}

func NewUpstreamResolver(upstream string, dialer *net.Dialer) *UpstreamResolver {
	return &UpstreamResolver{
		r: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Redirect all Resolver dials to the upstream.
				return dialer.DialContext(ctx, network,
					net.JoinHostPort(upstream, "53"))
			},
		},
	}
}

func (u *UpstreamResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	log.Println("Redirected DNS lookup:", name)
	addrs, err := u.r.LookupIPAddr(ctx, name)
	if err != nil {
		return ctx, nil, err
	}
	if len(addrs) == 0 {
		return ctx, nil, nil
	}
	// Prefer IPv4, like ResolveIPAddr. I can hear Olafur screaming, but the default
	// go-socks5 Resolver uses ResolveIPAddr, and the interface does not allow any better.
	for _, addr := range addrs {
		if addr.IP.To4() != nil {
			return ctx, addr.IP, nil
		}
	}

	return ctx, addrs[0].IP, nil
	// (Why the hell does this *return* a context?)
}

func main() {
	log.Println("Checking DHCP-provided DNS..")
	out, err := exec.Command("/bin/sh", "-c", "/usr/local/bin/current-dhcp-dns").Output()
	match := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}.\d{1,3}`).Find(out)
	if match == nil {
		log.Fatalln("No DHCP DNS servers found!")
	}

	dialer := &net.Dialer{}
	upstream := string(match)

	srv, err := socks5.New(&socks5.Config{
		Resolver: NewUpstreamResolver(upstream, dialer),
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
	})
	if err != nil {
		panic(err)
	}

	log.Println("Upstream DHCP servers are: " + upstream)
	go func() {
		log.Println("Proxy started on port 16662!")
		log.Fatalln(srv.ListenAndServe("tcp", "127.0.0.1:16662"))
	}()

	select {}
}
