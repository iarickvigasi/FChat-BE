package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

type Message struct {
	Name    string
	Message string
}

type GetRequest struct {
	Offset int
	Limit  int
}

func main() {

	conn, _ := connectSQL()
	http.HandleFunc("/", mainHandler(conn))
	http.ListenAndServe(":8080", nil)
}

func connectSQL() (*sqlx.DB, error) {
	conn, err := sqlx.Connect("mysql", "root:12345@tcp(mysql:3306)/chat?charset=utf8")
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

	_, err = db.Exec(fmt.Sprintf("INSERT INTO %s (name, message) VALUES (\"%s\", \"%s\")", room, t.Name, t.Message))
	checkHTTPError(w, err)
	if err != nil {
		return
	}

	w.Write([]byte("OK! Successfully sent message!"))
}

func getHandler(w http.ResponseWriter, r *http.Request, db *sqlx.DB, room string) {
	// Check if request contains offset and lenght
	// Write message

	decoder := json.NewDecoder(r.Body)
	var body GetRequest
	err := decoder.Decode(&body)
	if err != nil {
		checkHTTPError(w, err)
		return
	}

	if body.Limit <= 0 || body.Offset < 0 {
		checkHTTPError(w, incorectBodyGetRequest())
		return
	}

	var messages []Message
	db.Select(&messages,
		fmt.Sprintf("SELECT * FROM %s LIMIT %d,%d", room, body.Offset, body.Limit))

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
		action, room := parseURL(w, r.URL.Path)
		err := checkAndCreateRoom(db, room)
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

func checkCount(rows *sql.Rows) (count int) {
	for rows.Next() {
		err := rows.Scan(&count)
		checkErr(err)
	}
	return count
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
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

func checkAndCreateRoom(db *sqlx.DB, room string) (err error) {
	// Check if such table with messages exists
	// If not create
	var check = fmt.Sprintf(`
		SELECT Count(*) as count
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = "chat" AND TABLE_NAME = '%s';
	`, room)

	rows, err := db.Query(check)
	count := checkCount(rows)

	if err != nil || count == 0 {
		_, err := db.Exec(fmt.Sprintf(`
			CREATE TABLE %s (
				name varchar(50),
				message varchar(249)
			)
			CHARACTER SET utf8;
		`, room))

		return err
	}
	return nil
}
