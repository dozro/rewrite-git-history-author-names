package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	// flags
	oldName := flag.String("oldName", "", "Old name")
	oldEmail := flag.String("oldEmail", "", "Old email")
	newName := flag.String("newName", "", "New name")
	newEmail := flag.String("newEmail", "", "New email")
	signCommits := flag.Bool("signCommits", false, "Sign commits")
	signOnBranch := flag.String("signOnBranch", "resigned", "Sign on branch, be careful – this might BREAK your repo")
	flag.Parse()

	if *oldName == "" || *oldEmail == "" || *newName == "" || *newEmail == "" {
		flag.PrintDefaults()
		log.Fatal("Please specify a new name and a new email")
	}

	// Prepare fast-export and fast-import
	exportCmd := exec.Command("git", "fast-export", "--all", "--signed-tags=warn-strip")
	importCmd := exec.Command("git", "fast-import", "--force")

	// Pipe data between them
	exportOut, err := exportCmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error connecting fast-export output:", err)
		os.Exit(1)
	}
	importIn, err := importCmd.StdinPipe()
	if err != nil {
		fmt.Println("Error connecting fast-import input:", err)
		os.Exit(1)
	}

	exportCmd.Stderr = os.Stderr
	importCmd.Stdout = os.Stdout
	importCmd.Stderr = os.Stderr

	// Start commands
	if err := exportCmd.Start(); err != nil {
		fmt.Println("Error starting git fast-export:", err)
		os.Exit(1)
	}
	if err := importCmd.Start(); err != nil {
		fmt.Println("Error starting git fast-import:", err)
		os.Exit(1)
	}

	// Transform the export stream
	go func() {
		defer importIn.Close()
		reader := bufio.NewReader(exportOut)
		writer := bufio.NewWriter(importIn)

		inCommitMsg := false
		msgBytesRemaining := 0

		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Println("Error reading fast-export stream:", err)
				break
			}

			// Detect start of commit message block
			if strings.HasPrefix(line, "data ") {
				// "data <len>\n<message>"
				var length int
				fmt.Sscanf(line, "data %d", &length)
				msgBytesRemaining = length
				inCommitMsg = true
			}

			if strings.HasPrefix(line, "author ") || strings.HasPrefix(line, "committer ") {
				line = rewriteAuthor(line, *oldEmail, *newName, *newEmail)
			}

			// If entering commit message block, rewrite its contents
			if inCommitMsg && msgBytesRemaining > 0 {
				// Read the full message body (may span multiple lines)
				msg := make([]byte, msgBytesRemaining)
				n, _ := io.ReadFull(reader, msg)
				if n > 0 {
					newMsg := rewriteSignoffs(string(msg), *oldName, *oldEmail, *newName, *newEmail)
					msgBytesRemaining = len(newMsg)
					// Rewrite the preceding "data " line with new length
					_, err := writer.WriteString(fmt.Sprintf("data %d\n", msgBytesRemaining))
					if err != nil {
						log.Println("Error writing fast-export stream:", err)
					}
					_, err = writer.WriteString(newMsg)
					if err != nil {
						log.Println("Error writing fast-export stream:", err)
					}
				}
				inCommitMsg = false
				msgBytesRemaining = 0
				continue
			}
			writer.WriteString(line)

			if err == io.EOF {
				break
			}
		}
		writer.Flush()
	}()
	// Wait for completion
	exportCmd.Wait()
	importCmd.Wait()

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

// rewriteAuthor replaces the author or committer line fully, keeping the timestamp.
func rewriteAuthor(line, oldEmail, newName, newEmail string) string {
	if !strings.Contains(line, "<"+oldEmail+">") {
		return line
	}

	// "author Old Name <old@example.com> 1700000000 +0000"
	parts := strings.SplitN(line, ">", 2)
	if len(parts) != 2 {
		return line
	}

	// parts[1] contains the rest (timestamp/timezone)
	rest := parts[1]
	return fmt.Sprintf("%s %s <%s>%s", strings.Split(line, " ")[0], newName, newEmail, rest)
}

// rewriteSignoffs replaces Signed-off-by lines in commit messages
func rewriteSignoffs(msg, oldName, oldEmail, newName, newEmail string) string {
	lines := strings.Split(msg, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.ToLower(l), "signed-off-by:") &&
			strings.Contains(l, "<"+oldEmail+">") {
			lines[i] = fmt.Sprintf("Signed-off-by: %s <%s>", newName, newEmail)
		}
	}
	return strings.Join(lines, "\n")
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
