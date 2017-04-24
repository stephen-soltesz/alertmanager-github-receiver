package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/kr/pretty"
	"github.com/prometheus/alertmanager/notify"
	"github.com/stephen-soltesz/github-alertmanager-webook/issues"
	"io/ioutil"
	"log"
	"net/http"
)

var (
	authtoken   = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	client      *issues.Client
	githubOwner = flag.String("github-owner", "stephen-soltesz", "Probably the same as github organization.")
	githubRepo  = flag.String("github-repo", "public-issue-test", "The repository name for issues.")
)

// alertReceiverHandler handles AM notifications.
func alertReceiverHandler(w http.ResponseWriter, r *http.Request) {
	// Verify that request is a POST.
	if r.Method != http.MethodPost {
		log.Printf("Client used unsupported method: %s: %s", r.Method, r.RemoteAddr)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read request body.
	alertBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Parse webhook message.
	msg := &notify.WebhookMessage{}
	if err := json.Unmarshal(alertBytes, msg); err != nil {
		log.Printf("Failed to parse webhook message from %s: %s", r.RemoteAddr, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Print a debug message.
	pretty.Print(msg)

	// Handle the webhook message.
	log.Printf("Handling alert: %s", id(msg))
	if err := handleAlert(msg); err != nil {
		log.Printf("Failed to handle alert: %s: %s", id(msg), err)
		return
	}
	log.Printf("Completed alert: %s", id(msg))
}

// handleAlert performs all handling of a webhook message.
func handleAlert(msg *notify.WebhookMessage) error {
	// List known issues from github.
	issues, err := client.ListOpenIssues()
	if err != nil {
		return err
	}

	// Search for an issue that matches the notification message from AM.
	msgTitle := formatTitle(msg)
	var foundIssue *github.Issue
	for _, issue := range issues {
		if msgTitle == *issue.Title {
			log.Printf("Found matching issue: %s\n", msgTitle)
			foundIssue = issue
			break
		}
	}

	// The message is currently firing and we did not find a matching
	// issue from github, so create a new issue.
	if msg.Data.Status == "firing" && foundIssue == nil {
		msgBody := formatIssueBody(msg)
		_, err := client.CreateIssue(msgTitle, msgBody, msg)
		return err
	}

	// The message is resolved and we found a matching open issue from github,
	// so close the issue.
	if msg.Data.Status == "resolved" && foundIssue != nil {
		// NOTE: there will be multiple "resolved" messages for the same
		// alert. Prometheus evaluates rules every `evaluation_interval`.
		// And, alertmanager preserves an alert until `resolve_timeout`. So
		// expect (resolve_timeout / evaluation_interval) messages.
		return client.CloseIssue(foundIssue)
	}

	return fmt.Errorf("Unsupported WebhookMessage.Data.Status: %s", msg.Data.Status)
}

// issueViewerHandler lists all issues from github.
func issueViewerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><body>\n")

	fmt.Fprintf(w, "<table>\n")
	issueList, err := client.ListOpenIssues()
	if err != nil {
		fmt.Fprintf(w, "%s\n", err)
		return
	}
	for _, issue := range issueList {
		fmt.Fprintf(w, "<tr>\n")
		pretty.Print(issue)
		fmt.Fprintf(w, "<td><a href=%q>%s</a></td>\n", *issue.HTMLURL, *issue.Title)
		fmt.Fprintf(w, "</tr>\n")
	}
	fmt.Fprintf(w, "</table>\n")
	fmt.Fprintf(w, "</body></html>\n")
}

func id(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("0x%x", msg.GroupKey)
}

// formatTitle constructs an issue title from a webhook message.
func formatTitle(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("[%s] %s\n", id(msg), msg.Data.GroupLabels["alertname"])
}

// formatIssueBody constructs an issue body from a webhook message.
func formatIssueBody(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("Original alert: %s\nTODO: add graph url from annotations.", msg.ExternalURL)
}

func serveListener() {
	http.HandleFunc("/", issueViewerHandler)
	http.HandleFunc("/v1/receiver", alertReceiverHandler)
	http.ListenAndServe(":5100", nil)
}

func main() {
	flag.Parse()

	client = issues.NewClient(*githubOwner, *githubRepo, *authtoken)
	serveListener()
}
