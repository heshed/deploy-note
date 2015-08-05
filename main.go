package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-querystring/query"
	"github.com/heshed/go-github/github"
	"github.com/fatih/set"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
	"log"
	"bytes"
)

const (
	baseGitHubURL       = "https://api.github.com/"
	headerRateLimit     = "X-RateLimit-Limit"
	headerRateRemaining = "X-RateLimit-Remaining"
	headerRateReset     = "X-RateLimit-Reset"

	noteTemplate = `
{{ .DeployDate }} {{ Title }}

배포 시간

{{ .MilestoneDate }}

배포 내역

{{ .IssueSummary }}

배포 버전

{{ .RepoVersion }}

참조

{{ .MentionedPersons }}

배포 공지

통검프론트 :
통검 공통 템플릿 :
`
)

type Note struct {
	DeployDate		 string
	Title			 string
	MilestoneDate    string
	IssueSummary     string
	RepoVersion      string
	MentionedPersons string
	Mentioned		 set.Set
}

func (n *Note) Merge(m *Note) {
	n.MilestoneDate += m.MilestoneDate
	n.IssueSummary += m.IssueSummary
	n.RepoVersion += m.RepoVersion
	n.Mentioned.Merge(&m.Mentioned)
}

// TODO: parsing mentions
func getMensionedPersons(body *string) string {
	re := regexp.MustCompile(`.*관련 담당자 :(.*)`)
	mentioned := re.FindString(*body)

	fmt.Println("find :", mentioned)
	return mentioned
}

// addOptions adds the parameters in opt as URL query parameters to s.  opt
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opt interface{}) (string, error) {
	v := reflect.ValueOf(opt)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opt)
	if err != nil {
		return s, err
	}

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

// Response is a GitHub API response.  This wraps the standard http.Response
// returned from GitHub and provides convenient access to things like
// pagination links.
type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of
	// results.  Any or all of these may be set to the zero value for
	// responses that are not part of a paginated set, or for which there
	// are no additional pages.

	NextPage  int
	PrevPage  int
	FirstPage int
	LastPage  int

	github.Rate
}

// newResponse creates a new Response for the provided http.Response.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.populatePageValues()
	response.populateRate()
	return response
}

// populatePageValues parses the HTTP Link response headers and populates the
// various pagination link values in the Reponse.
func (r *Response) populatePageValues() {
	if links, ok := r.Response.Header["Link"]; ok && len(links) > 0 {
		for _, link := range strings.Split(links[0], ",") {
			segments := strings.Split(strings.TrimSpace(link), ";")

			// link must at least have href and rel
			if len(segments) < 2 {
				continue
			}

			// ensure href is properly formatted
			if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
				continue
			}

			// try to pull out page parameter
			url, err := url.Parse(segments[0][1 : len(segments[0])-1])
			if err != nil {
				continue
			}
			page := url.Query().Get("page")
			if page == "" {
				continue
			}

			for _, segment := range segments[1:] {
				switch strings.TrimSpace(segment) {
				case `rel="next"`:
					r.NextPage, _ = strconv.Atoi(page)
				case `rel="prev"`:
					r.PrevPage, _ = strconv.Atoi(page)
				case `rel="first"`:
					r.FirstPage, _ = strconv.Atoi(page)
				case `rel="last"`:
					r.LastPage, _ = strconv.Atoi(page)
				}

			}
		}
	}
}

