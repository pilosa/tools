package dx

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewSoloCompareCommand initializes a new compare command for dx.
func NewSoloCompareCommand(m *Main) *cobra.Command {
	compareCmd := &cobra.Command{
		Use:   "compare",
		Short: "compare two dx solo results",
		Long:  `Compare two result files generated by dx solo command.`,
		Args: func(cmd *cobra.Command, args []string) error {
			return validateCompareArgs(args)
		},
		Run: func(cmd *cobra.Command, args []string) {

			if err := ExecuteCompare(args[0], args[1]); err != nil {
				fmt.Printf("%+v", err)
				os.Exit(1)
			}

		},
	}

	return compareCmd
}

// validateCompareArgs validates that the args passed to compare command has length exactly 2
// and are valid filenames.
func validateCompareArgs(args []string) error {
	if len(args) != 2 {
		return errors.New("need exactly two files to compare")
	}
	fileExists, err := checkFileExists(args[0])
	if err != nil {
		return errors.Wrapf(err, "error verifying file %s exists", args[0])
	}
	if !fileExists {
		return errors.Errorf("%s does not exist", args[0])
	}
	fileExists, err = checkFileExists(args[1])
	if err != nil {
		return errors.Wrapf(err, "error verifying file %s exists", args[1])
	}
	if !fileExists {
		return errors.Errorf("%s does not exist", args[1])
	}
	return nil
}

// ExecuteCompare compares the results of two files generated by dx solo.
func ExecuteCompare(file1, file2 string) error {
	bench1, err := readResultFile(file1, "")
	if err != nil {
		return errors.Wrapf(err, "error reading file %v", file1)
	}
	bench2, err := readResultFile(file2, "")
	if err != nil {
		return errors.Wrapf(err, "error reading file %v", file2)
	}
	if bench1.Type != bench2.Type {
		return errors.Errorf("cannot compare results of type %v and %v", bench1.Type, bench2.Type)
	}

	if bench1.Type == cmdIngest {
		if err = compareIngest(bench1, bench2); err != nil {
			return errors.Wrap(err, "error comparing ingest")
		}
		return nil
	}

	if err = compareQueries(bench1, bench2); err != nil {
		return errors.Wrap(err, "error comparing queries")
	}

	return nil
}

func compareIngest(bench1, bench2 *SoloBenchmark) error {
	timeDelta := float64(bench1.Time.Duration-bench2.Time.Duration) / float64(bench1.Time.Duration)
	b := &Benchmark{
		CTime:     bench1.Time.Duration,
		PTime:     bench2.Time.Duration,
		TimeDelta: timeDelta,
	}

	if err := printIngestResults(b); err != nil {
		return errors.Wrap(err, "error printing ingest results")
	}
	return nil
}

func compareQueries(bench1, bench2 *SoloBenchmark) error {
	var totalBenches int
	if *bench1.NumBenchmarks <= *bench2.NumBenchmarks {
		totalBenches = *bench1.NumBenchmarks
	} else {
		totalBenches = *bench2.NumBenchmarks
	}

	benchmarks := make([]*Benchmark, 0)
	for i := 0; i < totalBenches; i++ {
		bench, err := compareQueryBenchmarks(bench1.Benchmarks[i], bench2.Benchmarks[i])
		if err != nil {
			return errors.Wrapf(err, "error comparing benchmarks in iteration %v", i)
		}
		benchmarks = append(benchmarks, bench)
	}

	if err := printQueryResults(benchmarks...); err != nil {
		return errors.Wrap(err, "error printing query results")
	}
	return nil
}

func compareQueryBenchmarks(querybench1, querybench2 *QueryBenchmark) (*Benchmark, error) {
	var time1, time2 time.Duration
	var numCorrect int64

	// validQueries is the number of queries that successfully ran.
	// This is not equivalent to the number of queries with correct results.
	validQueries := querybench1.NumQueries

	for i := 0; i < querybench1.NumQueries; i++ {
		query1, found := querybench1.Queries[i]
		// ignore if query doesn't exist in first file or doesn't have result
		if !found || (query1.Result == nil && query1.ResultCount == nil) {
			validQueries--
			continue
		}
		query2, found := querybench2.Queries[i]
		// accuracy decreases if query or query result doesn't exist in second file
		if !found || (query2.Result == nil && query2.ResultCount == nil) {
			continue
		}

		if queryResultsAreEqual(query1, query2) {
			numCorrect++
		}
		time1 += query1.Time.Duration
		time2 += query2.Time.Duration
	}

	accuracy := float64(numCorrect) / float64(validQueries)
	timeDelta := float64(time2-time1) / float64(time1)

	return &Benchmark{
		Size:      int64(validQueries),
		PTime:     time1,
		CTime:     time2,
		Accuracy:  accuracy,
		TimeDelta: timeDelta,
	}, nil
}

func queryResultsAreEqual(query1, query2 *Query) bool {
	// one of query1.Result and query1.ResultCount is not nil
	if query1.Result == nil {
		if query2.Result == nil {
			return *query1.ResultCount == *query2.ResultCount
		}
		return *query1.ResultCount == int64(len(query2.Result.Columns))
	}
	// else, query1.Result is not nil
	if query2.Result == nil {
		return int64(len(query1.Result.Columns)) == *query2.ResultCount
	}
	return reflect.DeepEqual(query1.Result, query2.Result)
}