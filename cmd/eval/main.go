// Command eval compares testdata/enrich gold and predictions JSONL.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/andrewmysliuk/jobhound_core/internal/enrich/impl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	goldPath := "testdata/enrich/gold.jsonl"
	predPath := "testdata/enrich/predictions.jsonl"
	if len(os.Args) > 1 {
		goldPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		predPath = os.Args[2]
	}

	gold, err := readEvalJSONL(goldPath)
	if err != nil {
		return fmt.Errorf("gold: %w", err)
	}
	predictions, err := readEvalJSONL(predPath)
	if err != nil {
		return fmt.Errorf("predictions: %w", err)
	}

	fmt.Print(impl.FormatEvalMetrics(impl.CompareEval(gold, predictions)))
	return nil
}

func readEvalJSONL(path string) ([]impl.EvalExample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []impl.EvalExample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ex impl.EvalExample
		if err := json.Unmarshal(line, &ex); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
		}
		if ex.ID == "" {
			return nil, fmt.Errorf("%s:%d: missing id", path, lineNo)
		}
		out = append(out, ex)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
