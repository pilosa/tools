package dx

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pilosa/go-pilosa"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// NewSoloQueryCommand initializes a new query command for dx.
func NewSoloQueryCommand() *cobra.Command {
	ingestCmd := &cobra.Command{
		Use:   "query",
		Short: "perform random queries",
		Long:  `Perform randomly generated queries on a single instances of Pilosa.`,
		Run: func(cmd *cobra.Command, args []string) {

			if err := ExecuteSoloQueries(cmd.Flags()); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		},
	}
	ingestCmd.PersistentFlags().IntSliceVarP(&m.NumQueries, "queries", "q", []int{100000, 1000000, 10000000, 100000000}, "Number of queries to run")
	ingestCmd.PersistentFlags().Int64VarP(&m.NumRows, "rows", "r", 2, "Number of rows to perform intersect query on")
	return ingestCmd
}

// SoloBenchmark represents the final result of a dx solo query benchmark.
type SoloBenchmark struct {
	Command       string            `json:"type"`
	Instance      string            `json:"instance"`
	NumBenchmarks int               `json:"numBenchmarks"`
	Benchmarks    []*QueryBenchmark `json:"benchmarks"`
	ThreadCount   int               `json:"threadcount"`
}

func newSoloBenchmark(benchType, instance string, threadCount int) *SoloBenchmark {
	return &SoloBenchmark{
		Command:     benchType,
		Instance:    instance,
		Benchmarks:  make([]*QueryBenchmark, 0),
		ThreadCount: threadCount,
	}
}

// QueryBenchmark represents a single query benchmark.
type QueryBenchmark struct {
	NumQueries int          `json:"numQueries"`
	Time       TimeDuration `json:"time"`
	Queries    []*Query     `json:"queries"`
}

func newQueryBenchmark(numQueries int) *QueryBenchmark {
	return &QueryBenchmark{
		NumQueries: numQueries,
		Queries:    make([]*Query, 0),
	}
}

// Query contains the information related to an intersect query.
type Query struct {
	IndexName string             `json:"index"`
	FieldName string             `json:"field"`
	Rows      []int64            `json:"rows"`
	Result    pilosa.QueryResult `json:"result"`
	Time      time.Duration
}

// ExecuteSoloQueries executes queries on a single Pilosa instance. This is where the configuration
// is determined before being passed on to the appropriate functions to handle.
func ExecuteSoloQueries(flags *flag.FlagSet) error {
	instanceType, err := determineInstance(flags)
	if err != nil {
		return err
	}
	_, isFirstQuery, hashFilename, err := checkBenchIsFirst(m.SpecsFile, m.DataDir)
	if err != nil {
		return fmt.Errorf("error checking if prior bench exists: %v", err)
	}
	// append `query` to the file name
	hashFilename = hashFilename + cmdQuery

	// initialize holder from specs
	iconfs, err := getSpecs(m.SpecsFile)
	if err != nil {
		return fmt.Errorf("could not parse specs: %v", err)
	}
	var holder *holder
	switch instanceType {
	case instanceCandidate:
		holder, err = initializeHolder(instanceCandidate, m.CHosts, m.CPort, iconfs)
	case instancePrimary:
		holder, err = initializeHolder(instancePrimary, m.PHosts, m.PPort, iconfs)
	default:
		err = fmt.Errorf("invalid instance type: %v", instanceType)
	}
	if err != nil {
		return fmt.Errorf("could not create holder for instance: %v", err)
	}

	// previous result file for specs does not exist. Running dx solo for the first time.
	if isFirstQuery {
		if err = executeFirstSoloQueries(holder, hashFilename, m.DataDir, m.NumQueries, m.ThreadCount, m.NumRows); err != nil {
			return fmt.Errorf("could not execute first solo queries: %v", err)
		}
	}

	// previous result file for specs exist. Running dx solo for the second time.
	if err = executeSecondSoloQueries(holder, hashFilename, m.DataDir); err != nil {
		return fmt.Errorf("could not execute second solo queries: %v", err)
	}
	return nil
}

