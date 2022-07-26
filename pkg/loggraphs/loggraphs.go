package loggraphs

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wcharczuk/go-chart/v2"
	"github.com/wcharczuk/go-chart/v2/drawing"
	bs "github.com/vk-en/fioplot-bs/pkg/bsdata"
)

const (
	LogChartWidth = 1920
	LogChartHeight = 1280
	LogChartPaddingTop = 110
	LogChartPaddingLeft = 50
	LogChartPaddingRight = 50
	LogChartPaddingBottom = 250
	LogChartIndentLegend = LogChartHeight - LogChartPaddingBottom + 30
)

// Logs have line from fio bw/IOPS/latency log file.  (Example: 15, 204800, 0, 0);
// For bw log value == KiB/sec;
// For IOPS log value == count Iops;
// For Latency log value == latency in nsecs.
type LogLine struct {
	time   int // msec
	value  int
	opType int // read - 0 ; write - 1 ; trim - 2
}

// GroupLogFiles - group log files by test name
type GroupLogFiles struct {
	filesPath    []string
	patternName  string
	minCountLine int
}

// LogGF - log files info
type LogGF []*GroupLogFiles

// LogFile - log data
type LogFile []*LogLine

// toFixed - round float64 to fixed decimal places
func toFixed(x float64, n int) float64 {
	var l = math.Pow(10, float64(n))
	var mbs = math.Round(x*l) / l
	return mbs * 1.049 // formula from google
}

// mbps - convert bytes to MB/s
func mbps(x int) float64 {
	return toFixed(float64(x)/1024, 2)
}

// readDirWithResults - read directory with logs and return list of files
func readDirWithResults(dirPath string) ([]fs.FileInfo, error) {
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	return files, nil
}

// saveFile	- save new log file (glued)
func (t *LogFile) saveFile(filePath string, countLine int) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error create file [%s] %w", filePath, err)
	}
	defer file.Close()

	for index, line := range *t {
		if index == countLine {
			break
		}
		_, err := file.WriteString(fmt.Sprintf("%d, %d, %d, 0\n",
			line.time, line.value, line.opType))
		if err != nil {
			return fmt.Errorf("error write file [%s] %w", filePath, err)
		}

	}

	return nil
}

// parsingLogfile - parsing log file
func (t *LogFile) parsingLogfile(filePath string) error {
	logFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open log file %s err:%w", filePath, err)
	}
	defer logFile.Close()

	reader, err := csv.NewReader(logFile).ReadAll()
	if err != nil {
		return fmt.Errorf("could not read log file %s err:%w", filePath, err)
	}

	for _, line := range reader {
		// example line [119995  3481600  0  0]
		sTime, _ := strconv.Atoi(strings.TrimSpace(line[0]))
		sValue, _ := strconv.Atoi(strings.TrimSpace(line[1]))
		sOpType, _ := strconv.Atoi(strings.TrimSpace(line[2]))
		logLine := LogLine{
			time:   sTime,
			value:  sValue,
			opType: sOpType,
		}
		*t = append(*t, &logLine)
	}

	return nil
}

func styleDefaultsSeries(c *chart.Chart, seriesIndex int) chart.Style {
	return chart.Style{
		DotColor:    c.GetColorPalette().GetSeriesColor(seriesIndex),
		StrokeColor: c.GetColorPalette().GetSeriesColor(seriesIndex),
		StrokeWidth: chart.DefaultSeriesLineWidth,
		Font:        c.GetFont(),
		FontSize:    chart.DefaultFontSize,
	}
}

// getPoints - Get values for the x-axis
func getPoints(data LogFile, logType bs.LogFileType) ([]float64, []float64) {
	var xPoints []float64
	var yPoints []float64
	for _, point := range data {
		xPoints = append(xPoints,float64(point.time/1000)) // convert to seconds
		if logType == bs.LOG_TYPE_BW {
			yPoints = append(yPoints, mbps(point.value))
		} else {
			yPoints = append(yPoints, float64(point.value))
		}
	}
	return xPoints, yPoints
}


