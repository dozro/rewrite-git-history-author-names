package main

import (
	"bufio"
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
	signAll := flag.Bool("signAll", false, "Sign all commits")
	signSinceHash := flag.String("signSinceHash", "", "Sign since hash")
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
		cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
		rOutput, err := cmd.Output()
		if err != nil {
			fmt.Println("Error executing git command:", err)
			return
		}
		oldestCommitHash := strings.TrimSpace(string(rOutput))
		if *signSinceHash != "" {
			fmt.Printf("your provided commit hash(%s) will override the found commit hash(%s)\n", *signSinceHash, oldestCommitHash)
			oldestCommitHash = strings.TrimSpace(*signSinceHash)
		}
		fmt.Println("Oldest commit hash:", oldestCommitHash)
		if *signAll {
			fmt.Println("Signing all old commits since hash:", oldestCommitHash)
			cmd := exec.Command("git", "rebase", "--exec", "'git commit --amend --no-edit -n -S'", oldestCommitHash)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println("Error running git command:", err)
			}
		} else {
			execCmd := fmt.Sprintf(`AUTHOR="$(git show -s --format=%%ae)" && if [ "$AUTHOR" = "%s" ]; then git commit --amend --no-edit -S; fi`, *newEmail)
			// Prepare rebase command
			rebaseCmd := exec.Command("git", "rebase", "--root", "--exec", execCmd)
			rebaseCmd.Stdout = os.Stdout
			rebaseCmd.Stderr = os.Stderr
			rebaseCmd.Stdin = os.Stdin

			if err := rebaseCmd.Run(); err != nil {
				fmt.Println("Error running git rebase:", err)
				os.Exit(1)
			}
		}
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
