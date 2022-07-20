package csvtable

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"

	"os"
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
type fioJSON struct {
	FioVersion   string `json:"fio version"`
	GlobalConfig struct {
		IoEngine string `json:"ioengine"`
		Direct   string `json:"direct"`
	} `json:"global options"`
	Jobs []struct {
		TestName   string `json:"jobname"`
		GroupID    int    `json:"groupid"`
		TestOption struct {
			RW      string `json:"rw"`
			BS      string `json:"bs"`
			IODepth string `json:"iodepth"`
			NumJobs string `json:"numjobs"`
			BwLog   string `json:"write_bw_log"`
			IOPSLog string `json:"write_iops_log"`
		} `json:"job options"`
		Read struct {
			BW       int     `json:"bw"`
			Iops     float64 `json:"iops"`
			IoKbs    int     `json:"io_kbytes"`
			Runtime  int     `json:"runtime"`
			SlatNS   LatNS   `json:"slat_ns"`
			ClatNS   LatNS   `json:"clat_ns"`
			LatNS    LatNS   `json:"lat_ns"`
			TotalIos int     `json:"total_ios"`
			IopsMin  int     `json:"iops_min"`
			IopsMax  int     `json:"iops_max"`
			IopsMean float64 `json:"iops_mean"`
			BWMin    int     `json:"bw_min"`
			BWMax    int     `json:"bw_max"`
			BWMean   float64 `json:"bw_mean"`
		} `json:"read"`
		Write struct {
			BW       int     `json:"bw"`
			Iops     float64 `json:"iops"`
			IoKbs    int     `json:"io_kbytes"`
			Runtime  int     `json:"runtime"`
			SlatNS   LatNS   `json:"slat_ns"`
			ClatNS   LatNS   `json:"clat_ns"`
			LatNS    LatNS   `json:"lat_ns"`
			TotalIos int     `json:"total_ios"`
			IopsMin  int     `json:"iops_min"`
			IopsMax  int     `json:"iops_max"`
			IopsMean float64 `json:"iops_mean"`
			BWMin    int     `json:"bw_min"`
			BWMax    int     `json:"bw_max"`
			BWMean   float64 `json:"bw_mean"`
		} `json:"write"`
		JobRuntime        int     `json:"job_runtime"`
		UsrCPU            float64 `json:"usr_cpu"`
		SysCPU            float64 `json:"sys_cpu"`
		Ctx               int     `json:"ctx"`
		LatencyDepth      int     `json:"latency_depth"`
		LatencyTarget     int     `json:"latency_target"`
		LatencyPercentile float64 `json:"latency_percentile"`
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

// parseJSON parses JSON input
func parseJSON(in []byte) (fioJSON, error) {
	var data fioJSON
	if err := json.Unmarshal(in, &data); err != nil {
		return fioJSON{}, fmt.Errorf("invalid JSON input: %w", err)
	}
	return data, nil
}

// toFixed is a helper function for rounding float64
func toFixed(x float64, n int) float64 {
	var l = math.Pow(10, float64(n))
	var mbs = math.Round(x*l) / l
	return mbs * 1.049 // formula from google
}

// mbps is a helper function for converting bytes to MB/s
func mbps(x int) float64 {
	return toFixed(float64(x)/1024, 2)
}

// formatCSV formats CSV input
func formatCSV(in fioJSON, to io.Writer) error {
	var header = []string{
		"Job Name", "Group ID", "Pattern", "Block Size", "IO Depth", "Jobs",
		"MB/s", "BWMim (MB/s)", "BWMax (MB/s)", "IOPS min", "IOPS max",
		"Latency Min (ms)", "Latency Max (ms)", "Latency stddev (ms)",
		"cLatency p99 (ms)",
	}

	var w = csv.NewWriter(to)
	if err := w.Write(header); err != nil {
		return err
	}

	for _, v := range in.Jobs {
		var bw = v.Write.BW
		var bwMin = v.Write.BWMin
		var bwMax = v.Write.BWMax
		var iopsMin = v.Write.IopsMin
		var iopsMax = v.Write.IopsMax
		var latNsMin = float64(v.Write.LatNS.Min) / 1000000
		var latNsMax = float64(v.Write.LatNS.Max) / 1000000
		var latNsStdDev = v.Write.LatNS.Stddev / 1000000
		var cLatNsPercent = float64(v.Write.ClatNS.Percentile["99.000000"]) / 1000000

		if v.TestOption.RW == "read" || v.TestOption.RW == "randread" {
			bw = v.Read.BW
			bwMin = v.Read.BWMin
			bwMax = v.Read.BWMax
			iopsMin = v.Read.IopsMin
			iopsMax = v.Read.IopsMax
			latNsMin = float64(v.Read.LatNS.Min) / 1000000
			latNsMax = float64(v.Read.LatNS.Max) / 1000000
			latNsStdDev = v.Read.LatNS.Stddev / 1000000
			cLatNsPercent = float64(v.Read.ClatNS.Percentile["99.000000"]) / 1000000
		}
		var row = []string{
			v.TestName,
			fmt.Sprintf("%v", v.GroupID),
			v.TestOption.RW,
			v.TestOption.BS,
			v.TestOption.IODepth,
			v.TestOption.NumJobs,
			fmt.Sprintf("%.2f", mbps(bw)),
			fmt.Sprintf("%.2f", mbps(bwMin)),
			fmt.Sprintf("%.2f", mbps(bwMax)),
			fmt.Sprintf("%d", iopsMin),
			fmt.Sprintf("%d", iopsMax),
			fmt.Sprintf("%.2f", latNsMin),
			fmt.Sprintf("%.2f", latNsMax),
			fmt.Sprintf("%.2f", latNsStdDev),
			fmt.Sprintf("%.2f", cLatNsPercent),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	w.Flush()
	return nil
}

// cleanJSON removes all another fields from JSON input
func cleanJSON(in []byte) ([]byte, error) {
	var begin = bytes.IndexAny(in, "{")
	var end = bytes.LastIndexAny(in, "}") + 1
	if begin >= end {
		return nil, errors.New("incorrect input format")
	}
	return in[begin:end], nil
}

// ConvertJSONtoCSV converts JSON input to CSV file
func ConvertJSONtoCSV(inputPath, outputPath string) error {
	data, err := ioutil.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("could not read file [%s]: %w", inputPath, err)
	}

	text, err := cleanJSON(data)
	if err != nil {
		return fmt.Errorf("could not clean JSON: %w", err)
	}
	obj, err := parseJSON(text)
	if err != nil {
		return fmt.Errorf("could not parse JSON: %w", err)
	}

	fd, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create CSV file [%s]: %w", outputPath, err)
	}
	defer fd.Close()

	if err := formatCSV(obj, fd); err != nil {
		return fmt.Errorf("could not format CSV: %w", err)
	}

	return nil
}