// drawlegend is a legend that is designed for longer series lists.
func drawlegend(c *chart.Chart, userDefaults ...chart.Style) chart.Renderable {
	return func(r chart.Renderer, cb chart.Box, chartDefaults chart.Style) {
		legendDefaults := chart.Style{
			FillColor:   drawing.ColorWhite,
			FontColor:   chart.DefaultTextColor,
			FontSize:    14.0,
			StrokeColor: chart.DefaultAxisColor,
			StrokeWidth: 2.0,
		}

		var legendStyle chart.Style
		if len(userDefaults) > 0 {
			legendStyle = userDefaults[0].InheritFrom(chartDefaults.InheritFrom(legendDefaults))
		} else {
			legendStyle = chartDefaults.InheritFrom(legendDefaults)
		}

		// DEFAULTS
		legendPadding := chart.Box{
			Top:    12,
			Left:   12,
			Right:  12,
			Bottom: 12,
		}

		lineTextGap := 5
		lineLengthMinimum := 25

		var labels []string
		var lines []chart.Style
		for index, s := range c.Series {
			if !s.GetStyle().Hidden {
				if _, isAnnotationSeries := s.(chart.AnnotationSeries); !isAnnotationSeries {
					labels = append(labels, s.GetName())
					lines = append(lines, s.GetStyle().InheritFrom(styleDefaultsSeries(c, index)))
				}
			}
		}

		legend :=chart.Box{
			Top: LogChartIndentLegend,
			Left: 50,
			// bottom and right will be sized by the legend content + relevant padding.
		}

		legendContent := chart.Box{
			Top:    legend.Top + legendPadding.Top,
			Left:   legend.Left + legendPadding.Left,
			Right:  legend.Left + legendPadding.Left,
			Bottom: legend.Top + legendPadding.Top,
		}

		legendStyle.GetTextOptions().WriteToRenderer(r)

		// measure
		labelCount := 0
		for x := 0; x < len(labels); x++ {
			if len(labels[x]) > 0 {
				tb := r.MeasureText(labels[x])
				if labelCount > 0 {
					legendContent.Bottom += chart.DefaultMinimumTickVerticalSpacing
				}
				legendContent.Bottom += tb.Height()
				right := legendContent.Left + tb.Width() + lineTextGap + lineLengthMinimum
				legendContent.Right = chart.MaxInt(legendContent.Right, right)
				labelCount++
			}
		}

 		legend = legend.Grow(legendContent)
		legend.Right = legendContent.Right + legendPadding.Right
		legend.Bottom = legendContent.Bottom + legendPadding.Bottom

		chart.Draw.Box(r, legend, legendStyle)

		legendStyle.GetTextOptions().WriteToRenderer(r)

		ycursor := legendContent.Top
		tx := legendContent.Left
		legendCount := 0
		var label string
		for x := 0; x < len(labels); x++ {
			label = labels[x]
			if len(label) > 0 {
				if legendCount > 0 {
					ycursor += chart.DefaultMinimumTickVerticalSpacing
				}

				tb := r.MeasureText(label)

				ty := ycursor + tb.Height()
				r.Text(label, tx, ty)

				th2 := tb.Height() >> 1

				lx := tx + tb.Width() + lineTextGap
				ly := ty - th2
				lx2 := legendContent.Right - legendPadding.Right

				r.SetStrokeColor(lines[x].GetStrokeColor())
				r.SetStrokeWidth(lines[x].GetStrokeWidth() + 4)
				r.SetStrokeDashArray(lines[x].GetStrokeDashArray())

				r.MoveTo(lx, ly)
				r.LineTo(lx2, ly)
				r.Stroke()

				ycursor += tb.Height()
				legendCount++
			}
		}
	}
}

