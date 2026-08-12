package api

import "time"

// Condition is one entry of the status conditions every mNi Cloud resource
// reports. It tells what the controller behind a resource made of it.
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}
