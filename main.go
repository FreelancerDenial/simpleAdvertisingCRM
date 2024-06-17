package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Global DB connection
var db *sql.DB

func main() {
	_, err := setupDatabase()
	if err != nil {
		log.Fatal(err)
	}

	startServer()
}

// Setting up SQLite DB and create Clients table for the first run
func setupDatabase() (*sql.DB, error) {
	var err error

	db, err = sql.Open("sqlite3", "./clients.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, working_hours TEXT, priority INTEGER, lead_capacity INTEGER, existing_leads INTEGER)`)
	if err != nil {
		log.Fatal(err)
	}

	return db, nil
}

// Starting up local server and handle the routes
func startServer() {
	http.HandleFunc("/clients/new", createNewClient)
	http.HandleFunc("/clients", retrieveAllClients)
	http.HandleFunc("/client", retrieveClient)
	http.HandleFunc("/assignLead", assignLead)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// createNewClient processes a POST request to create a new client.
// It checks if the client already exists in the database, and if so, returns the existing client's ID. Otherwise, it inserts the new client
// into the database and returns the newly generated ID.
func createNewClient(w http.ResponseWriter, r *http.Request) {
	var client Client

	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := json.NewDecoder(r.Body).Decode(&client)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	row := db.QueryRow("SELECT id FROM clients WHERE name = ?", client.Name)
	var existingID int
	err = row.Scan(&existingID)

	if !errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]int{
			"id": existingID,
		})
		return
	}

	res, err := db.Exec("INSERT INTO clients (name, working_hours, priority, lead_capacity, existing_leads) VALUES (?, ?, ?, ?, ?)",
		client.Name, client.WorkingHours, client.Priority, client.LeadCapacity, client.ExistingLeads)
	if err != nil {
		log.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{
		"id": id,
	})
}

// retrieveAllClients retrieves all clients from the database and sends them as a JSON response.
// It checks if the request method is a GET and queries the database for all client records.
// It loops through the result set and appends each client to a slice of Client structs.
// Upon completing the loop, it checks for any errors encountered during iteration.
// Finally, it returns the JSON-encoded slice of clients as the HTTP response body.
func retrieveAllClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, name, working_hours, priority, lead_capacity, existing_leads FROM clients")
	if err != nil {
		http.Error(w, "Failed to execute query", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer rows.Close()

	var clients []Client
	for rows.Next() {
		var c Client
		err := rows.Scan(&c.ID, &c.Name, &c.WorkingHours, &c.Priority, &c.LeadCapacity, &c.ExistingLeads)
		if err != nil {
			http.Error(w, "Failed to scan row", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		clients = append(clients, c)
	}

	err = rows.Err()
	if err != nil {
		http.Error(w, "Error iterating over rows", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clients)
}

// retrieveClient handles a GET request to retrieve the details of a specific client.
// It verifies that the request method is GET and checks for the presence of the "id" query parameter.
// If the client is found in the database, its details are extracted and returned as a JSON response.
// If the client is not found, an appropriate error response is sent.
// If any error occurs during the process, an internal server error is returned with an error log.
func retrieveClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Client ID is missing", http.StatusBadRequest)
		return
	}

	row := db.QueryRow("SELECT * FROM clients WHERE id = ?", id)
	var c Client
	err := row.Scan(&c.ID, &c.Name, &c.WorkingHours, &c.Priority, &c.LeadCapacity, &c.ExistingLeads)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Client not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to scan client", http.StatusInternalServerError)
			log.Println(err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(c)
}

// assignLead retrieves the list of clients from the database, sorts them based on priority and existing leads,
// and assigns a lead to the first suitable client within their working hours. If a client is found and has available
// lead capacity, the function updates the client's existing_leads counter and returns the client ID as a JSON response.
// If no suitable client is found, it returns a "No suitable client found" error response.
func assignLead(w http.ResponseWriter, r *http.Request) {
	currentTime := time.Now()

	rows, err := db.Query("SELECT id, priority, existing_leads, working_hours, lead_capacity FROM clients ORDER BY priority DESC, existing_leads ASC")
	if err != nil {
		http.Error(w, "Failed to retrieve clients", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	clients := make([]Client, 0)
	for rows.Next() {
		var client Client
		err := rows.Scan(&client.ID, &client.Priority, &client.ExistingLeads, &client.WorkingHours, &client.LeadCapacity)
		if err != nil {
			http.Error(w, "Failed to retrieve client data", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		clients = append(clients, client)
	}
	rows.Close()

	for _, client := range clients {
		workingHours, err := parseTimePeriod(client.WorkingHours)
		if err != nil {
			http.Error(w, "Failed to parse working hours", http.StatusInternalServerError)
			log.Println(err)
			return
		}

		if currentTime.After(workingHours.Start) && currentTime.Before(workingHours.End) {
			if client.ExistingLeads < client.LeadCapacity {
				_, err = db.Exec("UPDATE clients SET existing_leads = existing_leads + 1 WHERE id = ?", client.ID)
				if err != nil {
					http.Error(w, "Failed to update client's lead counter", http.StatusInternalServerError)
					log.Println(err)
					return
				}

				w.WriteHeader(http.StatusAccepted)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]int{
					"client_id": client.ID,
				})
			} else {
				continue
			}
			return
		}
	}

	http.Error(w, "No suitable client found", http.StatusNotFound)
}

// parseTimePeriod parses a string representation of a time period in the format "start-end" and returns
// a TimePeriod struct with the parsed start and end times. It also validates the format and checks for errors.
// The start and end times are set to the current year, month, and day.
func parseTimePeriod(s string) (TimePeriod, error) {
	var period TimePeriod
	times := strings.Split(s, "-")
	if len(times) != 2 {
		return period, errors.New("invalid time period format")
	}

	start, err := time.Parse("15:04", times[0])
	if err != nil {
		return period, fmt.Errorf("invalid start time: %v", err)
	}

	end, err := time.Parse("15:04", times[1])
	if err != nil {
		return period, fmt.Errorf("invalid end time: %v", err)
	}

	now := time.Now()
	year, month, day := now.Date()

	start = time.Date(year, month, day, start.Hour(), start.Minute(), 0, 0, start.Location())
	end = time.Date(year, month, day, end.Hour(), end.Minute(), 0, 0, end.Location())

	period.Start = start
	period.End = end

	return period, nil
}
