package csvtable

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	bs "github.com/vk-en/fioplot-bs/pkg/bsdata"
)

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
func formatCSV(in bs.FioJSON, to io.Writer) error {
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

// ConvertJSONtoCSV converts JSON input to CSV file
func ConvertJSONtoCSV(fioJSON bs.FioJSON, outputPath string) error {
	fd, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("could not create CSV file [%s]: %w", outputPath, err)
	}
	defer fd.Close()

	if err := formatCSV(fioJSON, fd); err != nil {
		return fmt.Errorf("could not format CSV: %w", err)
	}
	return nil
}
