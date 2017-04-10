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
	"time"
)

var (
	authtoken          = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	create             = flag.Bool("create", false, "Create a new issue.")
	list               = flag.Bool("list", false, "List open issues.")
	githubOwner        = flag.String("github-owner", "stephen-soltesz", "Probably the same as github organization.")
	githubRepo         = flag.String("github-repo", "public-issue-test", "The repository name for issues.")
	githubRecentIssues = flag.Int("github-recent-issues", 24, "Search issues created in the last N hours.")
	client             *github.Client
)

/*
 * Receive alert.
 * List alerts from github for the last hour from issue repo.
 * if alert is firing, and issue does not exist, then create new issue.
 * if alert is resolved, and issue exists and open, close it.
 */
func alertReceiverHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: verify that method is POST.
	alertBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	// Parse webhook message.
	msg := &notify.WebhookMessage{}
	if err := json.Unmarshal(alertBytes, msg); err != nil {
		log.Printf("Failed to parse webhook message from <IP>: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Printf("%#v\n", msg)
	pretty.Print(msg)

	log.Printf("Handling alert: 0x%x", msg.GroupKey)
	err = handleAlert(msg)
	if err != nil {
		log.Printf("Failed to handle alert: 0x%x: %s", msg.GroupKey, err)
	}
}

// formatTitle constructs an issue title from a webhook message.
func formatTitle(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("[0x%x] %s\n", msg.GroupKey, msg.Data.GroupLabels["alertname"])
}

// formatIssueBody constructs an issue body from a webhook message.
func formatIssueBody(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("Original alert: %s\nTODO: add graph url from annotations.", msg.ExternalURL)
}

// handleAlert performs all handling of a webhook message.
func handleAlert(msg *notify.WebhookMessage) error {
	// * List alerts from github for the last hour from issue repo.
	title := formatTitle(msg)
	log.Printf("Handling message: %s", title)
	issues, err := listIssues()
	if err != nil {
		return err
	}
	var foundIssue *github.Issue
	for _, issue := range issues {
		if title == *issue.Title {
			log.Printf("Found title: %s\n", title)
			foundIssue = issue
			break
		}
	}
	if msg.Data.Status == "firing" {
		log.Printf("Firing alert: 0x%x", msg.GroupKey)
		// if alert is firing and missing from github, create new issue.
		if foundIssue == nil {
			issue, err := createIssue(title, msg)
			if err != nil {
				return err
			}
			log.Printf("Created new issue: %#v\n", issue)
		}
	} else if msg.Data.Status == "resolved" {
		log.Printf("Resolving alert: 0x%x", msg.GroupKey)
		// if alert is resolved and present in github, close issue.
		// - NOTE: there will be multiple resolved messages because prometheus
		// continues to evaluate rules every `evaluation_interval`. And,
		// alertmanager preserves an alert until `resolve_timeout`.
		// resolve_timeout (alertmanager.yml) / evaluation_interval (prometheus.yml)
		// But, as long as we only act on "open" issues, it should be a no-op.
		if foundIssue != nil && *foundIssue.State == "open" {
			log.Printf("Closing issue: %#v\n", foundIssue)
			err = closeIssue(foundIssue)
			if err != nil {
				return err
			}
		}
	} else {
		log.Printf("Unsupported Status: %s", msg.Data.Status)
	}
	log.Printf("Finished handling alert: 0x%x", msg.GroupKey)
	return nil
}

// issueViewerHandler lists all issues from github.
func issueViewerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><body>okay</body></html>")
}

// listIssues from github.
func listIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue
	ctx := context.Background()

	log.Printf("Listing issues")
	opts := &github.IssueListByRepoOptions{Since: time.Now().Add(-(time.Duration(*githubRecentIssues)) * time.Hour)}
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, *githubOwner, *githubRepo, opts)
		if err != nil {
			log.Printf("Failed to list github repos: %s", err)
			return nil, err
		}
		for _, issue := range issues {
			fmt.Printf("%s / %d\n", *issue.Title, *issue.Number)
			allIssues = append(allIssues, issue)
		}
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	log.Printf("Returning all issues")
	return allIssues, nil
}

// createIssue on github.
func createIssue(title string, msg *notify.WebhookMessage) (*github.Issue, error) {
	log.Printf("Creating issue: %s", title)
	ctx := context.Background()
	body := formatIssueBody(msg)
	issueReq := github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	issue, _, err := client.Issues.Create(ctx, *githubOwner, *githubRepo, &issueReq)
	if err != nil {
		log.Printf("Error creating issue: %s", err)
		return nil, err
	}

	// fmt.Println(github.Stringify(resp))
	fmt.Println(github.Stringify(issue))
	return issue, nil
}

// closeIssue on github.
func closeIssue(issue *github.Issue) error {
	ctx := context.Background()
	state := "closed"
	issueReq := github.IssueRequest{
		State: &state,
	}
	// TODO: add comment about what is happening.

	// _, _, err := client.Issues.Edit(ctx, *githubOwner, *githubRepo, *issue.Number, &issueReq)
	_, _, err := client.Issues.Edit(ctx, *githubOwner, *githubRepo, *issue.Number, &issueReq)
	if err != nil {
		return err
	}
	return nil
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
