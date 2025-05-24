package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
	"k8s.io/klog/v2"
)

const (
	xSirupReason = "x-sirup-reason"
	xSirupHost   = "x-sirup-host"
	xSirupProto  = "x-sirup-proto"
	xSirupRemote = "x-sirup-remote"
)

type config struct {
	Mapping map[string]string `yaml:"mapping,flow"`
}

func main() {
	port := flag.Uint("p", 8080, "The port to listen on for incoming requests to proxy.")
	configPath := flag.String("c", "config.yaml", "The path to the config file for sirup.")
	klog.InitFlags(nil)
	defer klog.Flush()
	flag.Parse()

	config, err := readConfig(*configPath)
	if nil != err {
		klog.Exitf("Failed to read config file %q: %v", *configPath, err)
	}

	serverWait := &sync.WaitGroup{}
	serverWait.Add(1)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, os.Kill)
	srv := runServer(serverWait, *port, *config)

	<-stop
	klog.Info("Received shutdown request.")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); nil != err {
		klog.Exitf("Graceful shutdown failed: %v", err)
	}

	serverWait.Wait()
	os.Exit(0)
}

func runServer(wg *sync.WaitGroup, port uint, c config) *http.Server {
	m := c.Mapping
	if len(m) <= 0 {
		klog.Warning("The 'mapping' section of the configuration is empty, no forwarding will be done.")
	}

	r := mux.NewRouter()
	for host, target := range m {
		target = strings.TrimRight(target, "/")
		klog.Infof("Mapping %q to %q", host, target+"/")
		r.Host(host).PathPrefix("").Handler(http.HandlerFunc(makeProxyHandler(target)))
	}
	// Register a default handler for everything else.
	r.PathPrefix("").Handler(http.HandlerFunc(unmappedHandler))
	handler := handlers.ProxyHeaders(addHost(r))
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: handler}

	go func() {
		defer wg.Done()
		klog.Infof("Start listening on %q.", srv.Addr)
		if err := srv.ListenAndServe(); nil != err {
			if errors.Is(err, http.ErrServerClosed) {
				klog.Info("Server successfully shut down.")
				return
			}
			klog.Exitf("Failed to listen on %q: %v", srv.Addr, err)
		}
	}()

	return srv
}

func makeProxyHandler(targetBaseUrl string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Construct the URL of the target server to forward the request to.
		pathAndQuery := getPathAndQuery(r)
		target := targetBaseUrl + pathAndQuery
		klog.V(5).Infof("%s %s -> %s", r.Method, r.URL, target)

		// Pipe the body from the incoming request to the body of the outgoing request.
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			defer r.Body.Close()
			if _, err := io.Copy(pw, r.Body); nil != err {
				klog.Errorf("Failed to copy request body: %v", err)
			}
		}()

		// Create the outgoing request.
		req, err := http.NewRequestWithContext(r.Context(), r.Method, target, pr)
		if nil != err {
			klog.Errorf("Failed to create request %q %q: %v", r.Method, target, err)
			w.Header().Add(xSirupReason, "create-request-failed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Add some diagnostic information to the request.
		req.Header.Add(xSirupHost, r.Host)
		req.Header.Add(xSirupProto, r.URL.Scheme)
		req.Header.Add(xSirupRemote, r.RemoteAddr)

		// Copy request headers.
		for key, value := range r.Header {
			for _, val := range value {
				req.Header.Add(key, val)
			}
		}

		// Execute the request to the target server.
		resp, err := http.DefaultClient.Do(req)
		if nil != err {
			klog.Errorf("Request to \"%s %s\" failed: %v", r.Method, target, err)
			w.Header().Add(xSirupReason, "send-request-failed")
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		// Respond to the incoming request with the details of the response to
		// the outgoing
		for key, value := range resp.Header {
			for _, val := range value {
				w.Header().Add(key, val)
			}
		}
		w.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		io.Copy(w, resp.Body)
	}
}

func unmappedHandler(w http.ResponseWriter, r *http.Request) {
	klog.V(5).Infof("Unmapped %s %s", r.Method, r.URL)
	w.Header().Add(xSirupReason, "unmapped")
	w.WriteHeader(404)
	w.Write([]byte("This page does not exist"))
}

// addHost returns a middleware handler that ensures the request's URL host matches
// the value from the request host.
func addHost(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != r.Host {
			r.URL.Host = r.Host
		}
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func getPathAndQuery(r *http.Request) string {
	pathAndQuery := r.URL.EscapedPath()
	if r.URL.RawQuery != "" {
		pathAndQuery += "?" + r.URL.RawQuery
	}
	return pathAndQuery
}

func readConfig(path string) (*config, error) {
	yamlData, err := os.ReadFile(path)
	if nil != err {
		return nil, err
	}

	c := config{}
	err = yaml.Unmarshal([]byte(yamlData), &c)
	if nil != err {
		return nil, err
	}

	return &c, nil
}