func drawText(r chart.Renderer, yCursor, tx int, fontSize float64, str string, place chart.Box, chartDefaults chart.Style) int {
	sDefaults := chart.Style{
		FillColor:   drawing.ColorWhite,
		FontColor:   chart.DefaultTextColor,
		FontSize:    fontSize,

		StrokeWidth: 2.0,
	}

	textStyle := chartDefaults.InheritFrom(sDefaults)
	textStyle.GetTextOptions().WriteToRenderer(r)

	kb := r.MeasureText(str)
	ty := yCursor + kb.Height()
	r.Text(str, tx, ty)

	th2 := kb.Height() >> 1
	lx := tx + kb.Width() + 15
	ly := ty - th2
	lx2 := place.Left - 2

	r.MoveTo(lx, ly)
	r.LineTo(lx2, ly)
	yCursor += kb.Height() + 18

	return yCursor
}

// drawInfo is a legend that is designed for longer series lists.
func drawInfo(c *chart.Chart, info bs.LogFileInfo) chart.Renderable {
	return func(r chart.Renderer, cb chart.Box, chartDefaults chart.Style) {

		place := chart.Box{
			Top: LogChartIndentLegend,
			Left: 50 + 220,

		}
		// REWRITE ME PLS
		ycursor := place.Top
		tx := place.Left

		ycursor = drawText(r, ycursor, tx, 18.0, info.BasicInfoStr, place, chartDefaults)
		ycursor = drawText(r, ycursor, tx, 18.0, info.TestDescription, place, chartDefaults)
		ycursor = drawText(r, ycursor, tx, 16.0, info.InfoAboutFio, place, chartDefaults)
		ycursor = drawText(r, ycursor, tx, 16.0, info.Description, place, chartDefaults)
		drawText(r, ycursor, tx, 12.0, info.BSInfoString, place, chartDefaults)
	}
}


func createGraphForLog(data LogFile, logInfo bs.LogFileInfo, testName, discription, imgFormat string) error {

	xVal, YVal := getPoints(data, logInfo.FileType)
	mainSeries := chart.ContinuousSeries{
		Name:    logInfo.YName,
		YValues: YVal,
		XValues: xVal,
	}

	smaSeries := &chart.SMASeries{
		Name: "Average",
		InnerSeries: mainSeries,
		Style: chart.Style{
			StrokeColor: drawing.ColorRed,               // will supercede defaults
		},
	}

	graph := chart.Chart{
		Width: LogChartWidth,
		Height: LogChartHeight,
		Title: logInfo.Header,
		TitleStyle: chart.Style {
			FontSize:    30.0,
		},
		Background: chart.Style {
			Padding: chart.Box{
				Top:  LogChartPaddingTop,
				Left: LogChartPaddingLeft,
				Right: LogChartPaddingRight,
				Bottom: LogChartPaddingBottom,
			},
		},

		Series: []chart.Series {
			mainSeries,
			smaSeries,
		},

		YAxis: chart.YAxis {
			Name: logInfo.YName,
			NameStyle: chart.Style{
				FontSize:    20.0,
			},
			Style: chart.Style{
				FontSize:    15.0,
			},
			ValueFormatter: func(v interface{}) string {
				if vf, isFloat := v.(float64); isFloat {
					return fmt.Sprintf("%0.0f", vf)
				}
				return ""
			},
		},

		XAxis: chart.XAxis {
			Name: logInfo.XName,
			NameStyle: chart.Style{
				FontSize:    20.0,
			},
			Style: chart.Style{
				TextRotationDegrees: 45.0,
				FontSize:    12.5,
			},
			ValueFormatter: func(v interface{}) string {
				if vf, isFloat := v.(float64); isFloat {
					return fmt.Sprintf("%0.0f  ", vf)
				}
				return ""
			},
		},
	}

	graph.Elements = []chart.Renderable{
		drawlegend(&graph),
		drawInfo(&graph, logInfo),
	}

	logFraphPath := filepath.Join(logInfo.DirForImage, fmt.Sprintf("%s.%s", logInfo.ImgName, imgFormat))
	f, err := os.Create(logFraphPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s error: %w", logFraphPath, err)
	}
	defer f.Close()
	if imgFormat == "png" {
		if err := graph.Render(chart.PNG, f); err != nil {
			return fmt.Errorf("failed to render log chart: %v", err)
		}
	} else {
		if err := graph.Render(chart.SVG, f); err != nil {
			return fmt.Errorf("failed to render log chart: %v", err)
		}
	}

	return nil
}

