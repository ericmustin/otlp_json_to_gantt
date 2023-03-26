package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type Span struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Start    int64  `json:"start"`
	End      int64  `json:"end"`
	Status   string `json:"status"`
	ParentID string `json:"parentSpanID"`
	Service  string `json:"serviceName"`
}

type SpanTemplate struct {
	Name      string
	Active    bool
	Crit      bool
	StartUnix int64
	Duration  time.Duration
}

type ServiceSection struct {
	Name    string
	Spans   []SpanTemplate
	Section string
}

func jsonToTemplateData(jsonData []byte) ([]ServiceSection, error) {
	spans := make(map[string]Span)
	err := json.Unmarshal(jsonData, &spans)
	if err != nil {
		return nil, err
	}

	var rootSpanID string
	for _, span := range spans {
		if span.ParentID == "" {
			rootSpanID = span.Name
			break
		}
	}

	spanTemplates := make(map[string]SpanTemplate)
	for _, span := range spans {
		duration := time.Duration(span.End-span.Start) * time.Millisecond
		template := SpanTemplate{
			Name:      span.Name,
			Active:    isActive(span.Kind),
			Crit:      isCrit(span.Status),
			StartUnix: span.Start / int64(time.Millisecond),
			Duration:  duration,
		}
		spanTemplates[span.Name] = template
	}

	var sections []ServiceSection
	var currentSection ServiceSection

	addSpanToSection := func(span Span) {
		if currentSection.Name == "" {
			currentSection.Name = span.Service
			currentSection.Section = fmt.Sprintf("section %s", span.Service)
			currentSection.Spans = []SpanTemplate{}
		} else if currentSection.Name != span.Service {
			sections = append(sections, currentSection)
			currentSection = ServiceSection{
				Name:    span.Service,
				Section: fmt.Sprintf("section %s", span.Service),
				Spans:   []SpanTemplate{},
			}
		}

		spanTemplate := spanTemplates[span.Name]
		currentSection.Spans = append(currentSection.Spans, spanTemplate)

		if span.Name == rootSpanID {
			sections = append(sections, currentSection)
		}
	}

	for _, span := range spans {
		addSpanToSection(span)
	}

	return sections, nil
}

func isActive(kind string) bool {
	switch kind {
	case "CLIENT", "SERVER", "PRODUCER", "CONSUMER":
		return true
	default:
		return false
	}
}

func isCrit(status string) bool {
	if strings.ToLower(status) == "error" {
		return true
	}
	return false
}

const tmplStr = `gantt
dateFormat x
axisFormat %X:%L

{{range .}}
{{.Section}}
{{range .Spans}}
{{.Name}} :{{.Name}} {{if .Active}}active{{else}}done{{end}}{{if .Crit}}, crit{{end}}, {{.StartUnix}}, {{.Duration.Milliseconds}}ms
{{end}}
{{end}}
`

// func generateGantt(inputPath string) error {
// 	jsonData, err := ioutil.ReadFile(inputPath)
// 	if err != nil {
// 		return err
// 	}

// 	funcMap := template.FuncMap{
// 		"isActive": isActive,
// 		"isCrit":   isCrit,
// 	}
// 	tmpl, err := template.New("gantt").Funcs(funcMap).Parse(tmplStr)
// 	if err != nil {
// 		return err
// 	}

// 	sections, err := jsonToTemplateData(jsonData)
// 	if err != nil {
// 		return err
// 	}

// 	outputPath := fmt.Sprintf("output/%s.md", strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)))
// 	outputFile, err := os.Create(outputPath)
// 	if err != nil {
// 		return err
// 	}
// 	defer outputFile.Close()

// 	if err := tmpl.Execute(outputFile, sections); err != nil {
// 		return err
// 	}

// 	return nil
// }

func generateMarkdown(inputFilePath string, outputDirPath string) error {
	inputFile, err := os.Open(inputFilePath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	jsonData, err := ioutil.ReadAll(inputFile)
	if err != nil {
		return err
	}

	sections, err := jsonToTemplateData(jsonData)
	if err != nil {
		return err
	}

	outputFileName := filepath.Base(inputFilePath) + ".md"
	outputFilePath := filepath.Join(outputDirPath, outputFileName)
	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	funcMap := template.FuncMap{
		"isActive": isActive,
		"isCrit":   isCrit,
	}
	tmpl, err := template.New("gantt").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return err
	}

	err = tmpl.Execute(outputFile, sections)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	inputDirPath := "/input"
	outputDirPath := "/output"

	err := filepath.Walk(inputDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			err = generateMarkdown(path, outputDirPath)
			if err != nil {
				log.Printf("Failed to generate markdown for file %s: %v", path, err)
			} else {
				log.Printf("Successfully generated markdown for file %s", path)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk input directory: %v", err)
	}
}