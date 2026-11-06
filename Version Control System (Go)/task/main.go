package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"
)

const vcsDir = "vcs"

var (
	configPath = filepath.Join(vcsDir, "config.txt")
	indexPath  = filepath.Join(vcsDir, "index.txt")
	logPath    = filepath.Join(vcsDir, "log.txt")
	commitsDir = filepath.Join(vcsDir, "commits")
)

func main() {
	registry := Registry{
		Preface:  "These are SVCS commands:",
		Order:    make([]string, 0),
		Commands: make(map[string]Command),
	}
	registry.RegisterAll([]Command{
		{Path: "config", Usage: "Get and set a username.", Cmd: handleConfig},
		{Path: "add", Usage: "Add a file to the index.", Cmd: handleAdd},
		{Path: "log", Usage: "Show commit logs.", Cmd: handleLog},
		{Path: "commit", Usage: "Save changes.", Cmd: handleCommit},
		{Path: "checkout", Usage: "Restore a file.", Cmd: handleCheckout},
	})

	if len(os.Args) < 2 {
		registry.Help()
		return
	}
	target := os.Args[1]
	args := os.Args[2:]
	if target == "--help" || target == "" {
		registry.Help()
		return
	}
	if registry.Has(target) {
		registry.Run(target, args)
		return
	}
	fmt.Printf("'%s' is not a SVCS command.\n", target)
}

// Command is a named CLI command and its help text.
type Command struct {
	Path  string
	Usage string
	Cmd   func([]string)
}

// Registry stores commands and remembers their order for the help page.
type Registry struct {
	Commands map[string]Command
	Order    []string
	Preface  string
}

// Register adds a single command to the registry.
func (r *Registry) Register(cmd Command) {
	r.Commands[cmd.Path] = cmd
	r.Order = append(r.Order, cmd.Path)
}

// RegisterAll adds commands in the order they should appear in the help page.
func (r *Registry) RegisterAll(cmd []Command) {
	for _, c := range cmd {
		r.Register(c)
	}
}

// Run executes a command, or prints its usage if it has no handler.
func (r *Registry) Run(cmd string, args []string) {
	if command, ok := r.Commands[cmd]; ok {
		if command.Cmd != nil {
			command.Cmd(args)
		} else {
			fmt.Println(r.Commands[cmd].Usage)
		}
	}
}

// Has reports whether a command is registered.
func (r *Registry) Has(cmd string) bool {
	_, ok := r.Commands[cmd]
	return ok
}

// Help prints the help page.
func (r *Registry) Help() {
	fmt.Println(r.Preface)
	for _, cmd := range r.Order {
		Repr(r.Commands[cmd])
	}
}

// Repr prints one help page row for a command.
func Repr(cmd Command) {
	fmt.Printf("%-10s %s\n", cmd.Path, cmd.Usage)
}

// Config is the content of the config file.
type Config struct {
	Username string `json:"userName"`
}

// TrackedFiles is the content of the index file.
type TrackedFiles struct {
	Files []string `json:"files"`
}

// LogEntry describes a single commit.
type LogEntry struct {
	CommitHash string
	Author     string
	Message    string
	Timestamp  int64
}

// CommitLog is the content of the log file.
type CommitLog struct {
	Latest LogEntry
	Logs   []LogEntry
}

// loadJSON reads a JSON file into v. A missing or empty file leaves v unchanged.
func loadJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return nil
}

// saveJSON writes v to a JSON file, creating parent directories as needed.
func saveJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to serialize %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0666); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

