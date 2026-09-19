package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// runGitCommand executes a git subcommand in the current working directory.
func runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v failed: %w (stderr: %s)", args, err, stderr.String())
	}
	return nil
}

// gitAdd runs `git add <filePath>`.
func gitAdd(filePath string) error {
	return runGitCommand("add", filePath)
}

// gitCommit runs `git commit -m <message>`.
func gitCommit(message string) error {
	return runGitCommand("commit", "-m", message)
}

// gitPush runs `git push <remote> <branch>`.
func gitPush(remote, branch string) error {
	return runGitCommand("push", remote, branch)
}