// lineCounter - counts lines in  the log file
func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

// getCountLineInFile - get count lines in file
func getCountLineInFile(path string) int {
	file, _ := os.Open(path)
	countLine, err := lineCounter(file)
	if err != nil {
		fmt.Println("counting line in file failed! file:", path, "error: ", err)
		return 0
	}
	return countLine
}

// SepareteLogs - separate logs
func (t *LogGF) separeteLogs(fileList []fs.FileInfo, sessionPath string) error {
	haveName := false
	for _, file := range fileList {
		fTable := GroupLogFiles{}
		countLine := getCountLineInFile(fmt.Sprintf("%s/%s", sessionPath, file.Name()))
		fullPathToFile := fmt.Sprintf("%s/%s", sessionPath, file.Name())
		patternFile := strings.Split(file.Name(), ".")[0]
		haveName = false
		for _, value := range *t {
			if value.patternName == patternFile {
				value.filesPath = append(value.filesPath, fullPathToFile)
				if value.minCountLine > countLine {
					value.minCountLine = countLine
				}
				haveName = true
			}
		}
		if !haveName {
			fTable.patternName = patternFile
			fTable.filesPath = append(fTable.filesPath, fullPathToFile)
			fTable.minCountLine = countLine
			*t = append(*t, &fTable)
		}
	}
	return nil
}

// gluingFiles - gluing log files
func (t *LogGF) gluingFiles(resultsDir string) error {
	for _, value := range *t {
		var logDataMainFile = make(LogFile, 0)
		if err := logDataMainFile.parsingLogfile(value.filesPath[0]); err != nil {
			return fmt.Errorf("error with parsing log file %w", err)
		}
		if len(value.filesPath) == 1 {
			// There is nothing to glue here, just saving the file
			err := logDataMainFile.saveFile(filepath.Join(resultsDir, fmt.Sprintf("%s.log", value.patternName)), value.minCountLine)
			if err != nil {
				return fmt.Errorf("error create file %w", err)
			}
			continue

		}
		for index := 1; index < len(value.filesPath); index++ {
			var logTmpFile = make(LogFile, 0)
			if err := logTmpFile.parsingLogfile(value.filesPath[index]); err != nil {
				return fmt.Errorf("error with parsing log file %w", err)
			}
			for line := 0; line < value.minCountLine; line++ {
				logDataMainFile[line].value += logTmpFile[line].value
			}
		}

		err := logDataMainFile.saveFile(filepath.Join(resultsDir, fmt.Sprintf("%s.log", value.patternName)), value.minCountLine)
		if err != nil {
			return fmt.Errorf("error create file %w", err)
		}
	}
	return nil
}

func GetLogFilesFromGroup(dirWithLogs, mainResultsAbsDir string, fileList []fs.FileInfo) error {
	var logGroupF = make(LogGF, 0)
	if err := logGroupF.separeteLogs(fileList, dirWithLogs); err != nil {
		return fmt.Errorf("error with separeteLogs %w", err)
	}
	if err := logGroupF.gluingFiles(mainResultsAbsDir); err != nil {
		return fmt.Errorf("error with gluing log files %w", err)
	}

	return nil
}