// executeFirstSoloQueries is the entry point for executing queries on a single instance for the first time.
// Queries are randomly generated, and the queries and results are recorded in a JSON file whose name is the 
// sha256 hash of the specs file + `query`.
func executeFirstSoloQueries(holder *holder, filename, dataDir string, numBenchmarks []int, threadCount int, numRows int64) error {
	solobench := newSoloBenchmark(cmdQuery, holder.instance, threadCount)
	for _, numQueries := range numBenchmarks {
		querybench, err := runFirstSoloQueries(holder, numQueries, threadCount, numRows)
		if err != nil {
			return fmt.Errorf("could not run first solo queries: %v", err)
		}
		solobench.Benchmarks = append(solobench.Benchmarks, querybench)
		solobench.NumBenchmarks++
	}
	if err := writeQueryResultFile(solobench, holder.instance, filename, dataDir); err != nil {
		return fmt.Errorf("could not write solo queries results to file: %v", err)
	}
	return nil
}

// runFirstSoloQueries launches threadcount number of goroutines that do the actual generation and running
// of queries, and returns these results.
func runFirstSoloQueries(holder *holder, numQueries, threadCount int, numRows int64) (*QueryBenchmark, error) {
	qBench := newQueryBenchmark(numQueries)

	queryChan := make(chan *Query, numQueries)
	q := &queryOp{
		holder:    holder,
		queryChan: queryChan,
		numRows:   numRows,
	}
	go launchThreads(numQueries, threadCount, q, runFirstSoloQueriesOnInstance)

	for query := range q.queryChan {
		qBench.Time.Duration += query.Time
		qBench.Queries = append(qBench.Queries, query)
	}

	return qBench, nil
}

// runFirstSoloQueriesOnInstance generates and runs a single query. 
func runFirstSoloQueriesOnInstance(q *queryOp) {
	resultChan := make(chan *Result, 1)

	indexName, fieldName, err := q.holder.randomIF()
	if err != nil {
		m.Logger.Printf("could not generate random index and field from holder: %v", err)
		return
	}
	cif, err := q.holder.newCIF(indexName, fieldName)
	if err != nil {
		m.Logger.Printf("could not create index %v and field %v from holder: %v", indexName, fieldName, err)
		return
	}
	rows := generateRandomRows(cif.Min, cif.Max, q.numRows)

	go runQueryOnInstance(cif, rows, resultChan)
	result := <-resultChan

	if result.err != nil {
		m.Logger.Printf("error running query on instance: %v", result.err)
		return
	}
	query := &Query{
		IndexName: indexName,
		FieldName: fieldName,
		Rows:      rows,
		Result:    result.result,
		Time:      result.time,
	}

	q.queryChan <- query
}

// executeSecondSoloQueries is the entry point for executing queries on a single instance for the second time.
// A previous result file must already be present, and this is parsed to determine which queries the function
// will run. The second results are compared to the recorded ones and passed on to printing.
func executeSecondSoloQueries(holder *holder, filename, dataDir string) error {
	solobench, err := readQueryResultFile(holder.instance, filename, dataDir)
	if err != nil {
		return fmt.Errorf("could not read query result file: %v", err)
	}
	if solobench.Command != cmdQuery {
		return fmt.Errorf("running dx solo query, but previous result shows command: %v", solobench.Command)
	}
	otherInstanceType, _ := otherInstance(holder.instance)
	if solobench.Instance != otherInstanceType {
		return fmt.Errorf("running dx solo query on instance %v, but previous result file was already on %v", holder.instance, solobench.Instance)
	}
	benchmarks := make([]*Benchmark, 0, solobench.NumBenchmarks)

	for _, querybench := range solobench.Benchmarks {
		bench, err := runSecondSoloBenchmark(holder, querybench, solobench.ThreadCount)
		if err != nil {
			return err
		}
		benchmarks = append(benchmarks, bench)
	}

	if err = printQueryResults(benchmarks...); err != nil {
		return fmt.Errorf("error printing benchmarks: %v", err)
	}

	// everything was successful, so it is now safe to delete the previous result file
	path := filepath.Join(dataDir, filename)
	if err = os.Remove(path); err != nil {
		return fmt.Errorf("everything ran successfully, but previous results file could not be deleted: %v", err)
	}

	return nil
}

