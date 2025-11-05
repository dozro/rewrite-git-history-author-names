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

		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Println("Error reading fast-export stream:", err)
				break
			}
			if strings.HasPrefix(line, "author ") || strings.HasPrefix(line, "committer ") {
				line = rewriteAuthor(line, *oldEmail, *newName, *newEmail)
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
