package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	// flags
	signCommits := flag.Bool("signCommits", true, "Sign commits")
	signOnBranch := flag.String("signOnBranch", "resigned", "Sign on branch, be careful – this might BREAK your repo")
	flag.Parse()

	if *signCommits {
		// You can change HEAD to another branch name if needed
		revision := "HEAD"

		// Get the commit list in topological order (oldest to newest)
		commitsRaw, err := runGit("rev-list", "--reverse", revision)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list commits: %v\n", err)
			os.Exit(1)
		}

		commits := strings.Split(commitsRaw, "\n")
		newCommitMap := make(map[string]string)
		for i, commit := range commits {
			fmt.Printf("Re-signing commit %d/%d: %s\n", i+1, len(commits), commit)

			tree, _ := runGit("show", "-s", "--format=%T", commit)
			parentsRaw, _ := runGit("show", "-s", "--format=%P", commit)
			msg, _ := runGit("show", "-s", "--format=%B", commit)

			an, _ := runGit("show", "-s", "--format=%an", commit)
			ae, _ := runGit("show", "-s", "--format=%ae", commit)
			ad, _ := runGit("show", "-s", "--format=%aI", commit)

			cn, _ := runGit("show", "-s", "--format=%cn", commit)
			ce, _ := runGit("show", "-s", "--format=%ce", commit)
			cd, _ := runGit("show", "-s", "--format=%cI", commit)

			parents := strings.Fields(parentsRaw)
			var parentArgs []string
			for _, p := range parents {
				if newParent, ok := newCommitMap[p]; ok {
					parentArgs = append(parentArgs, "-p", newParent)
				} else {
					parentArgs = append(parentArgs, "-p", p)
				}
			}

			// Create the new signed commit using git commit-tree -S
			cmd := exec.Command("git", append([]string{"commit-tree", "-S", tree}, parentArgs...)...)
			cmd.Stdin = bytes.NewBufferString(msg)
			cmd.Stderr = os.Stderr
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("GIT_AUTHOR_NAME=%s", an),
				fmt.Sprintf("GIT_AUTHOR_EMAIL=%s", ae),
				fmt.Sprintf("GIT_AUTHOR_DATE=%s", ad),
				fmt.Sprintf("GIT_COMMITTER_NAME=%s", cn),
				fmt.Sprintf("GIT_COMMITTER_EMAIL=%s", ce),
				fmt.Sprintf("GIT_COMMITTER_DATE=%s", cd),
			)

			out, err := cmd.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error signing commit %s: %v\n", commit, err)
				os.Exit(1)
			}

			newHash := strings.TrimSpace(string(out))
			newCommitMap[commit] = newHash
		}

		// The last commit in the chain is the new HEAD
		newHead := newCommitMap[commits[len(commits)-1]]

		fmt.Printf("Resetting branch to new head %s\n", newHead)
		if _, err := runGit("update-ref", fmt.Sprintf("refs/heads/%s", *signOnBranch), newHead); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update ref: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("All commits have been re-signed and written to branch '%s'.\n", *signOnBranch)
		fmt.Println("You can inspect it using:")
		fmt.Printf("  git log --show-signature %s\n", *signOnBranch)
	}

	fmt.Println("All commits rewritten successfully!")
	fmt.Println("Run cleanup and push:")
	reflog := exec.Command("git", "reflog", "expire", "--expire=now", "--all")
	reflog.Stdout = os.Stdout
	reflog.Stderr = os.Stderr

	if err := reflog.Run(); err != nil {
		fmt.Println("Error running git reflog:", err)
	}
	fmt.Println("  git reflog expire --expire=now --all && git gc --prune=now --aggressive")
	fmt.Println("  git push --force --tags origin 'refs/heads/*'")
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
