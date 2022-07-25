package bsdata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// LatNS is a struct for latency in nanoseconds
type LatNS struct {
	Min        int64            `json:"min"`
	Max        int64            `json:"max"`
	Mean       float64          `json:"mean"`
	Stddev     float64          `json:"stddev"`
	Percentile map[string]int64 `json:"percentile,omitempty"`
}

// fioJSON is a struct for JSON input
type FioJSON struct {
	FioVersion   string `json:"fio version"`
	GlobalConfig struct {
		Ioengine       string `json:"ioengine"`
		Size           string `json:"size"`
		Direct         string `json:"direct"`
		Runtime        string `json:"runtime"`
		TimeBased      string `json:"time_based"`
		GroupReporting string `json:"group_reporting"`
		LogAvgMsec     string `json:"log_avg_msec"`
		Filename       string `json:"filename"`
	} `json:"global options"`
	Jobs []struct {
		TestName   string `json:"jobname"`
		GroupID    int    `json:"groupid"`
		Error      int    `json:"error"`
		Eta        int    `json:"eta"`
		Elapsed    int    `json:"elapsed"`
		TestOption struct {
			RW      	string `json:"rw"`
			BS      	string `json:"bs"`
			IODepth 	string `json:"iodepth"`
			NumJobs 	string `json:"numjobs"`
			BwLog   	string `json:"write_bw_log"`
			IOPSLog 	string `json:"write_iops_log"`
			LatLog 		string `json:"write_lat_log"`
		} `json:"job options"`
		Read struct {
			BW       int     `json:"bw"`
			Iops     float64 `json:"iops"`
			IoKbs    int     `json:"io_kbytes"`
			Runtime  int     `json:"runtime"`
			TotalIos int     `json:"total_ios"`
			ShortIos int     `json:"short_ios"`
			DropIos  int     `json:"drop_ios"`
			SlatNS   LatNS   `json:"slat_ns"`
			ClatNS   LatNS   `json:"clat_ns"`
			LatNS    LatNS   `json:"lat_ns"`
			IopsMin  int     `json:"iops_min"`
			IopsMax  int     `json:"iops_max"`
			IopsMean float64 `json:"iops_mean"`
			BWMin    int     `json:"bw_min"`
			BWMax    int     `json:"bw_max"`
			BWMean   float64 `json:"bw_mean"`
			BwDev       float64 `json:"bw_dev"`
			BwSamples   int     `json:"bw_samples"`
			IopsStddev  float64 `json:"iops_stddev"`
			IopsSamples int     `json:"iops_samples"`
		} `json:"read"`
		Write struct {
			BW       int     `json:"bw"`
			Iops     float64 `json:"iops"`
			IoKbs    int     `json:"io_kbytes"`
			Runtime  int     `json:"runtime"`
			TotalIos int     `json:"total_ios"`
			ShortIos int     `json:"short_ios"`
			DropIos  int     `json:"drop_ios"`
			SlatNS   LatNS   `json:"slat_ns"`
			ClatNS   LatNS   `json:"clat_ns"`
			LatNS    LatNS   `json:"lat_ns"`
			IopsMin  int     `json:"iops_min"`
			IopsMax  int     `json:"iops_max"`
			IopsMean float64 `json:"iops_mean"`
			BWMin    int     `json:"bw_min"`
			BWMax    int     `json:"bw_max"`
			BWMean   float64 `json:"bw_mean"`
			IopsStddev  float64 `json:"iops_stddev"`
			IopsSamples int     `json:"iops_samples"`
		} `json:"write"`
		JobRuntime        int     `json:"job_runtime"`
		UsrCPU            float64 `json:"usr_cpu"`
		SysCPU            float64 `json:"sys_cpu"`
		Majf      		  int     `json:"majf"`
		Minf     		  int     `json:"minf"`
		Ctx               int     `json:"ctx"`
		LatencyDepth      int     `json:"latency_depth"`
		LatencyTarget     int     `json:"latency_target"`
		LatencyPercentile float64 `json:"latency_percentile"`
		LatencyWindow     int     `json:"latency_window"`
	} `json:"jobs"`
	DiskUtil []struct {
		DiskName    string  `json:"name"`
		ReadIos     int     `json:"read_ios"`
		WriteIos    int     `json:"write_ios"`
		ReadMerges  int     `json:"read_merges"`
		WriteMerges int     `json:"write_merges"`
		ReadTicks   int     `json:"read_ticks"`
		WriteTicks  int     `json:"write_ticks"`
		InQueue     int     `json:"in_queue"`
		Util        float64 `json:"util"`
	} `json:"disk_util"`
}

