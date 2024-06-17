package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// clientsTest is a variable of type []Client used for testing purposes.
// It contains an array of Client struct instances with different properties.
var clientsTest = [...]Client{
	{Name: "Client One", WorkingHours: "09:00-17:00", Priority: 3, LeadCapacity: 10, ExistingLeads: 10},
	{Name: "Client Two", WorkingHours: "12:00-20:00", Priority: 3, LeadCapacity: 5, ExistingLeads: 0},
	{Name: "Client Three", WorkingHours: "16:30-00:30", Priority: 2, LeadCapacity: 7, ExistingLeads: 0},
}

// TestCreateNewClient is a unit test function that tests the createNewClient and retrieveClient handlers.
// It sets up a test database connection and starts a local server. It then sends multiple POST requests to create new clients,
// verifies the HTTP status codes and the response payloads, and uses the client ID from the response to fetch the client details.
// It checks if the retrieved client details match the expected values.
// The function utilizes the clientsTest variable for testing purposes, which is an array of Client struct instances with different properties.
// This test function is designed to be used with the Go testing package and should be executed using the "go test" command.
func TestCreateNewClient(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite3", "./test-clients.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, working_hours TEXT, priority INTEGER, lead_capacity INTEGER, existing_leads INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}

	go startServer()

	time.Sleep(1 * time.Second)

	for _, client := range clientsTest {
		jsonClient, err := json.Marshal(client)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest("POST", "/clients/new", bytes.NewBuffer(jsonClient))
		if err != nil {
			t.Fatal(err)
		}

		resp := httptest.NewRecorder()
		createNewClient(resp, req)

		if status := resp.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusCreated)
		}

		var response map[string]int
		err = json.Unmarshal(resp.Body.Bytes(), &response)
		if err != nil {
			t.Fatal(err)
		}

		// Using the client ID from the http response, fetch the client details
		req, err = http.NewRequest("GET", "/client?id="+fmt.Sprint(response["id"]), nil)
		if err != nil {
			t.Fatal(err)
		}

		resp = httptest.NewRecorder()
		retrieveClient(resp, req)

		if status := resp.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}

		var clientTest Client
		err = json.Unmarshal(resp.Body.Bytes(), &clientTest)
		if err != nil {
			t.Fatal(err)
		}

		// Check the client details match what we expect
		if clientTest.Name != client.Name ||
			clientTest.WorkingHours != client.WorkingHours ||
			clientTest.Priority != client.Priority ||
			clientTest.LeadCapacity != client.LeadCapacity ||
			clientTest.ExistingLeads != client.ExistingLeads {
			t.Errorf("handler returned wrong client details: got %v want %v", clientTest, client)
		}
	}
}

// TestRetrieveAllClients is a unit test function that tests the retrieval of all clients from the server.
// It sets up a test database connection and sends a GET request to retrieve all clients.
// It verifies the HTTP status code and decodes the response into a slice of Client structs.
// It then checks if the number of returned clients matches the expected value of 3.
// The function utilizes the db and Client variables, which are declared globally.
// This test function is designed to be used with the Go testing package and should be executed using the "go test" command.
func TestRetrieveAllClients(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite3", "./test-clients.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	resp, err := http.Get("http://localhost:8080/clients")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var clients []Client

	err = json.NewDecoder(resp.Body).Decode(&clients)
	if err != nil {
		t.Fatal(err)
	}

	if len(clients) != 3 {
		t.Errorf("Expected only 3 clients in the response, but received %d", len(clients))
	}
}

// TestAssignLead is a unit test function that tests the assignLead handler.
// It sets up a test database connection.
// It sends a GET request to the /assignLead endpoint and verifies the HTTP status code.
// It also checks if the response body contains the correct client ID.
// The client ID is expected to be 2.
// The test function utilizes the global db variable, which is a database connection.
// The test database file is created at "./test-clients.db" and is removed after the test.
// This test function is designed to be used with the Go testing package and should be executed using the "go test" command.
func TestAssignLead(t *testing.T) {
	var err error
	db, err = sql.Open("sqlite3", "./test-clients.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("./test-clients.db")
	defer db.Close()

	resp, err := http.Get("http://localhost:8080/assignLead")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("Expected status 202, got: %v", resp.StatusCode)
	}

	respBody := struct {
		ClientID int `json:"client_id"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		t.Fatal(err)
	}

	expectedID := 2

	if respBody.ClientID != expectedID {
		t.Fatalf("Client ID mismatch, got: %v, want: %v", respBody.ClientID, expectedID)
	}
}

// TestParseTimePeriod is a unit test function that tests the parseTimePeriod function.
// It defines multiple test cases with different inputs and expected outputs.
// For each test case, it calls parseTimePeriod and verifies the result and error status.
// If the test case is not expected to return an error, it also checks if the parsed TimePeriod matches the expected TimePeriod.
// This test function is designed to be used with the Go testing package and should be executed using the "go test" command.
func TestParseTimePeriod(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected TimePeriod
		IsError  bool
	}{
		{
			Input: "15:00-16:00",
			Expected: TimePeriod{
				Start: time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 15, 0, 0, 0, time.Local),
				End:   time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 16, 0, 0, 0, time.Local),
			},
			IsError: false,
		},
		{
			Input:    "non-time-format",
			Expected: TimePeriod{},
			IsError:  true,
		},
	}

	for _, tc := range testCases {
		// Calculate period and verify result
		res, err := parseTimePeriod(tc.Input)
		if (err != nil) != tc.IsError {
			t.Fatalf("Error status: %v, want: %v for input: %v", (err != nil), tc.IsError, tc.Input)
		}

		if !tc.IsError {
			if !(res.Start.Hour() == tc.Expected.Start.Hour() &&
				res.Start.Minute() == tc.Expected.Start.Minute() &&
				res.End.Hour() == tc.Expected.End.Hour() &&
				res.End.Minute() == tc.Expected.End.Minute()) {
				t.Fatalf("TimePeriod mismatch, got: %v - %v, want: %v - %v for input: %v",
					res.Start.Format("15:04"), res.End.Format("15:04"),
					tc.Expected.Start.Format("15:04"), tc.Expected.End.Format("15:04"),
					tc.Input)
			}
		}
	}
}