// checkFolderWithLogs - check folder with logs and return list of files
func checkFolderWithLogs(dirWithLogs string) (map[string]string, error) {
	folders := make(map[string]string)
	if err := filepath.Walk(dirWithLogs, func(path string, file os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("could not read dir with log files: %w", err)
		}
		if file.Name() == filepath.Base(dirWithLogs) {
			return nil // skip root dir
		}
		if file.IsDir() {
			var problemDir bool
			if files, err := readDirWithResults(path); err != nil {
				return fmt.Errorf("could not read dir with log files: %w", err)
			} else if len(files) != 0 {
				for _, file := range files {
					if file.IsDir() || filepath.Ext(file.Name()) != ".log" {
						fmt.Println("There are unrecognized files in the log directory without the .log extension.",
							"No will be plotting graphs for this directory:", file.Name())
						problemDir = true
						break
					}
				}
				if !problemDir {
					folders[file.Name()] = path // maybe it's dir with logs
				}
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not read dir with log files: %w", err)
	}

	return folders, nil
}

func getTetsInfoFromJSON(allTests bs.AllTestInfo, testName string) (bs.TestInfo, error) {
	for _, test := range allTests.Tests {
		if test.TestName == testName {
			return test, nil
		}
	}
	return bs.TestInfo{}, fmt.Errorf("test %s not found in JSON data", testName)
}

func getJobsFromTestInfo(testInfo bs.TestInfo, fileName, description string) (bs.LogFileInfo, error) {
	logFinfo := bs.LogFileInfo{}
	found := false

	for _, job := range testInfo.JSONResults.Jobs {
		// Here we compare log files that have already been glued together.
		// This means that there should not be files named like write-64k-8_bw.435.log
		// The expected filename is something like this: write-64k-8_bw.log
		// It consists of the name specified in the fio configuration file and
		// the prefix with the log type
		switch fileName {
		case fmt.Sprintf("%s_bw.log", filepath.Base(job.TestOption.BwLog)):
			found = true
			logFinfo.FileType = bs.LOG_TYPE_BW
			logFinfo.YName = "MB/s"
			logFinfo.Header = fmt.Sprintf("Bandwidth for %s  [test: %s]", job.TestName, testInfo.TestName)
			logFinfo.ImgName = fmt.Sprintf("bw-%s", filepath.Base(job.TestOption.BwLog))
			if job.TestOption.RW == "read" || job.TestOption.RW == "randread" {
				logFinfo.BasicInfoStr = fmt.Sprintf("bw (Kib/s):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f,   samples=%d",
							 job.Read.BwMin, job.Read.BwMax, job.Read.BwMean, job.Read.BwDev, job.Read.BwSamples)
			} else {
				logFinfo.BasicInfoStr = fmt.Sprintf("bw (Kib/s):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f,   samples=%d",
							 job.Write.BwMin, job.Write.BwMax, job.Write.BwMean, job.Write.BwDev, job.Write.BwSamples)
			}
		case fmt.Sprintf("%s_iops.log", filepath.Base(job.TestOption.IOPSLog)):
			found = true
			logFinfo.FileType = bs.LOG_TYPE_IOPS
			logFinfo.YName = "IOPS"
			logFinfo.Header = fmt.Sprintf("IOPS for %s  [test: %s]", job.TestName, testInfo.TestName)
			logFinfo.ImgName = fmt.Sprintf("iops-%s", filepath.Base(job.TestOption.IOPSLog))
			if job.TestOption.RW == "read" || job.TestOption.RW == "randread" {
				logFinfo.BasicInfoStr = fmt.Sprintf("IOPS:   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f,   samples=%d",
							job.Read.IopsMin, job.Read.IopsMax, job.Read.IopsMean, job.Read.IopsStddev, job.Read.IopsSamples)
			} else {
				logFinfo.BasicInfoStr = fmt.Sprintf("IOPS:   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f,   samples=%d",
							job.Write.IopsMin, job.Write.IopsMax, job.Write.IopsMean, job.Write.IopsStddev, job.Write.IopsSamples)
			}
		case fmt.Sprintf("%s_lat.log", filepath.Base(job.TestOption.LatLog)):
			found = true
			logFinfo.FileType = bs.LOG_TYPE_LAT
			logFinfo.YName = "Nanoseconds"
			logFinfo.Header = fmt.Sprintf("Total latency for %s  [test: %s]", job.TestName, testInfo.TestName)
			logFinfo.ImgName = fmt.Sprintf("lat-%s", filepath.Base(job.TestOption.LatLog))
			if job.TestOption.RW == "read" || job.TestOption.RW == "randread" {
				logFinfo.BasicInfoStr = fmt.Sprintf("lat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Read.LatNS.Min, job.Read.LatNS.Max, job.Read.LatNS.Mean, job.Read.LatNS.Stddev)
			} else {
				logFinfo.BasicInfoStr = fmt.Sprintf("lat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Write.LatNS.Min, job.Write.LatNS.Max, job.Write.LatNS.Mean, job.Write.LatNS.Stddev)
			}
		case fmt.Sprintf("%s_clat.log", filepath.Base(job.TestOption.LatLog)):
			found = true
			logFinfo.FileType = bs.LOG_TYPE_CLAT
			logFinfo.YName = "Nanoseconds"
			logFinfo.Header = fmt.Sprintf("Completion latency for %s  [test: %s]", job.TestName, testInfo.TestName)
			logFinfo.ImgName = fmt.Sprintf("clat-%s", filepath.Base(job.TestOption.LatLog))
			if job.TestOption.RW == "read" || job.TestOption.RW == "randread" {
				logFinfo.BasicInfoStr = fmt.Sprintf("clat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Read.ClatNS.Min, job.Read.ClatNS.Max, job.Read.ClatNS.Mean, job.Read.ClatNS.Stddev)
			} else {
				logFinfo.BasicInfoStr = fmt.Sprintf("clat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Write.ClatNS.Min, job.Write.ClatNS.Max, job.Write.ClatNS.Mean, job.Write.ClatNS.Stddev)
			}
		case fmt.Sprintf("%s_slat.log", filepath.Base(job.TestOption.LatLog)):
			found = true
			logFinfo.FileType = bs.LOG_TYPE_SLAT
			logFinfo.YName = "Nanoseconds"
			logFinfo.Header = fmt.Sprintf("Submission latency for %s  [test: %s]", job.TestName, testInfo.TestName)
			logFinfo.ImgName = fmt.Sprintf("slat-%s", filepath.Base(job.TestOption.LatLog))
			if job.TestOption.RW == "read" || job.TestOption.RW == "randread" {
				logFinfo.BasicInfoStr = fmt.Sprintf("slat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Read.SlatNS.Min, job.Read.SlatNS.Max, job.Read.SlatNS.Mean, job.Read.SlatNS.Stddev)
			} else {
				logFinfo.BasicInfoStr = fmt.Sprintf("slat (nsec):   min=%d,   max=%d,   avg=%.2f,   stdev=%.2f",
							job.Write.SlatNS.Min, job.Write.SlatNS.Max, job.Write.SlatNS.Mean, job.Write.SlatNS.Stddev)
			}
		default:
			continue
		}

		if found {
			logFinfo.InfoJobs = &job
			logFinfo.TestDescription = fmt.Sprintf("job name: %s   |   bs: %s   |   iodepth: %s   |   num jobs: %s   |   rw: %s   |   group ID: %d",
										 job.TestName, job.TestOption.BS, job.TestOption.IODepth,
										 job.TestOption.NumJobs, job.TestOption.RW, job.GroupID)
			logFinfo.XName = "Time line in seconds"
			logFinfo.InfoAboutFio = fmt.Sprintf("Date: %s   |   version.%s   |   IO engine: %s   |   LogAvgMsec=%s   |   size=%s   | direct=%s",
										testInfo.JSONResults.Time, testInfo.JSONResults.FioVersion,
										testInfo.JSONResults.GlobalOptions.Ioengine,
										testInfo.JSONResults.GlobalOptions.LogAvgMsec,
										testInfo.JSONResults.GlobalOptions.Size,
										testInfo.JSONResults.GlobalOptions.Direct)
			logFinfo.BSInfoString = "Created in fioplot-bs. https://github.com/vk-en/fioplot-bs"
			logFinfo.Description = fmt.Sprintf("Description: %s", description)
			return logFinfo, nil
		}
	}
	return logFinfo, fmt.Errorf("no found information in JSON about log file: %s in test: %s",
					 fileName, testInfo.TestName)
}

func getFinishDirForGraph(dir string, fType bs.LogFileType) string {
	// create folder for graph
	dirName := ""
	switch fType {
	case bs.LOG_TYPE_BW:
		dirName = filepath.Join(dir, "bw")
	case bs.LOG_TYPE_IOPS:
		dirName = filepath.Join(dir, "iops")
	case bs.LOG_TYPE_LAT:
		dirName = filepath.Join(dir, "lat")
	case bs.LOG_TYPE_CLAT:
		dirName = filepath.Join(dir, "clat")
	case bs.LOG_TYPE_SLAT:
		dirName = filepath.Join(dir, "slat")
	default:
		return dir
	}

	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err = os.Mkdir(dirName, 0755); err != nil {
			fmt.Println("Error creating directory for graph: ", err)
		}
	}
	return dirName

}

// CreateGraphsFromLogs - create graphs from logs
func CreateGraphsFromLogs(allResults bs.AllTestInfo) error {
	mainResultsAbsDirCharts := filepath.Join(allResults.MainPathToResults, "log-graphs")
	if err := os.Mkdir(mainResultsAbsDirCharts, 0755); err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	listWithLogsFolders, err := checkFolderWithLogs(allResults.PathWithSrcResults)
	if err != nil {
		return fmt.Errorf("could not check folder with logs %w", err)
	}

	for testName, pathToLogs := range listWithLogsFolders {
		testInfo, err := getTetsInfoFromJSON(allResults, testName)
		if err != nil {
			fmt.Println("could not get test info %w", err)
		}

		// main dir for graphs
		testNameDir := filepath.Join(mainResultsAbsDirCharts, fmt.Sprintf("%s-log-graphs", testName))
		if err := os.Mkdir(testNameDir, 0755); err != nil {
			return fmt.Errorf("could not create local dir for result: %w", err)
		}

		// tmp dir for glued logs
		curentLogsAbsDir := filepath.Join(allResults.MainPathToResults, fmt.Sprintf("%s-GluedLF", testName))
		err = os.Mkdir(curentLogsAbsDir, 0755)
		if err != nil {
			return fmt.Errorf(
				"could not create tmp dir: %s for result: %s err:%w",
				curentLogsAbsDir, testName, err)
		}

		fileWithResultsTmp, err := readDirWithResults(pathToLogs)
		if err != nil {
			return fmt.Errorf("could not read dir with log files: %w", err)
		}

		if err := GetLogFilesFromGroup(pathToLogs, curentLogsAbsDir, fileWithResultsTmp); err != nil {
			return fmt.Errorf("could not glued log files: %w", err)
		}

		gluedFilesWithResults, err := readDirWithResults(curentLogsAbsDir)
		if err != nil {
			return fmt.Errorf("could not read dir with log files: %w", err)
		}

		for _, fileName := range gluedFilesWithResults {
			if !fileName.IsDir() {
				var logData = make(LogFile, 0)

				logInfo, err := getJobsFromTestInfo(testInfo, fileName.Name(), allResults.Description)
				if err != nil {
					return fmt.Errorf("could not get jobs from test info: %w", err)
				}
				logInfo.DirForImage = getFinishDirForGraph(testNameDir, logInfo.FileType)

				if err := logData.parsingLogfile(filepath.Join(curentLogsAbsDir, fileName.Name())); err != nil {
					return fmt.Errorf("could not parse log file: %w", err)
				}

				if err := createGraphForLog(logData, logInfo, testName,
					allResults.Description, allResults.ImgFormat); err != nil {
					return fmt.Errorf("could not create log graphs: %w", err)
				}
			}
		}

		if err := os.RemoveAll(curentLogsAbsDir); err != nil {
			return fmt.Errorf("could not delete tmp dir: %s for result: %s err:%w",
				curentLogsAbsDir, testName, err)
		}
		delete(listWithLogsFolders, testName)
	}

	return nil
}
