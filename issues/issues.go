package issues

import (
	"fmt"
	"github.com/google/go-github/github"
	"github.com/kr/pretty"
	"github.com/prometheus/alertmanager/notify"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"log"
)

type Client struct {
	githubClient *github.Client
	owner        string
	repo         string
}

func NewClient(owner, repo, authToken string) *Client {
	if authToken == "" {
		// get auth token from the environment.
		fmt.Println("TODO: read GITHUB_AUTH_TOKEN from the environment.")
	}
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)
	return &Client{
		githubClient: github.NewClient(oauth2.NewClient(ctx, tokenSource)),
		owner:        owner,
		repo:         repo,
	}
}

// ListOpenIssues returns open issues from the github
func (c *Client) ListOpenIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue
	ctx := context.Background()

	log.Printf("Listing issues:")
	opts := &github.IssueListByRepoOptions{State: "open"}
	for {
		issues, resp, err := c.githubClient.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
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
func (c *Client) CreateIssue(title, body string, msg *notify.WebhookMessage) (*github.Issue, error) {
	log.Printf("Creating issue: %s", title)
	issueReq := github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Create the issue.
	issue, _, err := c.githubClient.Issues.Create(context.Background(), c.owner, c.repo, &issueReq)
	if err != nil {
		return nil, err
	}

	log.Printf("Created new issue: %s\n", pretty.Sprint(issue))
	return issue, nil
}

// closeIssue on github.
func (c *Client) CloseIssue(issue *github.Issue) error {
	log.Printf("Closing issue: %s\n", *issue.Title)
	state := "closed"
	issueReq := github.IssueRequest{
		State: &state,
	}

	// Set the issue state to "closed".
	_, _, err := c.githubClient.Issues.Edit(context.Background(), c.owner, c.repo, *issue.Number, &issueReq)
	log.Printf("Closed issue: %s\n", *issue.Title)
	return err
}
