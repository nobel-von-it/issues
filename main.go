package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

type Issue struct {
	Number int
	Title  string
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
			repo = strings.Split(userAndRepo, "/")[1]
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

	url := getApiUrl(user, repo)
	client := &http.Client{}

	res, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		panic(fmt.Sprintf("Error: %d", res.StatusCode))
	}

	bodyBytes, err := io.ReadAll(res.Body)
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
		return
	}

	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})
	for _, issue := range issues {
		fmt.Printf("%d: %s\n", issue.Number, issue.Title)
	}
}
