package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/intob/jfmt"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("expected argument for search text")
		return
	}
	var total uint32

	found := make(chan uint32, 1)
	tasks := make(chan string, 1)

	// gopher dispatches files as tasks
	go func() {
		files, err := filepath.Glob("*")
		if err != nil {
			fmt.Println("Failed to glob files:", err)
			return
		}
		for _, file := range files {
			tasks <- file
		}
		close(tasks)
	}()

	// 8 gophers complete the tasks
	wg := &sync.WaitGroup{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			for task := range tasks {
				searchFile(task, os.Args[1], found)
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(found)
	}()

	for count := range found {
		total += count
		fmt.Printf("\rtotal: %s\033[0K", jfmt.FmtCount32(total))
	}

	fmt.Printf("\ndone")
	if total == 0 {
		fmt.Printf("\nnothing found for \"%s\"\n", os.Args[1])
	}
}

// searchFile searches for a given text in the specified file
func searchFile(filePath, searchText string, found chan<- uint32) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), searchText) {
			found <- 1
			break // count at most once per file
		}
	}
}