// handleConfig prints the current username, or sets it if one is passed.
func handleConfig(args []string) {
	var cfg Config
	if err := loadJSON(configPath, &cfg); err != nil {
		log.Fatal(err)
	}

	if len(args) == 0 {
		if len(cfg.Username) > 0 {
			fmt.Printf("The username is %s.\n", cfg.Username)
		} else {
			fmt.Println("Please, tell me who you are.")
		}
		return
	}
	cfg.Username = args[0]
	if err := saveJSON(configPath, &cfg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("The username is %s.\n", cfg.Username)
}

// handleAdd prints the tracked files, or tracks a file if one is passed.
func handleAdd(args []string) {
	var tracked TrackedFiles
	if err := loadJSON(indexPath, &tracked); err != nil {
		log.Fatal(err)
	}
	if len(args) == 0 {
		if len(tracked.Files) > 0 {
			fmt.Println("Tracked files:")
			for _, file := range tracked.Files {
				fmt.Println(file)
			}
		} else {
			fmt.Println("Add a file to the index.")
		}
		return
	}
	file := filepath.Clean(args[0])
	if !filepath.IsLocal(file) {
		fmt.Printf("Can't track '%s': it's outside the current directory.\n", args[0])
		return
	}
	_, statErr := os.Stat(file)
	if os.IsNotExist(statErr) {
		fmt.Printf("Can't find '%s'.\n", args[0])
		return
	}
	if statErr != nil {
		log.Fatal(statErr)
	}
	if !slices.Contains(tracked.Files, file) {
		tracked.Files = append(tracked.Files, file)
	}
	if err := saveJSON(indexPath, &tracked); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("The file '%s' is tracked.\n", args[0])
}

// handleCommit saves a copy of the tracked files if they changed since the latest commit.
func handleCommit(args []string) {
	if len(args) == 0 {
		fmt.Println("Message was not passed.")
		return
	}
	var cfg Config
	if err := loadJSON(configPath, &cfg); err != nil {
		log.Fatal(err)
	}
	if len(cfg.Username) == 0 {
		fmt.Println("Please, tell me who you are.")
		return
	}
	var tracked TrackedFiles
	if err := loadJSON(indexPath, &tracked); err != nil {
		log.Fatal(err)
	}
	sha256Hash := sha256.New()
	for _, file := range tracked.Files {
		sha256Hash.Write([]byte(file))
		contents, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		sha256Hash.Write(contents)
	}
	newLogEntry := LogEntry{
		Timestamp:  time.Now().UnixMilli(),
		Author:     cfg.Username,
		Message:    args[0],
		CommitHash: fmt.Sprintf("%x", sha256Hash.Sum(nil)),
	}
	var commitLog CommitLog
	if err := loadJSON(logPath, &commitLog); err != nil {
		log.Fatal(err)
	}
	// if there's a matching commit, then nothing has changed
	if commitLog.Latest.CommitHash == newLogEntry.CommitHash {
		fmt.Println("Nothing to commit.")
		return
	}
	commitLog.Logs = append(commitLog.Logs, newLogEntry)
	commitLog.Latest = newLogEntry
	hashPath := filepath.Join(commitsDir, newLogEntry.CommitHash)
	if err := os.MkdirAll(hashPath, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	for _, file := range tracked.Files {
		if _, err := copyFile(file, filepath.Join(hashPath, file)); err != nil {
			log.Fatal(err)
		}
	}

	if err := saveJSON(logPath, &commitLog); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Changes are committed.")
}

// copyFile copies a regular file, creating parent directories as needed, and
// returns the number of bytes written.
func copyFile(src, dst string) (int64, error) {
	srcStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if !srcStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		return 0, err
	}
	dest, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer dest.Close()
	return io.Copy(dest, source)
}

// handleLog prints all commits, newest first.
func handleLog(_ []string) {
	var commitLog CommitLog
	if err := loadJSON(logPath, &commitLog); err != nil {
		log.Fatal(err)
	}
	if len(commitLog.Logs) == 0 {
		fmt.Println("No commits yet.")
		return
	}
	slices.SortFunc(commitLog.Logs, func(a, b LogEntry) int {
		return -cmp.Compare(a.Timestamp, b.Timestamp)
	})
	for _, logEntry := range commitLog.Logs {
		fmt.Printf("commit %s\n", logEntry.CommitHash)
		fmt.Printf("Author: %s\n", logEntry.Author)
		fmt.Println(logEntry.Message)
	}
}

// handleCheckout replaces the tracked files with their versions from a commit.
func handleCheckout(args []string) {
	if len(args) != 1 {
		fmt.Println("Commit id was not passed.")
		return
	}
	var commitLog CommitLog
	if err := loadJSON(logPath, &commitLog); err != nil {
		log.Fatal(err)
	}
	matchingCommitInd := slices.IndexFunc(commitLog.Logs, func(logEntry LogEntry) bool {
		return logEntry.CommitHash == args[0]
	})
	if matchingCommitInd == -1 {
		fmt.Println("Commit does not exist.")
		return
	}
	matchingCommit := commitLog.Logs[matchingCommitInd]
	var tracked TrackedFiles
	if err := loadJSON(indexPath, &tracked); err != nil {
		log.Fatal(err)
	}
	hashPath := filepath.Join(commitsDir, matchingCommit.CommitHash)
	// "uncheck out" the current files
	for _, file := range tracked.Files {
		err := os.Remove(file)
		if err != nil && !os.IsNotExist(err) {
			fmt.Println("Error removing file:", err)
			return
		}
	}
	// check in the new files
	err := filepath.WalkDir(hashPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		file, err := filepath.Rel(hashPath, path)
		if err != nil {
			return err
		}
		_, err = copyFile(path, file)
		return err
	})
	if err != nil {
		fmt.Println("Error copying file:", err)
		return
	}
	fmt.Printf("Switched to commit %s.\n", args[0])
}
