package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// Order represents an order placed by a user.
type Order struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
}

var (
	db             *sql.DB
	userServiceURL string
)

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./orders.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id  INTEGER NOT NULL,
			product  TEXT    NOT NULL,
			quantity INTEGER NOT NULL
		)
	`)
	if err != nil {
		log.Fatal("Failed to create orders table:", err)
	}
	log.Println("Database initialized")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// userExists calls User Service to verify the user with the given ID exists.
func userExists(userID int) (bool, error) {
	url := fmt.Sprintf("%s/users/%d", userServiceURL, userID)
	resp, err := http.Get(url) //nolint:gosec // URL is built from controlled config
	if err != nil {
		return false, fmt.Errorf("failed to contact user service: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// ----- /orders -----

func handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listOrders(w, r)
	case http.MethodPost:
		createOrder(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func listOrders(w http.ResponseWriter, r *http.Request) {
	var rows *sql.Rows
	var err error

	// Optional filter: GET /orders?user_id=1
	if rawUserID := r.URL.Query().Get("user_id"); rawUserID != "" {
		userID, convErr := strconv.Atoi(rawUserID)
		if convErr != nil || userID <= 0 {
			http.Error(w, "Invalid user_id query param", http.StatusBadRequest)
			return
		}
		rows, err = db.Query(
			"SELECT id, user_id, product, quantity FROM orders WHERE user_id = ? ORDER BY id",
			userID,
		)
	} else {
		rows, err = db.Query("SELECT id, user_id, product, quantity FROM orders ORDER BY id")
	}

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Product, &o.Quantity); err != nil {
			http.Error(w, "Scan error", http.StatusInternalServerError)
			return
		}
		orders = append(orders, o)
	}
	writeJSON(w, http.StatusOK, orders)
}

func createOrder(w http.ResponseWriter, r *http.Request) {
	var o Order
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	if o.UserID <= 0 || strings.TrimSpace(o.Product) == "" || o.Quantity <= 0 {
		http.Error(w, "Fields 'user_id' (>0), 'product', and 'quantity' (>0) are required", http.StatusBadRequest)
		return
	}

	// ── Inter-service call: verify user exists ──────────────────────────────
	exists, err := userExists(o.UserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Could not verify user: %v", err), http.StatusServiceUnavailable)
		return
	}
	if !exists {
		http.Error(w, fmt.Sprintf("User with id %d not found", o.UserID), http.StatusBadRequest)
		return
	}

	result, err := db.Exec(
		"INSERT INTO orders (user_id, product, quantity) VALUES (?, ?, ?)",
		o.UserID, strings.TrimSpace(o.Product), o.Quantity,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	o.ID = int(id)
	writeJSON(w, http.StatusCreated, o)
}

// ----- /orders/{id} -----

func handleOrder(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimPrefix(r.URL.Path, "/orders/")
	rawID = strings.TrimSuffix(rawID, "/")
	id, err := strconv.Atoi(rawID)
	if err != nil || id <= 0 {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		getOrder(w, id)
	case http.MethodDelete:
		deleteOrder(w, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getOrder(w http.ResponseWriter, id int) {
	var o Order
	err := db.QueryRow(
		"SELECT id, user_id, product, quantity FROM orders WHERE id = ?", id,
	).Scan(&o.ID, &o.UserID, &o.Product, &o.Quantity)

	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, fmt.Sprintf("Order with id %d not found", id), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func deleteOrder(w http.ResponseWriter, id int) {
	result, err := db.Exec("DELETE FROM orders WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, fmt.Sprintf("Order with id %d not found", id), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	userServiceURL = os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://localhost:8081"
	}
	log.Printf("Using User Service URL: %s", userServiceURL)

	initDB()
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/orders", handleOrders)
	mux.HandleFunc("/orders/", handleOrder)

	log.Println("Order Service listening on :8082")
	if err := http.ListenAndServe(":8082", mux); err != nil {
		log.Fatal(err)
	}
}
