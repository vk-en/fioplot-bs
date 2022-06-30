package barchart

import (
	"fmt"
	"os"
	"path/filepath"

	data "github.com/vk-en/fioplot-bs/pkg/getdata"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

// allLegendResults - structure for storing data for all legends
type allLegendResults struct {
	legend       string
	value        []float64
	pattern      []string
	yDiscription string
	fileName     string
}

// legensTable - structure for storing data for all legends
type legensTable []*allLegendResults

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

// countingSizeCanvas - counting size of canvas
func countingSizeCanvas(countBar int) (font.Length, font.Length) {
	if countBar > 22 {
		return 30 * vg.Inch, 7 * vg.Inch
	} else if countBar > 60 {
		return 55 * vg.Inch, 12 * vg.Inch
	} else if countBar > 100 {
		return 110 * vg.Inch, 12 * vg.Inch
	}

	return 10 * vg.Inch, 7 * vg.Inch
}

//getLegendsTable - Gets structures based on legends from patterns
func (t *legensTable) getLegendsTable(patternTable data.PatternsTable) {
	for _, ilegend := range patternTable[0].Legends {
		fTable := allLegendResults{
			legend: ilegend,
		}
		*t = append(*t, &fTable)
	}

	for _, ilegend := range *t {
		for _, pattern := range patternTable {
			for g := 0; g < len(pattern.Values); g++ {
				if ilegend.legend == pattern.Legends[g] {
					ilegend.value = append(ilegend.value, pattern.Values[g])
					ilegend.pattern = append(ilegend.pattern, pattern.PatternName)
				}
			}
			ilegend.yDiscription = pattern.YDiscription
			ilegend.fileName = pattern.FileName
		}
	}
}

//createGeneralBarCharts - generate bar charts for all groups between different tests
func createGeneralBarCharts(table data.PatternsTable, description, dirPath, imgType string) error {

	var lTable = make(legensTable, 0)
	lTable.getLegendsTable(table)
	p, _ := plotCreate(lTable[0].fileName, lTable[0].yDiscription, description, float64(len(table)))
	w := vg.Points(3)
	start := 0 - w
	for k := 0; k < len(lTable); k++ {
		var data plotter.Values
		data = lTable[k].value
		bars, _ := plotter.NewBarChart(data, w)
		bars.LineStyle.Width = vg.Length(0)
		bars.Width = font.Length(2)
		bars.Color = plotutil.Color(k)

		start = start + w
		bars.Offset = start
		p.Add(bars)
		p.Legend.Add(lTable[k].legend, bars)
	}

	p.X.Tick.Label.Rotation = -125
	ticks := make([]plot.Tick, len(table))
	for i, name := range table {
		ticks[i] = plot.Tick{float64(i), name.PatternName}
	}
	p.X.Tick.Width = font.Length(8)
	p.X.Tick.Marker = plot.ConstantTicks(ticks)
	p.X.Tick.Label.XAlign = -0.8

	width, height := countingSizeCanvas(len(table))

	if err := p.Save(width, height, filepath.Join(dirPath, fmt.Sprintf("%s.%s", lTable[0].fileName, imgType))); err != nil {
		return fmt.Errorf("generate BarCharts failed! err:%v", err)
	}
	return nil
}

//createSeparateBarCharts - generate bar charts for only one groups between different tests
func createSeparateBarCharts(table data.PatternsTable, description, dirPath, imgType string) error {
	resultsAbsDir := filepath.Join(dirPath, table[0].FileName)
	err := os.Mkdir(resultsAbsDir, 0755)
	if err != nil {
		fmt.Println("could not create local dir for result: %w", err)
	}

	for _, pattern := range table {
		p, _ := plotCreate(pattern.FileName, pattern.YDiscription, description, float64(len(table)))
		w := vg.Points(7)
		p.NominalX(pattern.PatternName)
		start := 0 - w
		for i := 0; i < len(pattern.Values); i++ {
			bars, err := plotter.NewBarChart(plotter.Values{pattern.Values[i]}, w)
			if err != nil {
				return fmt.Errorf("generate BarCharts for [%s] failed! err:%v", pattern.PatternName, err)
			}
			bars.LineStyle.Width = vg.Length(0)
			bars.Color = plotutil.Color(i)
			bars.Width = font.Length(4)
			start = start + w
			bars.Offset = start
			p.Add(bars)
			p.Legend.Add(pattern.Legends[i], bars)
		}
		if err := p.Save(4*vg.Inch, 7*vg.Inch,
			filepath.Join(resultsAbsDir, fmt.Sprintf("%s.%s", pattern.PatternName, imgType))); err != nil {
			return fmt.Errorf("generate BarCharts for [%s] failed! err:%v",
				pattern.PatternName, err)
		}
	}
	return nil
}

// CreateBarCharts - generate bar charts for all groups between different tests
func CreateBarCharts(csvFiles []string, descriptionForCharts, pathForResults, imgType string) error {
	var testResults = make(data.AllResults, 0)

	barChartAbsDir := filepath.Join(pathForResults, "bar-charts")
	err := os.Mkdir(barChartAbsDir, 0755)
	if err != nil {
		return fmt.Errorf("could not create BarCharts dir for results. error: %w", err)
	}

	for _, file := range csvFiles {
		if err := testResults.ParsingCSVfile(file); err != nil {
			return fmt.Errorf("parsing csv file [%s] failed! err:%v", file, err)
		}
	}

	identicalPatterns, err := data.GetIdenticalPatterns(testResults)
	if err != nil {
		return fmt.Errorf("could not get identical patterns: %w", err)
	}

	for _, valRes := range []uint16{performance, minIOPS, maxIOPS, minBW, maxBW, minLat, maxLat, stdLat, p99Lat} {
		var pTable = make(data.PatternsTable, 0)
		pTable.GetPatternTable(identicalPatterns, testResults, valRes)
		if err := createSeparateBarCharts(pTable, descriptionForCharts, barChartAbsDir, imgType); err != nil {
			return fmt.Errorf("generate BarChart failed! err:%v", err)
		}

		if err := createGeneralBarCharts(pTable, descriptionForCharts, barChartAbsDir, imgType); err != nil {
			return fmt.Errorf("generate BarCharts failed! err:%v", err)
		}
	}

	return nil
}
