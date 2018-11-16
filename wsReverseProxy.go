package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Proxy struct {
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := p.handler(r); h != nil {
		h.ServeHTTP(w, r)
		return
	}
	log.Println("couldn't find any handler")
	http.Error(w, "Not found.", http.StatusNotFound)
	return
}

func (p *Proxy) handler(req *http.Request) http.Handler {

	// get target url from DB
	target := "http://localhost:3000"
	url, _ := url.Parse(target)
	if isWebsocket(req) {
		log.Println("WEBSOCKET")
		return websocketHandler("127.0.0.1:8080")
	}

	return reverseProxyHandler(url)
}

func reverseProxyHandler(target *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme
		},
	}
}

func websocketHandler(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, "Error contacting backend server.", 500)
			log.Printf("Error dialing websocket backend %s: %v", target, err)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Not a hijacker?", 500)
			return
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			log.Printf("Hijack error: %v", err)
			return
		}
		defer nc.Close()
		defer d.Close()

		err = r.Write(d)
		if err != nil {
			log.Printf("Error copying request to target: %v", err)
			return
		}

		errc := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err := io.Copy(dst, src)
			errc <- err
		}
		go cp(d, nc)
		go cp(nc, d)
		<-errc
	})
}

func isWebsocket(req *http.Request) bool {
	conn_hdr := ""
	conn_hdrs := req.Header["Connection"]
	if len(conn_hdrs) > 0 {
		conn_hdr = conn_hdrs[0]
	}

	upgrade_websocket := false
	if strings.ToLower(conn_hdr) == "upgrade" {
		upgrade_hdrs := req.Header["Upgrade"]
		if len(upgrade_hdrs) > 0 {
			upgrade_websocket = (strings.ToLower(upgrade_hdrs[0]) == "websocket")
		}
	}

	return upgrade_websocket
}

func main() {
	// err's are ommited for sake of pasting code and simplicity, production code has err handling
	reverseProxy := &Proxy{}
	addr, _ := net.ResolveTCPAddr("tcp", ":1330")
	log.Println(":1330")
	listener, _ := net.ListenTCP("tcp", addr)
	_ = http.Serve(listener, reverseProxy)

}
