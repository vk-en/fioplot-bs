package getdata

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GroupResults - struct for group results
// Curent format CSV: "Group ID", "Pattern", "Block Size", "IO Depth", "Jobs", "Mb/s"
type GroupResults struct {
	JobName		string
	GroupID     string
	Pattern     string
	Bs          string
	Depth       string
	JobsCount   string
	Performance string
	BwMin       string
	BwMax       string
	IopsMin     int
	IopsMax     int
	LatMin      float64
	LatMax      float64
	LatStd      float64
	CLatPercent float64
}

// TestResult - struct for test results
type TestResult struct {
	GroupRes GroupResults
	Pattern  string
}

// GroupRes - type for group results
type GroupTestRes []*TestResult

// ListAllResults - list of all results
type ListAllResults struct {
	IOTestResults GroupTestRes
	FileName      string
	TestName      string
}

// AllPatternResults - struct for all pattern results
type AllPatternResults struct {
	PatternName  string
	Values       []float64
	Legends      []string
	YDiscription string
	FileName     string
}

// PatternsTable - type for table of patterns from AllPatternResults
type PatternsTable []*AllPatternResults

// AllResults - just all reuslts
type AllResults []*ListAllResults

const (
	performance uint16 = 1 << iota
	minIOPS
	maxIOPS
	minBW
	maxBW
	minLat
	maxLat
	stdLat
	p99Lat
)

// Round - round performance value
func Round(x float64) float64 {
	t := math.Trunc(x)
	if math.Abs(x-t) >= 0.5 {
		return t + math.Copysign(1, x)
	}
	return t
}

// ParsingCSVfile - Parsing CSV file
func (t *AllResults) ParsingCSVfile(fullPathToCsv string) error {
	csvfileName := filepath.Base(fullPathToCsv)
	var groupFile = make(GroupTestRes, 0)
	csvFile, err := os.Open(fullPathToCsv)
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	defer csvFile.Close()

	reader, err := csv.NewReader(csvFile).ReadAll()
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	for iter, line := range reader {
		if iter == 0 {
			continue
		}
		pIopsMin, _ := strconv.Atoi(line[9])
		pIopsMax, _ := strconv.Atoi(line[10])
		pLatMin, _ := strconv.ParseFloat(line[11], 64)
		pLatMax, _ := strconv.ParseFloat(line[12], 64)
		pLatStd, _ := strconv.ParseFloat(line[13], 64)
		pLatP, _ := strconv.ParseFloat(line[14], 64)
		resultOneGroup := GroupResults{
			JobName:	 line[0],
			GroupID:     line[1],
			Pattern:     line[2],
			Bs:          line[3],
			Depth:       line[4],
			JobsCount:   line[5],
			Performance: line[6],
			BwMin:       line[7],
			BwMax:       line[8],
			IopsMin:     pIopsMin,
			IopsMax:     pIopsMax,
			LatMin:      pLatMin,
			LatMax:      pLatMax,
			LatStd:      pLatStd,
			CLatPercent: pLatP,
		}
		group := TestResult{
			GroupRes: resultOneGroup,
			Pattern: fmt.Sprintf("%s-%s d=%s j=%s", //change on only JobName later when will add more info in legend
				resultOneGroup.Pattern,
				resultOneGroup.Bs,
				resultOneGroup.Depth,
				resultOneGroup.JobsCount),
		}
		groupFile = append(groupFile, &group)
	}
	testN := strings.Split(csvfileName, ".")
	finishRes := ListAllResults{
		IOTestResults: groupFile,
		FileName:      fullPathToCsv,
		TestName:      testN[0],
	}
	*t = append(*t, &finishRes)
	return nil
}

//GetIdenticalPatterns - search for identical results patterns
func GetIdenticalPatterns(groups AllResults) ([]string, error) {
	var allPattern []string
	uniq := make(map[string]int)
	for _, group := range groups {
		/* This output of a list of files is necessary to
		* understand which files we are transferring for
		* comparison. Sometimes it happens that there
		*are hidden temporary files in the folder! */
		for _, result := range group.IOTestResults {
			uniq[result.Pattern]++
		}
	}
	for key, val := range uniq {
		if val == len(groups) {
			allPattern = append(allPattern, key)
		}
	}

	if len(allPattern) == 0 {
		return nil, fmt.Errorf("error: the number of files is more than necessary")
	}
	return allPattern, nil
}

//GetPatternTable - gets patterns based structures
func (t *PatternsTable) GetPatternTable(identicalPattern []string, results AllResults, valueType uint16) {
	for _, ipattern := range identicalPattern {
		fTable := AllPatternResults{
			PatternName: ipattern,
		}
		*t = append(*t, &fTable)
	}

	for _, stroka := range *t {
		for _, test := range results {
			for _, pattern := range test.IOTestResults {
				if pattern.Pattern == stroka.PatternName {
					var value float64
					switch val := valueType; val {
					case performance:
						tmpBw, _ := strconv.ParseFloat(pattern.GroupRes.Performance, 64)
						value = Round(tmpBw)
						stroka.YDiscription = "Mb/s"
						stroka.FileName = "Performance"
					case minIOPS:
						value = float64(pattern.GroupRes.IopsMin)
						stroka.YDiscription = "IOPS min"
						stroka.FileName = "IOPS_min_value"
					case maxIOPS:
						value = float64(pattern.GroupRes.IopsMax)
						stroka.YDiscription = "IOPS max"
						stroka.FileName = "IOPS_max_value"
					case minBW:
						tmpBwMin, _ := strconv.ParseFloat(pattern.GroupRes.BwMin, 64)
						value = Round(tmpBwMin)
						stroka.YDiscription = "BW Min (MB/s)"
						stroka.FileName = "BW_min_value"
					case maxBW:
						tmpBwMax, _ := strconv.ParseFloat(pattern.GroupRes.BwMax, 64)
						value = Round(tmpBwMax)
						stroka.YDiscription = "BW Max (MB/s)"
						stroka.FileName = "BW_max_value"
					case minLat:
						value = float64(pattern.GroupRes.LatMin)
						stroka.YDiscription = "Latency min (ms)"
						stroka.FileName = "Latency_min_value"
					case maxLat:
						value = float64(pattern.GroupRes.LatMax)
						stroka.YDiscription = "Latency max (ms)"
						stroka.FileName = "Latency_max_value"
					case stdLat:
						value = float64(pattern.GroupRes.LatStd)
						stroka.YDiscription = "Latency stddev (ms)"
						stroka.FileName = "Latency_stdev"
					case p99Lat:
						value = float64(pattern.GroupRes.CLatPercent)
						stroka.YDiscription = "cLatency p99 (ms)"
						stroka.FileName = "Latency_p99"
					default:
						fmt.Println("Error with options")
					}
					stroka.Values = append(stroka.Values, value)
					stroka.Legends = append(stroka.Legends, test.TestName)
				}
			}
		}
	}
}
