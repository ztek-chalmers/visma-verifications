package main

import (
	"bytes"
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/tealeg/xlsx"
)

type Args struct {
	Format string `default:"xlsx" help:"output format, valid values are xlsx and csv"`
	Input  string `arg:"positional, required" help:"input CSV file to read"`
	Output string `arg:"positional" help:"output directory, if omitted this will be INPUT-split"`
}

func (Args) Description() string {
	return `An application which splits a single Visma verifiaction list, CSV file, based on result units. 
For Z this would result in a file split by committees.`
}

type ResultUnit struct {
	Name   string
	Buffer []Line
}
type DumpFn func(buffer []Line) ([]byte, string, error)
type Line []string

func main() {
	inFile, outDir, dumpFn := parseCLIArguments()
	defaultResultUnitName, resultMap := getConfiguration()
	lines := readFile(inFile)
	resultUnits := findResultUnits(lines)
	var defaultResultUnit *ResultUnit
	for _, r := range resultUnits {
		if r.Name == defaultResultUnitName {
			defaultResultUnit = r
			break
		}
	}
	if defaultResultUnit == nil {
		panic("A result unit with the name Ztyret was not found ")
	}
	splitFileByResult(resultUnits, defaultResultUnit, lines)
	writeFile(resultUnits, resultMap, dumpFn, outDir)
}

func getConfiguration() (string, map[string]string) {
	// Hard coded for Automation och Mekatronik
	defaultResultUnitName := "Ztyret"
	resultMap := map[string]string{
		"IntrezzeK": "Ztyret",
		"Revisorer": "Ztyret",
		"VB":        "Ztyret",
		"ZKK":       "Ztyret",
		"Zpel":      "Ztyret",
		"Ztyret":    "Ztyret",
		"ZÅG":       "Ztyret",
		"WebGroup":  "Ztyret",
	}
	return defaultResultUnitName, resultMap
}

func parseCLIArguments() (string, string, DumpFn) {
	var args Args
	arg.MustParse(&args)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get the current working directory: %s", err)
	}
	inFile := args.Input
	if !path.IsAbs(inFile) {
		inFile = path.Join(wd, inFile)
	}
	outDir := inFile + "-split"
	if args.Output != "" {
		outDir = args.Output
		if !path.IsAbs(outDir) {
			outDir = path.Join(wd, outDir)
		}
	}
	var outFormat DumpFn
	switch args.Format {
	case "csv":
		outFormat = dumpCSV
	case "xlsx":
		outFormat = dumpXLSX
		break
	default:
		log.Fatalf("Invalid output format specified %s. Only csv and xlsx are supported", args.Format)
	}
	return inFile, outDir, outFormat
}

func readFile(inFile string) []Line {
	fileReader, err := os.Open(inFile)
	if err != nil {
		log.Fatalf("Failed to start the application: %s", err)
	}
	csvReader := csv.NewReader(fileReader)
	csvReader.LazyQuotes = true
	csvReader.Comma = ';'

	strs, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	lines := make([]Line, len(strs))
	for i, s := range strs {
		lines[i] = s
	}
	return lines
}

// splitFileByResult splits the input lines by result units.
//
// Summarized this function will:
// 1. Include common fields like the csv header in all result units
// 2. When a debit/kredit line is found, the result unit for that line
//    is set as the current result unit, only result units in this list will
//    get the previous "verifikat" line as well as all lines until the next
//    "verifikat".
// 3. When a new "verifikat" is found, the buffer built up is flushed to all
//    the current result unit's, in other words the kommittees affected by this
//    "verifikat". The new verifikat is added to the buffer and [2] is repeated.
//
func splitFileByResult(resultUnits []*ResultUnit, defaultResultUnit *ResultUnit, lines []Line) []*ResultUnit {
	currentResultUnits := make([]*ResultUnit, 0)
	buffer := make([]Line, 0)
	for _, line := range lines {
		// Sanity check
		if len(line) != 8 {
			log.Fatal("Invalid CSV file, the line did not contain 8 elements")
		}
		parsed := parseDebetCreditLine(line, resultUnits, &buffer, &currentResultUnits)
		if parsed {
			continue
		}
		parsed = parseEmptyLine(line, resultUnits, &buffer, &currentResultUnits)
		if parsed {
			continue
		}
		parsed = parseNewVerificatLine(line, defaultResultUnit, &buffer, &currentResultUnits)
		if parsed {
			continue
		}
	}
	return resultUnits
}

