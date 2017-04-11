package main

import (
	//"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/kr/pretty"
	"github.com/prometheus/alertmanager/notify"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/http"
)

var (
	authtoken          = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	githubOwner        = flag.String("github-owner", "stephen-soltesz", "Probably the same as github organization.")
	githubRepo         = flag.String("github-repo", "public-issue-test", "The repository name for issues.")
	githubRecentIssues = flag.Int("github-recent-issues", 24, "Search issues created in the last N hours.")
	client             *github.Client
)

// alertReceiverHandler handles AM notifications.
func alertReceiverHandler(w http.ResponseWriter, r *http.Request) {
	// Verify that request is a POST.
	if r.Method != http.MethodPost {
		log.Printf("Client used unsupported method: %s: %s", r.Method, r.RemoteAddr)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

	// Read request body.
	alertBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
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
	}
	log.Printf("Completed alert: %s", id(msg))
}

// handleAlert performs all handling of a webhook message.
func handleAlert(msg *notify.WebhookMessage) error {
	// List known issues from github.
	issues, err := listIssues()
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
		_, err := createIssue(msgTitle, msg)
		return err
	}

	// The message is resolved and we found a matching open issue from github,
	// so close the issue.
	if msg.Data.Status == "resolved" && foundIssue != nil {
		// NOTE: there will be multiple "resolved" messages for the same
		// alert. Prometheus evaluates rules every `evaluation_interval`.
		// And, alertmanager preserves an alert until `resolve_timeout`. So
		// expect (resolve_timeout / evaluation_interval) messages.
		return closeIssue(foundIssue)
	}

	return fmt.Errorf("Unsupported WebhookMessage.Data.Status: %s", msg.Data.Status)
}

// issueViewerHandler lists all issues from github.
func issueViewerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><body>okay</body></html>")
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

// listIssues from github.
func listIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue
	ctx := context.Background()

	log.Printf("Listing issues:")
	opts := &github.IssueListByRepoOptions{State: "open"} // Since: time.Now().Add(-(time.Duration(*githubRecentIssues)) * time.Hour)}
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, *githubOwner, *githubRepo, opts)
		if err != nil {
			log.Printf("Failed to list github repos: %s", err)
			return nil, err
		}
		for _, issue := range issues {
			fmt.Printf("  %s / %d\n", *issue.Title, *issue.Number)
			allIssues = append(allIssues, issue)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	return allIssues, nil
}

// createIssue on github.
func createIssue(title string, msg *notify.WebhookMessage) (*github.Issue, error) {
	log.Printf("Creating issue: %s", title)
	body := formatIssueBody(msg)
	issueReq := github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Create the issue.
	issue, _, err := client.Issues.Create(context.Background(), *githubOwner, *githubRepo, &issueReq)
	if err != nil {
		return nil, err
	}

	log.Printf("Created new issue: %s\n", pretty.Sprint(issue))
	return issue, nil
}

// closeIssue on github.
func closeIssue(issue *github.Issue) error {
	log.Printf("Closing issue: %s\n", *issue.Title)
	state := "closed"
	issueReq := github.IssueRequest{
		State: &state,
	}

	// Set the issue state to "closed".
	_, _, err := client.Issues.Edit(context.Background(), *githubOwner, *githubRepo, *issue.Number, &issueReq)
	log.Printf("Closed issue: %s\n", *issue.Title)
	return err
}

func serveListener() {
	http.HandleFunc("/", issueViewerHandler)
	http.HandleFunc("/v1/receiver", alertReceiverHandler)
	http.ListenAndServe(":5100", nil)
}

func main() {
	flag.Parse()

	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *authtoken},
	)

	client = github.NewClient(oauth2.NewClient(ctx, tokenSource))
	serveListener()
}

/*
	opt := &github.RepositoryListOptions{Affiliation: "owner"}

	for {
		// list all repositories for the authenticated user
		repos, resp, err := client.Repositories.List(ctx, "stephen-soltesz", opt)
		if err != nil {
			panic(err)
		}
		for _, repo := range repos {
			fmt.Printf("%s %-20s %s\n", *repo.Owner.Login, *repo.Name, *repo.GitURL)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}
*/
