// Command ipv6proxy is a local HTTP CONNECT proxy that dials targets over IPv6 first.
// make docker-up runs it on the host so the Docker worker can reach euremotejobs.com:
// the container has no IPv6 route, and that site's IPv4 answers with a SiteGround challenge.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "listen address")
	detach := flag.Bool("detach", false, "run in a new session and print the child pid")
	flag.Parse()
	if *detach {
		if err := detachChild(*addr); err != nil {
			log.Fatal(err)
		}
		return
	}
	signal.Ignore(syscall.SIGHUP)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           http.HandlerFunc(serve),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func detachChild(addr string) error {
	logf, err := os.OpenFile("bin/ipv6proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	p, err := os.StartProcess(os.Args[0], []string{os.Args[0], "-addr", addr}, &os.ProcAttr{
		Files: []*os.File{null, logf, logf},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		return err
	}
	fmt.Println(p.Pid)
	return nil
}

func serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/health" {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
		return
	}
	if r.Method != http.MethodConnect {
		http.Error(w, "connect required", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	upstream, err := dialPreferIPv6(ctx, r.Host)
	cancel()
	if err != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	defer func() { _ = upstream.Close() }()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()
	<-done
}

func dialPreferIPv6(ctx context.Context, hostport string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 15 * time.Second}
	var v4 []net.IPAddr
	var dialErr error
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			v4 = append(v4, ip)
			continue
		}
		c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return c, nil
		}
		dialErr = err
	}
	for _, ip := range v4 {
		c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return c, nil
		}
		dialErr = err
	}
	if dialErr == nil {
		dialErr = fmt.Errorf("no addresses for %s", host)
	}
	return nil, dialErr
}
