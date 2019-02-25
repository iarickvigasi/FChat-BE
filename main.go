package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Message struct {
	Name    string
	Message string
}

type GetRequest struct {
	Offset int
	Limit  int
}

var (
	repeat int
	db     *sql.DB
)

func repeatFunc(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var buffer bytes.Buffer
		for i := 0; i < repeat; i++ {
			buffer.WriteString("Hello from Go!")
		}
		w.Write([]byte(buffer.String()))
	}
}

func dbFunc(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec("CREATE TABLE IF NOT EXISTS ticks (tick timestamp)"); err != nil {
			http.Error(w, fmt.Sprintf("Error creating database table: %q", err), http.StatusInternalServerError)
			return
		}

		if _, err := db.Exec("INSERT INTO ticks VALUES (now())"); err != nil {
			http.Error(w, fmt.Sprintf("Error incrementing tick: %q", err), http.StatusInternalServerError)
			return
		}

		rows, err := db.Query("SELECT tick FROM ticks")
		if err != nil {
			http.Error(w, fmt.Sprintf("Error reading ticks: %q", err), http.StatusInternalServerError)
			return
		}

		defer rows.Close()
		for rows.Next() {
			var tick time.Time
			if err := rows.Scan(&tick); err != nil {
				http.Error(w, fmt.Sprintf("Error scanning ticks: %q", err), http.StatusInternalServerError)
				return
			}
			w.Write([]byte(fmt.Sprintf("Read from DB: %s\n", tick.String())))
		}
	}
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

func main() {

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	var err error
	tStr := os.Getenv("REPEAT")
	repeat, err = strconv.Atoi(tStr)
	if err != nil {
		log.Print("Error converting $REPEAT to an int: %q - Using default", err)
		repeat = 5
	}

	conn, _ := connectSQL()

	http.HandleFunc("/", mainHandler(conn))
	http.HandleFunc("/db", dbFunc(conn))
	http.HandleFunc("/repeat", dbFunc(conn))
	http.ListenAndServe(":"+port, nil)
}

func connectSQL() (*sqlx.DB, error) {
	conn, err := sqlx.Connect("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	return conn, err
}

func validateURL(url string) ([]string, error) {
	str := strings.FieldsFunc(url, func(r rune) bool {
		if r == '/' {
			return true
		}
		return false
	})

	if len(str) < 2 || len(str) > 2 {
		return nil, incorectURLError()
	}
	return str, nil
}

func incorectURLError() error {
	return errors.New("Incorrect URL: Use /send/{channelName} or /get/{channelName}")
}

func incorectBodyRequest() error {
	return errors.New("Incorrect Body Data: send { Message: string, Name: string }")
}

func incorectBodyGetRequest() error {
	return errors.New("Incorrect Body Data: send { Offset: int, Limit: int }")
}

func sendHandler(w http.ResponseWriter, r *http.Request, db *sqlx.DB, room string) {
	// Check if request contains message and name
	// Write message
	decoder := json.NewDecoder(r.Body)
	var t Message
	err := decoder.Decode(&t)
	if err != nil {
		checkHTTPError(w, err)
		return
	}

	if len(t.Message) == 0 || len(t.Name) == 0 {
		checkHTTPError(w, incorectBodyRequest())
		return
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO %s VALUES ('%s', '%s');", room, t.Name, t.Message))
	checkHTTPError(w, err)
	if err != nil {
		return
	}

	w.Write([]byte("OK! Successfully sent message!"))
}

func getHandler(w http.ResponseWriter, r *http.Request, db *sqlx.DB, room string) {
	// Check if request contains offset and lenght
	// Write message
	var Offset, err = strconv.Atoi(r.URL.Query().Get("offset"))
	if err != nil {
		checkHTTPError(w, err)
		return
	}

	var Limit, errLimitConv = strconv.Atoi(r.URL.Query().Get("limit"))
	if errLimitConv != nil {
		checkHTTPError(w, err)
		return
	}

	var body = GetRequest{Offset, Limit}

	if body.Limit <= 0 || body.Offset < 0 {
		checkHTTPError(w, incorectBodyGetRequest())
		return
	}

	var messages []Message
	db.Select(&messages,
		fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", room, body.Limit, body.Offset))

	js, err := json.Marshal(messages)
	checkHTTPError(w, err)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func mainHandler(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)

		action, room := parseURL(w, r.URL.Path)
		err := checkAndCreateRoom(db, room, w)
		if err != nil {
			checkHTTPError(w, err)
			return
		}

		if action == "send" {
			sendHandler(w, r, db, room)
			return
		}

		if action == "get" {
			getHandler(w, r, db, room)
			return
		}
	}
}

func checkCount(rows *sql.Rows) (count int, err error) {
	for rows.Next() {
		err = rows.Scan(&count)
	}
	return count, err
}

func checkHTTPError(w http.ResponseWriter, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func parseURL(w http.ResponseWriter, url string) (action string, room string) {
	urls, err := validateURL(url)
	checkHTTPError(w, err)

	// Check first part (get\send)
	match, _ := regexp.MatchString("^[get|send]{1}", urls[0])
	if !match {
		http.Error(w, incorectURLError().Error(), http.StatusBadRequest)
		return
	}

	// Check seconds part
	reg, err := regexp.Compile("([a-z]){1,200}")
	checkHTTPError(w, err)

	room = reg.FindString(urls[1])
	action = urls[0]
	return action, room
}

func checkAndCreateRoom(db *sqlx.DB, room string, w http.ResponseWriter) (err error) {
	_, err = db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			name VARCHAR (50),
			message TEXT
		);
	`, room))

	return err
}
