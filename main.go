package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"
)

func bold(s string) string {
	return Bold + s + Reset
}

func dim(s string) string {
	return Dim + s + Reset
}

type Issue struct {
	Number int
	Title  string
	Body   string
}

type IssueEvent struct {
	Event     string     `json:"event"`
	CommitID  *string    `json:"commit_id,omitempty"`
	Milestone *Milestone `json:"milestone,omitempty"`
	Rename    *Rename    `json:"rename,omitempty"`
	Label     *Label     `json:"label,omitempty"`
}

type Milestone struct {
	Title string `json:"title"`
}

type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Label struct {
	Name string `json:"name"`
}

func findGitRoot(path string) (string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.Name() == ".git" {
			return path, nil
		}
	}
	return findGitRoot(path + "/..")
}

func getApiUrl(user string, repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", user, repo)
}

func getUserAndRepo(content string) (string, string, error) {
	user := ""
	repo := ""
	for line := range strings.Lines(content) {
		if strings.Contains(line, "url") {
			url := strings.Split(line, "=")[1]
			// https://github.com/user/repo.git
			// git@github.com:user/repo.git
			userAndRepo := ""
			if strings.Contains(url, "https") {
				userAndRepo = url[20:]
			} else if strings.Contains(url, "git@") {
				userAndRepo = url[16:]
			} else {
				return "", "", fmt.Errorf("Unknown url: %s", url)
			}

			user = strings.Split(userAndRepo, "/")[0]
			repo = strings.Trim(strings.Split(userAndRepo, "/")[1], "\n\t\r")
			if strings.Contains(repo, ".git") {
				repo = repo[:len(repo)-5]
			}
			break
		}
	}
	if user == "" || repo == "" {
		return "", "", fmt.Errorf("Could not find user and repo")
	}
	return user, repo, nil
}

func main() {
	cwd, _ := os.Getwd()

	gitRoot, err := findGitRoot(cwd)
	if err != nil {
		panic(err)
	}

	gitConfig := gitRoot + "/.git/config"

	bytes, err := os.ReadFile(gitConfig)
	if err != nil {
		panic(err)
	}
	content := string(bytes)

	user, repo, err := getUserAndRepo(content)
	if err != nil {
		panic(err)
	}

	args := os.Args[1:]

	var issueId *int

	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			issueId = &n
		} else {
			fmt.Fprintf(os.Stderr, "Ошибка: '%s' не является числом\n", args[0])
			os.Exit(1)
		}
	}

	client := &http.Client{}

	if issueId != nil {
		err = runWithId(client, user, repo, *issueId)
	} else {
		err = run(client, user, repo)
	}
	if err != nil {
		panic(err)
	}

}

func getFromGithub(client *http.Client, url string) ([]byte, error) {
	res, err := client.Get(url)
	if err != nil {
		return []byte{}, fmt.Errorf("Error: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return []byte{}, fmt.Errorf("Error: %d", res.StatusCode)
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, fmt.Errorf("Error: %s", err)
	}

	return bodyBytes, nil
}

func runWithId(client *http.Client, user, repo string, issueId int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/events", user, repo, issueId)

	bodyBytes, err := getFromGithub(client, url)
	if err != nil {
		return err
	}

	var events []IssueEvent
	err = json.Unmarshal([]byte(bodyBytes), &events)
	if err != nil {
		return err
	}

	fmt.Println("events for issue", issueId)
	for i, e := range events {
		if e.CommitID != nil {
			fmt.Printf("  - %d: commit id %s\n", i, dim((*e.CommitID)[:7]))
		}
		if e.Milestone != nil {
			fmt.Printf("  - %d: milestone %s\n", i, bold(e.Milestone.Title))
		}
		if e.Rename != nil {
			fmt.Printf("  - %d: renamed\n", i)
			fmt.Println("    -", e.Rename.From)
			fmt.Println("    -", e.Rename.To)
		}
		if e.Label != nil {
			fmt.Printf("  - %d: label %s\n", i, bold(e.Label.Name))
		}
	}

	return nil
}

func run(client *http.Client, user, repo string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", user, repo)

	bodyBytes, err := getFromGithub(client, url)
	if err != nil {
		panic(err)
	}

	var issues []Issue
	err = json.Unmarshal(bodyBytes, &issues)
	if err != nil {
		panic(err)
	}

	if len(issues) == 0 {
		fmt.Println("No issues found")
		return nil
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})
	for _, issue := range issues {
		fmt.Println(bold(fmt.Sprintf("%d: %s", issue.Number, issue.Title)))
		fmt.Println("  - " + issue.Body)
		fmt.Println()
	}

	return nil
}
