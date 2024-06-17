package main

import (
	"time"
)

// Client represents a client in the system.
type Client struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	WorkingHours  string `json:"working_hours"`
	Priority      int    `json:"priority"`
	LeadCapacity  int    `json:"lead_capacity"`
	ExistingLeads int    `json:"existing_leads"`
}

// TimePeriod represents a period of time with a start and end time. The start and end times are of type time.Time.
type TimePeriod struct {
	Start time.Time
	End   time.Time
}