// populateRate parses the rate related headers and populates the response Rate.
func (r *Response) populateRate() {
	if limit := r.Header.Get(headerRateLimit); limit != "" {
		r.Rate.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := r.Header.Get(headerRateRemaining); remaining != "" {
		r.Rate.Remaining, _ = strconv.Atoi(remaining)
	}
	if reset := r.Header.Get(headerRateReset); reset != "" {
		if v, _ := strconv.ParseInt(reset, 10, 64); v != 0 {
			r.Rate.Reset = github.Timestamp{time.Unix(v, 0)}
		}
	}
}

// Do sends an API request and returns the API response.  The API response is
// JSON decoded and stored in the value pointed to by v, or returned as an
// error if an API error has occurred.  If v implements the io.Writer
// interface, the raw response body will be written to v, without attempting to
// first decode it.
func GetResponse(c *http.Client, req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	response := newResponse(resp)

	// rate := response.Rate

	err = github.CheckResponse(resp)
	if err != nil {
		// even though there was an error, we still return the response
		// in case the caller wants to inspect it further
		return response, err
	}

	if v != nil {
		if w, ok := v.(io.Writer); ok {
			io.Copy(w, resp.Body)
		} else {
			err = json.NewDecoder(resp.Body).Decode(v)
		}
	}
	return response, err
}

type GitHub struct {
	apiURL   string
	user     string
	password string
}

func (g *GitHub) ListByRepo(owner string, repo string, opt *github.IssueListByRepoOptions) ([]github.Issue, *Response, error) {
	u := fmt.Sprintf("%srepos/%v/%v/issues", g.apiURL, owner, repo)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	if g.user != "" && g.password != "" {
		req.SetBasicAuth(g.user, g.password)
	}

	issues := new([]github.Issue)
	client := &http.Client{}
	resp, err := GetResponse(client, req, issues)
	if err != nil {
		return nil, resp, err
	}

	return *issues, resp, err
}

func (g *GitHub) GetNotes(owner string, repo string, milestoneID string) (*Note, error) {
	opt := github.IssueListByRepoOptions{Milestone: milestoneID, State: "all"}

	note := &Note{}
	issues, _, err := g.ListByRepo(owner, repo, &opt)
	if err != nil {
		return note, err
	}

	var buf bytes.Buffer

	for _, issue := range issues {
		summary := fmt.Sprintf("- %v [%s #%d / %s](%s)\n", issue.Labels, repo, *issue.Number, *issue.Title, *issue.HTMLURL)
		note.IssueSummary += summary

		note.RepoVersion = fmt.Sprintf("- [%s:%s]()\n", repo, *issue.Milestone.Title)

		// TODO: check Mentions
		/*
		m := getMensionedPersons(issue.Body)
		if m != "" {
			note.Mentioned.Add(m)
		}
		*/

		note.MilestoneDate = fmt.Sprintf("- %s:%s 10:00\n", repo, issue.Milestone.DueOn.Format("2006-01-02"))
		note.DeployDate = issue.Milestone.DueOn.Format("2006.01.02"))
	}

	fmt.Println("after mention:", buf.String())

	return note, nil
}

func getUsage() string {
	usage := `
export GITHUB_URL=https://enterprise.github.com/api/v3/
export CLIENT_ID=user
export CLIENT_SECRET=password
export OWNER=heshed
export REPOS=milestones-test:milestones-test
export MILESTONE_ID=1
deploy-note
`
	return usage
}

// TODO: classify by Labels
func main() {
	gitHubURL := baseGitHubURL
	url := os.Getenv("GITHUB_URL")
	if url != "" {
		gitHubURL = url
	}
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	owner := os.Getenv("OWNER")
	milestoneID := os.Getenv("MILESTONE_ID")
	repos := os.Getenv("REPOS")

	// check arguments..
	if clientID == "" || clientSecret == "" || gitHubURL == "" || owner == "" || milestoneID == "" || repos == "" {
		fmt.Println(getUsage())
		os.Exit(0)
	}

	var note Note
	hub := GitHub{
		apiURL:   gitHubURL,
		user:     clientID,
		password: clientSecret,
	}

	// TODO: magic string
	note.Title = "통합검색 배포 안내드립니다."

	for _, repo := range strings.Split(repos, ":") {
		n, err := hub.GetNotes(owner, repo, milestoneID)
		if err != nil {
			log.Fatalln("error: %v", err)
		}
		note.Merge(n)
	}
	note.MentionedPersons = note.Mentioned.String()

	t := template.Must(template.New("note").Parse(noteTemplate))
	err := t.Execute(os.Stdout, note)
	if err != nil {
		log.Fatalln("template error:", err)
	}
}
