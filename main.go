package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {

	var flags struct {
		BindAddr string
	}

	flag.StringVar(&flags.BindAddr, "b", "0.0.0.0", "bind address")

	log.SetPrefix("TRACE ")
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// http
	if err := serve(flags.BindAddr, "80", parseDomainHttp); err != nil {
		log.Fatal(err)
	}

	// https
	if err := serve(flags.BindAddr, "443", parseDomainHttps); err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sigCh
	log.Fatalf("Exit!!!")
}

func serve(host, port string, readDomain func(net.Conn) (string, []byte, int, error)) error {
	l, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}

	log.Printf("listening on %s", net.JoinHostPort(host, port))
	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("failed to accept: %s", err)
			continue
		}

		go func() {
			defer c.Close()
			// read domain name
			domain, header, headerSize, err := readDomain(c)
			if err != nil {
				log.Printf("cannot parse domain from [%s] header: %s, close connect", c.RemoteAddr(), err)
				return
			}

			// TODO white and black list

			remoteAddr := net.JoinHostPort(domain, port)
			rc, err := net.DialTimeout("tcp", remoteAddr, time.Second*2)
			if err != nil {
				log.Printf("failed to connect to target %s: %s", remoteAddr, err)
				return
			}

			log.Printf("proxy %s <-> %s[%s]", c.RemoteAddr(), remoteAddr, rc.RemoteAddr())
			_, _ = rc.Write(header[:headerSize])
			if err = relay(c, rc); err != nil {
				log.Printf("relay error: %s <-> %s, %s", c.RemoteAddr(), remoteAddr, err)
			}
		}()
	}
}

func readHeader(r net.Conn) ([]byte, error) {
	buf := make([]byte, 16384)
	readOffset := 0
	for {
		r.SetReadDeadline(time.Now().Add(time.Second * 1))
		n, err := r.Read(buf[readOffset:])
		if err != nil {
			return nil, err
		}
		if n <
		return nil, err
	}
}

func parseDomainHttp(r net.Conn) (string, []byte, int, error) {
	header, err := readHeader(r)
	var readBuf int

	for {
		r.SetReadDeadline(time.Now().Add(time.Second * 1))
		n, err := r.Read(buf[readBuf:])
		if err != nil {
			return "", nil, 0, nil
		}
		readBuf += n
		reg := regexp.MustCompile(`\r\n\r\n`)
		header := reg.FindAllString(string(buf[:readBuf]), -1)
		if header != nil {
			break
		}
	}
	// read all header
	reg := regexp.MustCompile(`Host:.*\r\n`)
	domains := reg.FindAllString(string(buf[:readBuf]), -1)
	if domains == nil {
		return "", nil, 0, fmt.Errorf("not found Host field")
	}
	domain := strings.TrimRight(strings.Split(domains[0], ":")[1], "\r\n")
	domain = strings.TrimLeft(domain, " ")
	return domain, buf, readBuf, nil
}

func parseDomainHttps(r net.Conn) (string, []byte, int, error) {
	/* 1   TLS_HANDSHAKE_CONTENT_TYPE
	 * 1   TLS major version
	 * 1   TLS minor version
	 * 2   TLS Record length
	 * --------------
	 * 1   Handshake type
	 * 3   Length
	 * 2   Version
	 * 32  Random
	 * 1   Session ID length
	 * ?   Session ID
	 * 2   Cipher Suites length
	 * ?   Cipher Suites
	 * 1   Compression Methods length
	 * ?   Compression Methods
	 * 2   Extensions length
	 * ---------------
	 * 2   Extension data length
	 * 2   Extension type (0x0000 for server_name)
	 * ---------------
	 * 2   server_name list length
	 * 1   server_name type (0)
	 * 2   server_name length
	 * ?   server_name
	 */
	return "", nil, 0, nil
}

func relay(left, right net.Conn) error {
	var err, err1 error
	var wg sync.WaitGroup
	var wait = 5 * time.Second
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err1 = io.Copy(right, left)
		right.SetReadDeadline(time.Now().Add(wait)) // unblock read on right
	}()
	_, err = io.Copy(left, right)
	left.SetReadDeadline(time.Now().Add(wait)) // unblock read on left
	wg.Wait()
	if err1 != nil && !errors.Is(err1, os.ErrDeadlineExceeded) { // requires Go 1.15+
		return err1
	}
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) {
		return err
	}
	return nil
}