// runSecondSoloBenchmark launches threadcount number of goroutines that read and run queries from 
// the query channel, and also where we analyze the results.
func runSecondSoloBenchmark(holder *holder, querybench *QueryBenchmark, threadCount int) (*Benchmark, error) {
	numQueries := querybench.NumQueries
	queryChan := make(chan *Query, numQueries)

	for _, query := range querybench.Queries {
		queryChan <- query
	}
	q := &queryOp{
		holder:     holder,
		queryChan:  queryChan,
		resultChan: make(chan *QResult, numQueries),
	}
	go launchThreads(numQueries, threadCount, q, runSecondQueriesOnInstance)

	benchmark, err := analyzeQueryResults(q.resultChan, numQueries)
	if err != nil {
		return nil, fmt.Errorf("error analyzing results: %v", err)
	}
	return benchmark, nil
}

// runSecondQueriesOnInstance runs a query received from the query channel. The result of this query
// is compared to the previous recorded result.
func runSecondQueriesOnInstance(q *queryOp) {
	qResult := newQResult()
	resultChan := make(chan *Result, 1)

	query := <-q.queryChan
	cif, err := q.holder.newCIF(query.IndexName, query.FieldName)
	if err != nil {
		m.Logger.Printf("could not create index %v and field %v from holder", query.IndexName, query.FieldName)
		return
	}

	go runQueryOnInstance(cif, query.Rows, resultChan)
	result := <-resultChan
	if result.err != nil {
		m.Logger.Printf("error running query on instance: %v", err)
		return
	}
	
	switch q.holder.instance {
	case instanceCandidate:
		qResult.candidateTime = result.time
		qResult.primaryTime = query.Time
	case instancePrimary:
		qResult.candidateTime = query.Time
		qResult.primaryTime = result.time
	default:
		m.Logger.Printf("invalid instance type: %v", q.holder.instance)
	}

	if reflect.DeepEqual(query.Result, result.result) {
		qResult.correct = true
	}
	q.resultChan <- qResult
}

// writeQueryResulFile writes the results of a SoloBenchmark to a JSON file whose name is the hash of the specs.
func writeQueryResultFile(bench *SoloBenchmark, instanceType, filename, dataDir string) error {
	jsonBytes, err := json.Marshal(bench)
	if err != nil {
		return fmt.Errorf("could not marshal results to JSON: %v", err)
	}
	path := filepath.Join(dataDir, filename)
	if err = ioutil.WriteFile(path, jsonBytes, 0666); err != nil {
		return fmt.Errorf("could not write JSON to file: %v", err)
	}
	return nil
}

// readQueryResultFile reads the results of a SoloBenchmark from a file.
func readQueryResultFile(instanceType string, filename, dataDir string) (*SoloBenchmark, error) {
	path := filepath.Join(dataDir, filename)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not open previous result file: %v", err)
	}
	defer file.Close()

	jsonBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read previous result file: %v", err)
	}
	var soloBench SoloBenchmark
	json.Unmarshal(jsonBytes, &soloBench)

	return &soloBench, nil
}

// TimeDuration wraps time.Duration for encoding to JSON
type TimeDuration struct {
	Duration time.Duration
}

// UnmarshalJSON to satisfy
func (d *TimeDuration) UnmarshalJSON(b []byte) (err error) {
	d.Duration, err = time.ParseDuration(strings.Trim(string(b), `"`))
	return
}

// MarshalJSON to satisfy
func (d *TimeDuration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%v"`, d.Duration)), nil
}