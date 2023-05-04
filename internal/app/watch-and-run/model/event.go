package model

import "time"

type Event struct {
	Id        int       `json:"id" db:"id"`
	Path      string    `json:"path" db:"path"`
	FileName  string    `json:"file_name" db:"file_name"`
	EventType string    `json:"type" db:"type"`
	Time      time.Time `json:"time" db:"time"`
}
