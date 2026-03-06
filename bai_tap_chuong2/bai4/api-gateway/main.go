package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func newReverseProxy(targetURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Rewrite the Host header to the target host so upstream services receive it correctly
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		original(req)
		req.Host = target.Host
	}
	return proxy, nil
}

func main() {
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8081"
	}
	orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
	if orderServiceURL == "" {
		orderServiceURL = "http://localhost:8082"
	}

	userProxy, err := newReverseProxy(userServiceURL)
	if err != nil {
		log.Fatalf("Failed to create user-service proxy: %v", err)
	}
	orderProxy, err := newReverseProxy(orderServiceURL)
	if err != nil {
		log.Fatalf("Failed to create order-service proxy: %v", err)
	}

	mux := http.NewServeMux()

	// Route /users and /users/* → user-service
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[GW] %s %s → user-service", r.Method, r.URL.Path)
		userProxy.ServeHTTP(w, r)
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[GW] %s %s → user-service", r.Method, r.URL.Path)
		userProxy.ServeHTTP(w, r)
	})

	// Route /orders and /orders/* → order-service
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[GW] %s %s → order-service", r.Method, r.URL.Path)
		orderProxy.ServeHTTP(w, r)
	})
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[GW] %s %s → order-service", r.Method, r.URL.Path)
		orderProxy.ServeHTTP(w, r)
	})

	// Health / root
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "Route not found", http.StatusNotFound)
			return
		}
		routes := []string{
			"GET/POST /users       → user-service:8081",
			"GET/POST /users/{id}  → user-service:8081",
			"GET/POST /orders      → order-service:8082",
			"GET/POST /orders/{id} → order-service:8082",
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("API Gateway\n\nRoutes:\n  " + strings.Join(routes, "\n  ") + "\n"))
	})

	log.Printf("API Gateway listening on :8080")
	log.Printf("  /users/*  → %s", userServiceURL)
	log.Printf("  /orders/* → %s", orderServiceURL)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
