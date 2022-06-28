package xlsxchart

import (
	"fmt"
	"path/filepath"

	data "github.com/vk-en/fioplot-bs/pkg/getdata"
	"github.com/xuri/excelize/v2"
)

const barTpl = `
	{
		"name": "%s",
		"categories": "%s!%s",
		"values": "%s!%s"
	}%s`

const globalTpl = `{
	"type": "col",
	"series":
	%s
	,
	"y_axis":
	{
		"major_grid_lines": true,
		"minor_grid_lines": true
	},
	"x_axis":
	{
		"major_grid_lines": true
	},
	"legend":
	{
		"position": "left",
		"show_legend_key": true
	},
	"title":
	{
		"name": "%s"
	}
}`

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

//
func genExcelfile(pathFile string) bool {
	f := excelize.NewFile()
	if err := f.SaveAs(pathFile); err != nil {
		fmt.Println("could save xlsx file failed %w", err)
		return false
	}
	return true
}

// createExcelCharts - create charts in Excel file
func createExcelCharts(countStroke int, filePath string) error {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("open %s xlsx file failed: %w", filePath, err)
	}
	//letters with a margin
	str := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N"}
	finishStr := []string{}
	sheets := f.GetSheetList()
	for index, sheet := range sheets {
		value, _ := f.GetCellValue(sheet, fmt.Sprintf("%s1", str[index]))
		if len(value) == 0 {
			break
		} else if value == "Pattern" {
			continue
		}
		finishStr = append(finishStr, str[index])
	}

	f.NewSheet("Bars")
	barsIndent := 1

	for _, sheet := range sheets {
		series := []string{}
		point := ","
		for i, char := range finishStr {
			n1, err := f.GetCellValue(sheet, fmt.Sprintf("%s1", char))
			if err != nil {
				return fmt.Errorf("could not get cell value: %w", err)
			}
			if i == len(finishStr)-1 {
				point = ""
			}
			bar := fmt.Sprintf(barTpl, n1,
				sheet, fmt.Sprintf("$A$%d:$A$%d", 2, countStroke),
				sheet, fmt.Sprintf("$%s$%d:$%s$%d", char, 2, char, countStroke), point)
			series = append(series, bar)
		}

		chartsP := fmt.Sprintf(globalTpl, series, sheet)

		if err := f.AddChart("Bars", fmt.Sprintf("B%d", barsIndent), chartsP); err != nil {
			fmt.Println(err)
		}
		barsIndent += 20
	}

	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("could save xlsx file failed %w", err)
	}
	return nil
}

//createExcelTables - generate table for all groups between different tests in Excel
func createExcelTables(table data.PatternsTable, filePath string) error {

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("open %s xlsx file failed: %w", filePath, err)
	}

	rowIter := 2
	sheetName := ""
	for _, pattern := range table {
		if sheetName != pattern.FileName {
			sheetName = pattern.FileName
			var headerXSLS = []string{"Pattern"}
			f.NewSheet(sheetName)
			if err := f.SetColWidth(sheetName, "A", "A", 25); err != nil {
				return fmt.Errorf("could not set column width: %w", err)
			}
			if err := f.SetSheetRow(sheetName, "A1", &headerXSLS); err != nil {
				return fmt.Errorf("could not set row: %w", err)
			}
			if err := f.SetSheetRow(sheetName, "B1", &pattern.Legends); err != nil {
				return fmt.Errorf("could not set row: %w", err)
			}
			rowIter = 2
		}
		var patternName = []string{pattern.PatternName}
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("A%d", rowIter), &patternName); err != nil {
			return fmt.Errorf("could not set row: %w", err)
		}
		if err := f.SetSheetRow(sheetName, fmt.Sprintf("B%d", rowIter), &pattern.Values); err != nil {
			return fmt.Errorf("could not set row: %w", err)
		}
		rowIter++
	}

	f.DeleteSheet("Sheet1") //remove default sheet
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("could save xlsx file failed %w", err)
	}

	return nil
}

// CreateXlsxReport - create xlsx report with table and charts
func CreateXlsxReport(csvFiles []string, pathForResults string) error {
	var testResults = make(data.AllResults, 0)
	countStroke := 0

	testName := filepath.Base(pathForResults)
	mainResultsFile := filepath.Join(pathForResults, fmt.Sprintf("%s.xlsx", testName))
	if !genExcelfile(mainResultsFile) {
		return fmt.Errorf("could not create excel file")
	}

	for _, file := range csvFiles {
		if err := testResults.ParsingCSVfile(file); err != nil {
			return fmt.Errorf("could not parse csv file: %w", err)
		}
	}

	identicalPatterns, err := data.GetIdenticalPatterns(testResults)
	if err != nil {
		return fmt.Errorf("could not get identical patterns: %w", err)
	}

	for _, valRes := range []uint16{performance, minIOPS, maxIOPS, minBW, maxBW, minLat, maxLat, stdLat, p99Lat} {
		var pTable = make(data.PatternsTable, 0)
		pTable.GetPatternTable(identicalPatterns, testResults, valRes)
		if err := createExcelTables(pTable, mainResultsFile); err != nil {
			return fmt.Errorf("could not create table in Xlsx file: %w", err)
		}
		countStroke = len(pTable)
	}

	if err := createExcelCharts(countStroke+1, mainResultsFile); err != nil {
		return fmt.Errorf("could not create excel charts: %w", err)
	}

	return nil
}
