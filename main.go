package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Book struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Price       float64   `json:"price"`
	PublishYear int       `json:"publish_year"`
	CreatedAt   time.Time `json:"created_at"`
}

var db *sql.DB

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "admin")
	password := getEnv("DB_PASSWORD", "password123")
	dbname := getEnv("DB_NAME", "libra_db")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)

	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Retry connecting up to 10 times (useful when DB starts after the app in k8s)
	for i := 1; i <= 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("DB not ready (attempt %d/10): %v", i, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	log.Println("Connected to database successfully")

	http.HandleFunc("GET /healthz", healthCheck)
	http.HandleFunc("POST /v1/books", createBook)
	http.HandleFunc("GET /v1/books", getAllBooks)
	http.HandleFunc("GET /v1/books/{id}", getBookByID)
	http.HandleFunc("PUT /v1/books/{id}", updateBook)
	http.HandleFunc("DELETE /v1/books/{id}", deleteBook)

	log.Println("Server running on :3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// --- HANDLERS ---

func healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func createBook(w http.ResponseWriter, r *http.Request) {
	var b Book
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	err := db.QueryRow(
		"INSERT INTO books (title, author, price, publish_year) VALUES ($1, $2, $3, $4) RETURNING id, created_at",
		b.Title, b.Author, b.Price, b.PublishYear,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(b)
}

func getAllBooks(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, author, price, publish_year, created_at FROM books ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		rows.Scan(&b.ID, &b.Title, &b.Author, &b.Price, &b.PublishYear, &b.CreatedAt)
		books = append(books, b)
	}
	if books == nil {
		books = []Book{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(books)
}

func getBookByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var b Book
	err := db.QueryRow(
		"SELECT id, title, author, price, publish_year, created_at FROM books WHERE id = $1", id,
	).Scan(&b.ID, &b.Title, &b.Author, &b.Price, &b.PublishYear, &b.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

func updateBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var b Book
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	result, err := db.Exec(
		"UPDATE books SET title=$1, author=$2, price=$3, publish_year=$4 WHERE id=$5",
		b.Title, b.Author, b.Price, b.PublishYear, id,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message":"book %s updated"}`, id)
}

func deleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := db.Exec("DELETE FROM books WHERE id = $1", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
