package main
import (
	"github.com/google/go-github/github"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"os"
)

const noteTemplate = `
배포 시간

{{ .MilestoneDate }}

배포 내역

{{ .IssueSummary }}

배포 버전

{{ .RepoVersion }}

참조

{{ .MentionedPersons }}
@@usf @lulu

배포 공지

통검프론트 :
통검 공통 템플릿 :
`

type Note struct {
	MilestoneDate string
	IssueSummary string
	RepoVersion string
	MentionedPersons string
}

func (n *Note) Merge(m Note) {
	n.MilestoneDate += m.MilestoneDate
	n.IssueSummary += m.IssueSummary
	n.RepoVersion += m.RepoVersion
	n.MentionedPersons += m.MentionedPersons
}

func getMensionedPersons(body *string) string {
	re := regexp.MustCompile(`.*관련 담당자 :.*`)
	mentioned := re.FindString(*body)

	return strings.Replace(mentioned, "관련 담당자 :", "", 1)
}

func GetNotes(orgs string, repo string, milestone string) Note {
	client := github.NewClient(nil)
	opt := github.IssueListByRepoOptions{Milestone: milestone, State: "all"}

	var note Note
	issues, _, err := client.Issues.ListByRepo(orgs, repo, &opt)
	if err != nil {
		fmt.Printf("error: %v\n\n", err)
		return note
	}


	for _, issue := range issues {
		summary := fmt.Sprintf("- %v [%s #%d / %s](%s)\n", issue.Labels, repo, *issue.Number, *issue.Title, *issue.HTMLURL)
		note.IssueSummary += summary

		note.RepoVersion = repo + ":" + *issue.Milestone.Title + "\n"

		m := getMensionedPersons(issue.Body)
		if m != "" {
			note.MentionedPersons += m
		}

		note.MilestoneDate = repo + ":" + issue.Milestone.DueOn.Format("2006-01-02") + " 10:00 \n"
	}

	return note
}

func main() {
	orgs := os.Getenv("ORGS")
	milestone := os.Getenv("MILESTONE")
	repos := os.Getenv("REPOS")

	var note Note
	for _, repo := range strings.Split(repos, ":") {
		n := GetNotes(orgs, repo, milestone)
		note.Merge(n)
	}

	t := template.Must(template.New("note").Parse(noteTemplate))
	err := t.Execute(os.Stdout, note)
	if err == nil {
		fmt.Fprintln(os.Stderr, err)
	}

}