// CleanJSON removes all another fields from JSON input
func CleanJSON(in []byte) ([]byte, error) {
	var begin = bytes.IndexAny(in, "{")
	var end = bytes.LastIndexAny(in, "}") + 1
	if begin >= end {
		return nil, fmt.Errorf("incorrect input format in cleanJSON")
	}
	return in[begin:end], nil
}

// ParseJSON parses FIO JSON input
func ParseJSON(in []byte) (FioJSON, error) {
	var data FioJSON
	if err := json.Unmarshal(in, &data); err != nil {
		return FioJSON{}, fmt.Errorf("invalid JSON input: %w", err)
	}
	return data, nil
}

type TestInfo struct {
	TestName 		string
	JSONResults 	FioJSON
	LogDirectory 	string
	JSONFileName 	string
	JSONFilePath 	string
	CSVFileName 	string
	CSVFilePath 	string
}

type AllTestInfo struct {
	Tests []TestInfo
	MainPathToResults string
	Description string
	PathWithSrcResults string
	ImgFormat string
}
// checkFolderWithJSONfiles - check if folder with JSON files exists
func checkFolderWithJSONfiles(pathToJSONfiles string) ([]string, error) {
	var jsonFilesPath []string
	files, err := ioutil.ReadDir(pathToJSONfiles)
	if err != nil {
		return jsonFilesPath,
			fmt.Errorf("could not read folder with JSON files: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		// Here later it will be possible to write check for the first bytes of
		// the file, this will get rid of the dependency of the .json extension.
		if strings.Contains(file.Name(), ".json") {
			jsonFilesPath = append(jsonFilesPath,
				filepath.Join(pathToJSONfiles, file.Name()))
		}
	}

	if len(jsonFilesPath) == 0 {
		return jsonFilesPath,
			fmt.Errorf("files with .json extension not found in directory:%s", pathToJSONfiles)
	}

	return jsonFilesPath, nil
}

func ReadAllJSONFiles(catalogWithJSONfiles string) (AllTestInfo, error){
	var allTestInfo AllTestInfo

 	jsonFiles, err := checkFolderWithJSONfiles(catalogWithJSONfiles)
	if err != nil {
		return allTestInfo, err
	}

	// Create CSV tables for each JSON file
	for _, srcFile := range jsonFiles {
		var testInfo TestInfo
		data, err := ioutil.ReadFile(srcFile)
		if err != nil {
			return allTestInfo, fmt.Errorf("could not read file [%s]: %w", srcFile, err)
		}

		text, err := CleanJSON(data)
		if err != nil {
			return allTestInfo, fmt.Errorf("could not clean JSON: %w", err)
		}

		testInfo.JSONResults, err = ParseJSON(text)
		if err != nil {
			return allTestInfo, fmt.Errorf("could not parse JSON: %w", err)
		}

		testInfo.JSONFileName = filepath.Base(srcFile)
		testInfo.JSONFilePath = srcFile
		testInfo.TestName = strings.TrimSuffix(testInfo.JSONFileName, ".json")
		allTestInfo.Tests = append(allTestInfo.Tests, testInfo)
	}

	return allTestInfo, nil
}
