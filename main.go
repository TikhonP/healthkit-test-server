package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Database related

var db *sql.DB

type Record struct {
	Id        int
	Data      string
	CreatedAt time.Time
}

func (record Record) save() error {
	_, err := db.Exec(`INSERT INTO records (created_at, data) VALUES (?, ?)`, record.CreatedAt, record.Data)
	return err
}

func NewRecord(data string) Record {
	return Record{Data: data, CreatedAt: time.Now()}
}

var schema = `
            CREATE TABLE IF NOT EXISTS records (
                id INTEGER PRIMARY KEY NOT NULL,
                created_at DATETIME NOT NULL,
                data TEXT NOT NULL
            );`

func initializeDB() {
	var sqlErr error
	db, sqlErr = sql.Open("sqlite3", "db.sqlite3")
	if sqlErr != nil {
		log.Fatal("Failed to connect database. ", sqlErr.Error())
	}
	if pingErr := db.Ping(); pingErr != nil {
		log.Fatal("Failed to ping. ", pingErr.Error())
	}
	if _, schemaErr := db.Exec(schema); schemaErr != nil {
		log.Fatal("Failed to create schema. ", schemaErr.Error())
	}
}

// URLs handlers

// Handles request with some string data in body and stores it to database
func hkRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("Failed to close request body after read.", err.Error())
		}
	}(r.Body)
	reqBody, readBodyErr := io.ReadAll(r.Body)
	if readBodyErr != nil {
		log.Println("Failed read request body.", readBodyErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	saveErr := NewRecord(string(reqBody)).save()
	if saveErr != nil {
		log.Println("Failed to save new record.", saveErr.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Home path and any path handler.
func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		log.Println("Page not found (404).", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_, err := fmt.Fprintf(w, "Купил мужик щляпу, а она ему как раз.")
	if err != nil {
		log.Println("Failed to fire HTTP response.", err.Error())
		return
	}
}

// Handler for samples list path.
func getSamplesHandler(w http.ResponseWriter, _ *http.Request) {
	rows, queryErr := db.Query(`SELECT id, data, created_at FROM records ORDER BY created_at DESC`)
	if queryErr != nil {
		log.Println("Failed to fetch records from db. ", queryErr.Error())
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println("Failed to close SQL query. ", err.Error())
		}
	}(rows)

	var records []Record
	for rows.Next() {
		var r Record
		scanErr := rows.Scan(&r.Id, &r.Data, &r.CreatedAt)
		if scanErr != nil {
			log.Println("Failed to scan record. ", scanErr.Error())
		}
		records = append(records, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		log.Println("Row iteration error. ", rowsErr.Error())
	}

	type TemplateData struct {
		Records []Record
	}

	tmpl, templateErr := template.ParseFiles("records.html")
	if templateErr != nil {
		log.Fatal("Failed to load template. ", templateErr.Error())
	}
	executeErr := tmpl.Execute(w, TemplateData{Records: records})
	if executeErr != nil {
		log.Println("Execute query failed. ", executeErr.Error())
		return
	}

}

// Logging middleware
func logging(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL.Path)
		f(w, r)
	}
}

func main() {
	initializeDB()

	http.HandleFunc("/", logging(homeHandler))
	http.HandleFunc("/samples/", logging(getSamplesHandler))
	http.HandleFunc("/sample/", logging(hkRequestHandler))

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}
	log.Printf("Starting server at http://127.0.0.1:%v/", httpPort)
	err := http.ListenAndServe(":"+httpPort, nil)
	if err != nil {
		log.Fatal("Failed to start server. ", err.Error())
	}
}
