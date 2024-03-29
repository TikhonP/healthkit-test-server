package main

import (
	"database/sql"
	"encoding/json"
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
	Id           int
	CreatedAt    time.Time
	Time         time.Time
	CategoryName string
	Source       string
	Value        string
}

func (record Record) save() error {
	log.Println("Saving record", record)
	_, err := db.Exec(`INSERT INTO records (created_at, time, category_name, source, value) VALUES (?, ?, ?, ?, ?)`, record.CreatedAt, record.Time, record.CategoryName, record.Source, record.Value)
	return err
}

func NewRecord(data *HKRecord) Record {
	log.Println("NewRecord form", data)
	return Record{CreatedAt: time.Now().UTC(), Time: data.GetTime().UTC(), CategoryName: data.CategoryName, Source: data.Source, Value: data.Value}
}

type QueryHandleEventJSON struct {
	CategoryName string `json:"category_name"`
	ValuesCount  int    `json:"values_count"`
}

func (e QueryHandleEventJSON) save() error {
	log.Println("Saving query handle event", e)
	_, err := db.Exec(`INSERT INTO query_handle_event (created_at, category_name, values_count) VALUES (?, ?, ?)`, time.Now().UTC(), e.CategoryName, e.ValuesCount)
	return err
}

type QueryHandleEvent struct {
	Id           int
	CreatedAt    time.Time
	CategoryName string
	ValuesCount  int
}

var schema = `
            CREATE TABLE IF NOT EXISTS records (
                id INTEGER PRIMARY KEY NOT NULL,
                created_at DATETIME NOT NULL,
                time DATETIME NOT NULL,
                category_name STRING NOT NULL,
                source STRING NOT NULL,
                value STRING NOT NULL
            );
			
			CREATE TABLE IF NOT EXISTS query_handle_event (
				id INTEGER PRIMARY KEY NOT NULL,
				created_at DATETIME NOT NULL,
				category_name STRING NOT NULL,
				values_count INTEGER NOT NULL
			)
`

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

// JSON schema

type HKRecord struct {
	Timestamp    int64  `json:"time"`
	CategoryName string `json:"category_name"`
	Source       string `json:"source"`
	Value        string `json:"value"`
}

func (r *HKRecord) GetTime() time.Time {
	return time.Unix(r.Timestamp, 0)
}

// URLs handlers

func hkRequestEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var event QueryHandleEventJSON
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dbErr := event.save()
	if dbErr != nil {
		log.Println("Failed to save query handle event", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Handles request with some string data in body and stores it to database
func hkRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(r.Body)
	for {
		var records []HKRecord
		if err := dec.Decode(&records); err == io.EOF {
			break
		} else if err != nil {
			log.Println("Failed read decode request body.", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			_, err := fmt.Fprintf(w, err.Error())
			if err != nil {
				log.Println("Failed to raise response", err.Error())
				return
			}
			return
		}
		for _, record := range records {
			saveErr := NewRecord(&record).save()
			if saveErr != nil {
				log.Println("Failed to save new record.", saveErr.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
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
	rows, queryErr := db.Query(`SELECT id, created_at, time, category_name, source, value FROM records ORDER BY created_at DESC`)
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
		scanErr := rows.Scan(&r.Id, &r.CreatedAt, &r.Time, &r.CategoryName, &r.Source, &r.Value)
		if scanErr != nil {
			log.Println("Failed to scan record. ", scanErr.Error())
		}
		records = append(records, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		log.Println("Row iteration error. ", rowsErr.Error())
	}

	eventsRows, eventQueryErr := db.Query(`SELECT id, created_at, category_name, values_count FROM query_handle_event ORDER BY created_at DESC`)
	if eventQueryErr != nil {
		log.Println("Failed to fetch query handle events from db. ", queryErr.Error())
	}
	defer func(eventsRows *sql.Rows) {
		err := eventsRows.Close()
		if err != nil {
			log.Println("Failed to close SQL query. ", err.Error())
		}
	}(eventsRows)

	var queryHandleEvents []QueryHandleEvent
	for eventsRows.Next() {
		var e QueryHandleEvent
		scanErr := eventsRows.Scan(&e.Id, &e.CreatedAt, &e.CategoryName, &e.ValuesCount)
		if scanErr != nil {
			log.Println("Failed to scan event. ", scanErr.Error())
		}
		queryHandleEvents = append(queryHandleEvents, e)
	}
	if eventRowsErr := eventsRows.Err(); eventRowsErr != nil {
		log.Println("Row iteration error. ", eventRowsErr.Error())
	}

	type HtmlData struct {
		Records           []Record
		QueryHandleEvents []QueryHandleEvent
	}

	tmpl, templateErr := template.ParseFiles("records.html")
	if templateErr != nil {
		log.Fatal("Failed to load template. ", templateErr.Error())
	}
	executeErr := tmpl.Execute(w, HtmlData{Records: records, QueryHandleEvents: queryHandleEvents})
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
	http.HandleFunc("/query_handle_event/", logging(hkRequestEventHandler))

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
