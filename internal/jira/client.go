package jira

// Client handles communication with the Jira REST API.
type Client struct {
	BaseURL string
	Email   string
	Token   string
}
