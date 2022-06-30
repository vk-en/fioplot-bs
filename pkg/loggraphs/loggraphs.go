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

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
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

//plotCreate - сreates a skeleton for plotting graphs
func plotCreate(testName, typeVolume, description string, xMax float64) (*plot.Plot, error) {
	p := plot.New()
	p.Title.Text = testName
	p.Title.TextStyle.Font.Size = font.Length(20)
	p.Y.Label.Text = typeVolume
	p.Y.Label.Padding = 10
	p.X.Label.Text = description
	p.X.Label.TextStyle.Font.Size = font.Length(10)
	p.X.Label.Padding = 25
	p.Legend.Top = true
	p.Legend.YOffs = vg.Length(+40)
	p.Legend.XOffs = vg.Length(-20)
	p.Legend.Padding = 2
	p.X.Max = xMax
	p.Title.Padding = 40
	p.X.Tick.Width = 0
	p.X.Tick.Length = 0
	p.X.Width = 0
	p.Add(plotter.NewGrid())
	return p, nil
}

// getPoints - Get values for the x-axis
func getPoints(data LogFile, logType int) plotter.XYs {
	pts := make(plotter.XYs, len(data))
	for i, point := range data {
		pts[i].X = float64(i)
		if logType == 1 {
			pts[i].Y = mbps(point.value)
		} else {
			pts[i].Y = float64(point.value)
		}
	}
	return pts
}

//getInfoAboutLogFile   return: fileType (1-bw,2-iops,3-lat), Y-name, test Name, error
func getInfoAboutLogFile(fileName string) (int, string, string, error) {
	testName := strings.Split(fileName, ".")
	var err error

	bwLog := strings.Contains(fileName, "bw")
	if bwLog {
		return 1, "MB/s", testName[0], nil
	}

	iopsLog := strings.Contains(fileName, "iops")
	if iopsLog {
		return 2, "IOPS", testName[0], nil
	}

	latLog := strings.Contains(fileName, "lat")
	if latLog {
		return 3, "msec", testName[0], nil
	}

	return 0, "", "", err
}

// addLinePoints adds Line and Scatter plotters to a
// plot.  The variadic arguments must be either strings
// or plotter.XYers.  Each plotter.XYer is added to
// the plot using the next color, dashes, and glyph
// shape via the Color, Dashes, and Shape functions.
// If a plotter.XYer is immediately preceeded by
// a string then a legend entry is added to the plot
// using the string as the name.
//
// If an error occurs then none of the plotters are added
// to the plot, and the error is returned.
func addLinePoints(plt *plot.Plot, vs ...interface{}) error {
	var ps []plot.Plotter
	type item struct {
		name  string
		value [2]plot.Thumbnailer
	}
	var items []item
	name := ""
	var i int
	for _, v := range vs {
		switch t := v.(type) {
		case string:
			name = t

		case plotter.XYer:
			l, s, err := plotter.NewLinePoints(t)
			if err != nil {
				return err
			}
			l.Color = plotutil.Color(2)
			l.Dashes = plotutil.Dashes(0)
			l.StepStyle = plotter.NoStep
			s.Color = plotutil.Color(10)
			s.Shape = nil
			i++
			ps = append(ps, l, s)
			if name != "" {
				items = append(items, item{name: name, value: [2]plot.Thumbnailer{l, s}})
				name = ""
			}

		default:
			return fmt.Errorf("plotutil: AddLinePoints handles strings and plotter.XYers, got %T", t)
		}
	}
	plt.Add(ps...)
	for _, item := range items {
		v := item.value[:]
		plt.Legend.Add(item.name, v[0], v[1])
	}
	return nil
}

// countingSizeLogCanvas - counting size of log canvas
func countingSizeLogCanvas(countX int) (font.Length, font.Length) {
	if countX > 30000 {
		return 400 * vg.Inch, 10 * vg.Inch
	} else if countX > 20000 {
		return 250 * vg.Inch, 7 * vg.Inch
	} else if countX > 10000 {
		return 150 * vg.Inch, 7 * vg.Inch
	} else if countX > 5000 {
		return 100 * vg.Inch, 7 * vg.Inch
	} else if countX > 1000 {
		return 70 * vg.Inch, 7 * vg.Inch
	} else if countX > 500 {
		return 35 * vg.Inch, 7 * vg.Inch
	} else if countX > 100 {
		return 20 * vg.Inch, 7 * vg.Inch
	}
	return 10 * vg.Inch, 7 * vg.Inch
}

// createGraphForLog - creates graph for log file
func (t *LogFile) createGraphForLog(data LogFile, logType int, yName, testName, discription, DirResPath, imgFormat string) error {
	p, _ := plotCreate(testName, yName, discription, float64(len(data)))
	err := addLinePoints(p, yName, getPoints(data, logType))
	if err != nil {
		return fmt.Errorf("error with get values for the Y-axis %w", err)
	}

	width, height := countingSizeLogCanvas(len(data))
	if err := p.Save(width, height, filepath.Join(DirResPath, fmt.Sprintf("%s.%s", testName, imgFormat))); err != nil {
		return fmt.Errorf("error with save charts %w", err)
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
		fmt.Println("counting line in file failed! error:%w", path, err)
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

// CreateGraphsFromLogs - create graphs from logs
func CreateGraphsFromLogs(dirWithResults, dirWithLogs, discription, ImgFormat string) error {
	mainResultsAbsDirCharts := filepath.Join(dirWithResults, "log-graphs")
	if err := os.Mkdir(mainResultsAbsDirCharts, 0755); err != nil {
		return fmt.Errorf("could not create local dir for result: %w", err)
	}

	listWithLogsFolders, err := checkFolderWithLogs(dirWithLogs)
	if err != nil {
		return fmt.Errorf("could not check folder with logs %w", err)
	}

	for testName, pathToLogs := range listWithLogsFolders {
		testNameDir := filepath.Join(mainResultsAbsDirCharts, fmt.Sprintf("%s-log-graphs", testName))
		if err := os.Mkdir(testNameDir, 0755); err != nil {
			return fmt.Errorf("could not create local dir for result: %w", err)
		}

		curentLogsAbsDir := filepath.Join(dirWithResults, fmt.Sprintf("%s-GluedLF", testName))
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
				fType, yName, testName, _ := getInfoAboutLogFile(fileName.Name())
				if err := logData.parsingLogfile(filepath.Join(curentLogsAbsDir, fileName.Name())); err != nil {
					return fmt.Errorf("could not parse log file: %w", err)
				}

				if err := logData.createGraphForLog(logData, fType, yName, testName,
					discription, testNameDir, ImgFormat); err != nil {
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
