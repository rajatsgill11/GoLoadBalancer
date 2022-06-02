package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Config is a configuration.
type Config struct {
	Proxy    Proxy     `json:"proxy"`
	Backends []Backend `json:"backends"`
}

// Proxy is a reverse proxy, and means load balancer.
type Proxy struct {
	Port string `json:"port"`
}

// Backend is servers which load balancer is transferred.
type Backend struct {
	URL    string `json:"url"`
	IsDead bool
	mu     sync.RWMutex
}

type RegisterRequest struct {
	URL string `json:"url"`
}

var cfg Config
var wg = new(sync.WaitGroup)
var regPortNo = 8085

func serveBackend(name string, port string) {
	mux := http.NewServeMux()
	mux.Handle("/proxy", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Backend server name:%v\n", name)
		fmt.Fprintf(w, "Response header:%v\n", r.Header)
	}))
	mux.Handle("/urls/register", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader((http.StatusServiceUnavailable))
		} else {
			decoder := json.NewDecoder(r.Body)
			var request RegisterRequest
			err := decoder.Decode(&request)
			if err != nil {
				panic(err)
			}
			cfg.Backends = append(cfg.Backends, Backend{URL: request.URL})
			// http.ListenAndServe(strconv.Itoa(regPortNo), mux)
			// serverName := request.URL
			// portNumber := regPortNo
			// go func() {
			// 	serveBackend(serverName, strconv.Itoa(portNumber))
			// 	wg.Done()
			// }()
			// regPortNo++
			// lbHandler(w, r)
			fmt.Fprintf(w, "Server Added : %v\n", request.URL)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Backend server name:%v\n", name)
			fmt.Fprintf(w, "Response header:%v\n", r.Header)
		}
	}))
	http.ListenAndServe(port, mux)
}

func main() {

	data, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatal(err.Error())
	}
	json.Unmarshal(data, &cfg)

	generateServers()

}

func generateServers() {

	wg.Add(len(cfg.Backends))

	go func() {
		Serve()
		wg.Done()
	}()
	for i := 1; i <= len(cfg.Backends); i++ {
		serverName := "server" + strconv.Itoa(i)
		portNumber := ":808" + strconv.Itoa(i)
		go func() {
			serveBackend(serverName, portNumber)
			wg.Done()
		}()
	}

	wg.Wait()
}

// SetDead updates the value of IsDead in Backend.
func (backend *Backend) SetDead(b bool) {
	backend.mu.Lock()
	backend.IsDead = b
	backend.mu.Unlock()
}

// GetIsDead returns the value of IsDead in Backend.
func (backend *Backend) GetIsDead() bool {
	backend.mu.RLock()
	isAlive := backend.IsDead
	backend.mu.RUnlock()
	return isAlive
}

var mu sync.Mutex
var idx int = 0

// lbHandler is a handler for loadbalancing
func lbHandler(w http.ResponseWriter, r *http.Request) {
	maxLen := len(cfg.Backends)
	// Round Robin
	mu.Lock()
	currentBackend := cfg.Backends[idx%maxLen]
	if currentBackend.GetIsDead() {
		idx++
	}
	targetURL, err := url.Parse(cfg.Backends[idx%maxLen].URL)
	if err != nil {
		log.Fatal(err.Error())
	}
	idx++
	mu.Unlock()
	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		// NOTE: It is better to implement retry.
		log.Printf("%v is dead.", targetURL)
		currentBackend.SetDead(true)
		lbHandler(w, r)
	}
	reverseProxy.ServeHTTP(w, r)
}

// pingBackend checks if the backend is alive.
func isAlive(url *url.URL) bool {
	conn, err := net.DialTimeout("tcp", url.Host, time.Minute*1)
	if err != nil {
		log.Printf("Unreachable to %v, error:", url.Host, err.Error())
		return false
	}
	defer conn.Close()
	return true
}

// healthCheck is a function for healthcheck
func healthCheck() {
	t := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-t.C:
			for _, backend := range cfg.Backends {
				pingURL, err := url.Parse(backend.URL)
				if err != nil {
					log.Fatal(err.Error())
				}
				isAlive := isAlive(pingURL)
				backend.SetDead(!isAlive)
				msg := "ok"
				if !isAlive {
					msg = "dead"
				}
				log.Printf("%v checked %v by healthcheck", backend.URL, msg)
			}
		}
	}

}

// Serve serves a loadbalancer.
func Serve() {

	go healthCheck()

	s := http.Server{
		Addr:    ":" + cfg.Proxy.Port,
		Handler: http.HandlerFunc(lbHandler),
	}
	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err.Error())
	}
}