func parseDebetCreditLine(line Line, resultUnits []*ResultUnit, buffer *[]Line, currentResultUnits *[]*ResultUnit) bool {
	owner := line[5]
	if owner != "" && !strings.HasPrefix(owner, "\"") {
		exists := false
		for _, o := range *currentResultUnits {
			if o.Name == owner {
				exists = true
				break
			}
		}
		if !exists {
			for _, o := range resultUnits {
				if o.Name == owner {
					*currentResultUnits = append(*currentResultUnits, o)
				}
			}
		}
		*buffer = append(*buffer, line)
		return true
	}
	return false
}

func parseEmptyLine(line Line, _ []*ResultUnit, buffer *[]Line, _ *[]*ResultUnit) bool {
	ver := line[0]
	if ver == "" {
		*buffer = append(*buffer, line)
		return true
	}
	return false
}

func parseNewVerificatLine(line Line, defaultResultUnit *ResultUnit, buffer *[]Line, currentResultUnits *[]*ResultUnit) bool {
	if len(*currentResultUnits) == 0 {
		*currentResultUnits = []*ResultUnit{defaultResultUnit}
	}
	for _, b := range *buffer {
		buff := b
		for _, o := range *currentResultUnits {
			o.Buffer = append(o.Buffer, buff)
		}
	}
	*currentResultUnits = make([]*ResultUnit, 0)
	*buffer = make([]Line, 0)
	*buffer = append(*buffer, line)
	return true
}

func findResultUnits(lines []Line) []*ResultUnit {
	ownerMap := make(map[string]bool)
	for _, line := range lines {
		owner := line[5]
		if strings.Index(owner, "\"") == 0 {
			continue
		}
		if owner == "" {
			continue
		}
		ownerMap[owner] = true
	}
	resultUnits := make([]*ResultUnit, 0)
	for o := range ownerMap {
		resultUnits = append(resultUnits, &ResultUnit{
			Name:   o,
			Buffer: make([]Line, 0),
		})
	}
	return resultUnits
}

func dumpXLSX(buffer []Line) ([]byte, string, error) {
	excel := xlsx.NewFile()
	sheet, err := excel.AddSheet("Sheet1")
	if err != nil {
		return nil, "", err
	}
	widths := []float64{10, 15, 70, 10, 30, 10, 10, 10}
	for _, r := range buffer {
		row := sheet.AddRow()
		for _, c := range r {
			cell := row.AddCell()
			cell.Value = c
		}
	}
	for i, c := range sheet.Cols {
		c.Width = widths[i]
	}

	buff := &bytes.Buffer{}
	err = excel.Write(buff)
	if err != nil {
		return nil, "", err
	}
	return buff.Bytes(), "xlsx", nil
}

func dumpCSV(buffer []Line) ([]byte, string, error) {
	buff := &bytes.Buffer{}
	for _, line := range buffer {
		row := strings.Join(line, ";") + "\n"
		buff.Write([]byte(row))
	}
	return buff.Bytes(), "csv", nil
}

func writeFile(resultUnits []*ResultUnit, resultMap map[string]string, dump DumpFn, exportDir string) {
	err := os.MkdirAll(exportDir, 0770)
	if err != nil {
		log.Fatal(err)
	}
	for _, owner := range resultUnits {
		data, ext, err := dump(owner.Buffer)
		if err != nil {
			log.Printf("Failed to export result for %s: %s", owner.Name, err)
		}
		var outFile string
		if targetDir, ok := resultMap[owner.Name]; ok {
			outDir := path.Join(exportDir, targetDir)
			_ = os.MkdirAll(outDir, 0770)
			outFile = path.Join(outDir, owner.Name+"."+ext)
		} else {
			outDir := path.Join(exportDir, owner.Name)
			err = os.MkdirAll(outDir, 0770)
			if err != nil {
				log.Printf("Failed to export result for %s: %s", owner.Name, err)
			}
			outFile = path.Join(outDir, "13. Verifikatlista."+ext)
		}

		err = ioutil.WriteFile(outFile, data, 0664)
		if err != nil {
			log.Printf("Failed to export result for %s: %s", owner.Name, err)
		}
		if err == nil {
			log.Printf("Successfully exported result for %s to %s", owner.Name, outFile)
		}
	}
}
