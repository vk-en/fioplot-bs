package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	bar "github.com/vk-en/fioplot-bs/pkg/barchart"
	csv "github.com/vk-en/fioplot-bs/pkg/csvtable"
//	log "github.com/vk-en/fioplot-bs/pkg/loggraphs"
	xlsx "github.com/vk-en/fioplot-bs/pkg/xlsxchart"
	bs "github.com/vk-en/fioplot-bs/pkg/bsdata"
)

// Options - command line arguments
type Options struct {
	TestName    string `short:"n" long:"name" description:"Name for folder with results" required:"true"`
	Catalog     string `short:"c" long:"catalog" description:"Full path to catalog with *.json files and/or catalogs with *.log results (Ex. /home/user/dirWithResults)" required:"true"`
	ImgFormat   string `short:"f" long:"format" description:"Format of an images with charts" default:"png" choice:"png" choice:"svg"`
	Description string `short:"d" long:"description" description:"Description for image results" default:"github.com/vk-en/fioplot-bs"`
	LogGraphs   bool   `short:"l" long:"loggraphs" description:"Create log graphs" optionalArgument:"true"`
}

const (
	version = "1.0.0"
)

var opts Options
var parser = flags.NewParser(&opts, flags.Default)
var pathToResults string
var csvFiles []string

// argparse - parse command line arguments
func argparse() {
	if _, err := parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case flags.ErrorType:
			if flagsErr == flags.ErrHelp {
				os.Exit(0)
			}
			os.Exit(1)
		default:
			os.Exit(1)
		}
	}
}

// createFolderForResults - if folder already exists create new folder with new name
func createFolderForResults(folderName string) (string, error) {
	curentlyPath, _ := os.Getwd()
	fullPath := filepath.Join(curentlyPath, folderName)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if err := os.Mkdir(fullPath, 0755); err != nil {
			return "", err
		}
		return fullPath, nil
	} else {
		fmt.Printf("folder with results [%s] already exists.\n", fullPath)
		fmt.Println("Enter a new directory name with results:")
		var newName string
		fmt.Scanf("%s\n", &newName)
		if fullPath, err = createFolderForResults(newName); err != nil {
			return "", err
		}
	}
	return fullPath, nil
}

// createCSVTable - create CSV table from JSON file
func createCSVTables(allTestInfo bs.AllTestInfo) error {
	csvFolderPath := filepath.Join(allTestInfo.MainPathToResults, "csv-tables")
	if _, err := os.Stat(csvFolderPath); os.IsNotExist(err) {
		if err := os.Mkdir(csvFolderPath, 0755); err != nil {
			return err
		}
	}
	// Create CSV tables for each tests
	for _, testResults := range allTestInfo.Tests {
		testResults.CSVFileName = fmt.Sprintf("%s.%s", testResults.TestName, "csv")
		testResults.CSVFilePath = filepath.Join(csvFolderPath, testResults.CSVFileName)
		if err := csv.ConvertJSONtoCSV(testResults.JSONResults, testResults.CSVFilePath); err != nil {
			fmt.Printf("could not create CSV table for file [%s]\n. Error: %v\n",
						 testResults.TestName, err)
			continue
		}
		// Temporary solution
		csvFiles = append(csvFiles, filepath.Join(csvFolderPath, testResults.CSVFileName))
	}

	return nil
}

// makeResults - create folder with results (xlsx, csv, and BarChars img)
func makeResults() error {
	var err error
	if pathToResults, err = createFolderForResults(opts.TestName); err != nil {
		return fmt.Errorf("could not create folder for results: %w", err)
	}

	allResults, err := bs.ReadAllJSONFiles(opts.Catalog)
	if err != nil {
		cleanUpDir()
		return fmt.Errorf("could not read all JSON files: %w", err)
	}

	allResults.MainPathToResults = pathToResults
	allResults.PathWithSrcResults = opts.Catalog
	allResults.ImgFormat = opts.ImgFormat
	allResults.Description = opts.Description

/* 	if opts.LogGraphs {
		if err := log.CreateGraphsFromLogs(pathToResults, opts.Catalog, opts.Description, opts.ImgFormat); err != nil {
			return fmt.Errorf("could not create graphs from logs: %w", err)
		}
	} */

	// Create CSV tables for each JSON file
	if err := createCSVTables(allResults); err != nil {
		return fmt.Errorf("could not create CSV tables: %w", err)
	}

/* 	if len(csvFiles) == 0 {
		cleanUpDir()
		return fmt.Errorf(fmt.Sprintf("%s\n%s\n",
			"Failed to read FIO results from JSON.",
			"Results and graphs were not generated =("))
	} */

	if err := xlsx.CreateXlsxReport(csvFiles, pathToResults); err != nil {
		// not a critical error, can move next, just log it
		fmt.Printf("could not create xsls file.\n Error: %v\n", err)
	}

	//Create bar charts
	if err := bar.CreateBarCharts(csvFiles, opts.Description, pathToResults, opts.ImgFormat); err != nil {
		fmt.Printf("could not create barCharts.\n Error: %v\n", err)
	} else {
		fmt.Println("Results and graphs were generated successfully!")
	}

	fmt.Println("Results are in folder:", pathToResults)
	return nil
}

// cleanUpDir - remove all files from directory for current test
func cleanUpDir() {
	if err := os.RemoveAll(pathToResults); err != nil {
		fmt.Printf("could not remove folder with results: %v", err)
	}
}

func main() {
	argparse()
	if err := makeResults(); err != nil {
		fmt.Println(err)
	}
}

func init() {
	fmt.Println("fioplot-bs version ", version)
}
