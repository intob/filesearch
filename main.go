package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/intob/jfmt"
	"github.com/ulikunitz/xz"
)

type result struct {
	linesMatched, linesSearched uint32
}

func main() {
	globFlag := flag.String("glob", "*", "file path glob")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("expected argument for search text")
		return
	}
	searchTxt := flag.Args()[0]

	fmt.Printf("searching %s for \"%s\"\n", *globFlag, searchTxt)

	results := make(chan *result, 1)
	tasks := make(chan string, 1)

	// gopher dispatches files as tasks
	go func(glob string) {
		files, err := filepath.Glob(glob)
		if err != nil {
			fmt.Println("failed to glob files:", err)
			return
		}
		for _, file := range files {
			tasks <- file
		}
		close(tasks)
	}(*globFlag)

	// 8 gophers complete the tasks
	wg := &sync.WaitGroup{}
	for i := 0; i < runtime.NumCPU()*2; i++ {
		wg.Add(1)
		go func(searchTxt string) {
			for task := range tasks {
				searchFile(task, searchTxt, results)
			}
			wg.Done()
		}(searchTxt)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var linesMatchedTotal, linesSearchedTotal uint32
	for result := range results {
		linesMatchedTotal += result.linesMatched
		linesSearchedTotal += result.linesSearched
		fmt.Printf("\rlines searched: %s, lines contain \"%s\": %s\033[0K",
			jfmt.FmtCount32(linesSearchedTotal),
			searchTxt,
			jfmt.FmtCount32(linesMatchedTotal))
	}

	fmt.Printf("\ndone")
	if linesMatchedTotal == 0 {
		fmt.Printf("\nnothing found for \"%s\"\n", searchTxt)
	}
}

// searchFile searches for a given text in the specified file
func searchFile(filePath, searchText string, results chan<- *result) error {
	var file *os.File
	var err error
	var tempPath string
	if path.Ext(filePath) == ".xz" {
		tempPath, err = decompressXZTemp(filePath)
		if err != nil {
			return fmt.Errorf("failed to decompress .xz: %w", err)
		}
		file, err = os.Open(tempPath)
	} else {
		file, err = os.Open(filePath)
	}
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()
	defer func() {
		if tempPath != "" {
			err = os.Remove(tempPath)
			if err != nil {
				fmt.Println("failed to remove temp file:", tempPath, err)
			}
		}
	}()

	var linesMatched, linesSearched uint32
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		linesSearched++
		if strings.Contains(scanner.Text(), searchText) {
			linesMatched++
		}
	}

	results <- &result{
		linesMatched:  linesMatched,
		linesSearched: linesSearched,
	}

	return nil
}

// decompressXZ decompresses an .xz file.
func decompressXZTemp(src string) (string, error) {
	// Open the source .xz file
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	// Create a reader to decompress the .xz file
	reader, err := xz.NewReader(srcFile)
	if err != nil {
		return "", err
	}

	// Create the output file
	destFile, err := os.CreateTemp("", "fsearch")
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	// Copy the decompressed data to the output file
	_, err = io.Copy(destFile, reader)
	return destFile.Name(), err
}
