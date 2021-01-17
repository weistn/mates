package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

const (
	columnUnknown = iota
	columnX
	columnY
	columnLabel
	columnColor
)

type chartData struct {
	columns   []*chartDataColumn
	donut     bool
	legend    bool
	legendpos string
	text      string
	lines     bool
	bars      bool
	points    bool
	stack     bool
	fill      bool
	ymin      string
	ymax      string
}

type chartDataColumn struct {
	kind int
	name string
	data []interface{}
}

func extractData(table *DocumentNode) *chartData {
	result := &chartData{}
	result.donut = table.HasClass("donut")
	result.legend = table.HasClass("legend")
	result.lines = table.HasClass("lines")
	result.bars = table.HasClass("bars")
	result.points = table.HasClass("points")
	result.stack = table.HasClass("stack")
	result.fill = table.HasClass("fill")
	result.legendpos = table.Attributes["legend-position"]
	result.text = table.Attributes["text"]
	result.ymin = table.Attributes["ymin"]
	result.ymax = table.Attributes["ymax"]

	cols, _ := table.Columns()
	for _, c := range cols {
		cdata := &chartDataColumn{}
		t := strings.TrimSpace(c.PlainText())
		cdata.name = t
		if t == "X" {
			cdata.kind = columnX
		} else if t[0] == 'Y' {
			cdata.kind = columnY
			cdata.name = t[1:]
		} else if t == "Label" {
			cdata.kind = columnLabel
		} else if t == "Color" {
			cdata.kind = columnColor
		}
		result.columns = append(result.columns, cdata)
	}

	rows := table.Rows()
	for _, row := range rows {
		cells := row.DocumentNodes("tbody-cell")
		for col, cell := range cells {
			if col >= len(result.columns) {
				break
			}
			t := strings.TrimSpace(cell.PlainText())
			c := result.columns[col]
			var v interface{}
			switch c.kind {
			case columnX, columnY:
				if t != "" {
					var err error
					v, err = strconv.ParseFloat(t, 64)
					if err != nil {
						log.Printf("Malformed value in chart data: %v", t)
						v = float64(0)
					}
				}
			case columnLabel:
				v = t
			case columnColor:
				// TODO: Syntax check
				v = t
			case columnUnknown:
				// Do nothing by intention
				log.Printf("Unknown column: %v", c.name)
			}
			c.data = append(c.data, v)
		}
	}

	return result
}

func generatePieChart(table *DocumentNode) (result string) {
	data := extractData(table)
	result = `<div id="` + table.ForceID() + `" class="piechart ` + table.Class() + `"`
	if table.Style() != "" {
		result += ` style="` + table.Style() + `"`
	}
	result += `></div>`
	result += `<script type="text/javascript"> $(function () { var data = [`
	// Find X column and Label column
	var colY *chartDataColumn
	var colLabel *chartDataColumn
	for _, c := range data.columns {
		if c.kind == columnY {
			colY = c
		} else if c.kind == columnLabel {
			colLabel = c
		}
	}
	if colY == nil {
		log.Printf("Missing Y column in pie chart data")
		return ""
	}
	for i, v := range colY.data {
		if i > 0 {
			result += ","
		}
		result += `{data:`
		result += strconv.FormatFloat(v.(float64), 'f', -1, 64)
		if colLabel != nil && i < len(colLabel.data) {
			result += `, label:` + strconv.Quote(colLabel.data[i].(string))
		}
		result += "}"
	}
	result += `]; $.plot($(` + strconv.Quote("#"+table.ForceID()) + `), data, { series: { pie: { show: true`
	if data.donut {
		result += `, innerRadius: 0.5`
	}
	if data.text == "inside" {
		result += ", radius: 1"
	}
	result += `, label: {`
	if data.text == "inside" {
		result += `show: true, radius:3/4, formatter: function(label, series) { return '<div style="text-align:center;padding:2px;color:white;">'+label+'<br/>'+Math.round(series.percent)+'%</div>';}, background: { opacity: 0.5 }`
	} else if data.text == "outside" {
		result += "show: true"
	} else {
		result += "show: false"
	}
	result += ` } } }, legend: { show: `
	if data.legend || data.legendpos != "" {
		result += "true"
	} else {
		result += "false"
	}
	if data.legendpos != "" {
		result += ", position: " + strconv.Quote(data.legendpos)
	}
	result += `} }); }); </script>`
	return
}

func generateLineBarChart(table *DocumentNode) (result string) {
	data := extractData(table)
	result = `<div id="` + table.ForceID() + `" class="chart ` + table.Class() + `"`
	if table.Style() != "" {
		result += ` style="` + table.Style() + `"`
	}
	result += `></div>`
	result += `<script type="text/javascript"> $(function () { var data = [`
	// Find X column and Label column
	var colX *chartDataColumn
	var colLabel *chartDataColumn
	for _, c := range data.columns {
		if c.kind == columnX {
			colX = c
		} else if c.kind == columnLabel {
			colLabel = c
		}
	}
	if colX == nil && colLabel == nil {
		log.Printf("Missing X or Label column in chart data")
		return ""
	}
	// Iterate over all data series
	seriesCount := 0
	for _, colY := range data.columns {
		if colY.kind != columnY {
			continue
		}
		if seriesCount != 0 {
			result += ","
		}
		seriesCount++
		result += "{label:" + strconv.Quote(colY.name) + ", data: ["
		rows := 0
		if colX != nil {
			for j, x := range colX.data {
				var y interface{}
				if j >= len(colY.data) {
					log.Printf("Too few values in the y-axis")
				} else {
					y = colY.data[j]
				}
				if y == nil {
					continue
				}
				if rows > 0 {
					result += ","
				}
				result += `[`
				result += strconv.FormatFloat(x.(float64), 'f', -1, 64)
				result += ","
				result += strconv.FormatFloat(y.(float64), 'f', -1, 64)
				result += "]"
				rows++
			}
		} else {
			for j := 0; j < len(colY.data); j++ {
				var y interface{}
				if j >= len(colY.data) {
					log.Printf("Too few values in the y-axis")
				} else {
					y = colY.data[j]
				}
				if y == nil {
					continue
				}
				if rows > 0 {
					result += ","
				}
				result += fmt.Sprintf("[%v,%v]", j+1, strconv.FormatFloat(y.(float64), 'f', -1, 64))
				rows++
			}
		}
		result += "]}"
	}

	result += `]; $.plot($(` + strconv.Quote("#"+table.ForceID()) + `), data, { `
	if colLabel != nil {
		result += "xaxis:{ ticks: ["
		for j, l := range colLabel.data {
			if j > 0 {
				result += ","
			}
			if colX == nil {
				result += fmt.Sprintf("[%v, %v]", j+1, strconv.Quote(l.(string)))
			} else {
				result += fmt.Sprintf("[%v, %v]", colX.data[j], strconv.Quote(l.(string)))
			}
		}
		result += "]}, "
	}
	if data.ymin != "" || data.ymax != "" {
		result += "yaxis: { "
		sep := false
		if data.ymin != "" {
			result += "min: " + strconv.Quote(data.ymin)
			sep = true
		}
		if data.ymax != "" {
			if sep {
				result += ","
			}
			result += "max: " + strconv.Quote(data.ymax)
		}
		result += "}, "
	}

	result += `series: { `
	sep := false
	if data.lines {
		result += "lines: { show: true"
		if data.fill {
			result += ", fill: true"
		}
		result += " }"
		sep = true
	}
	if data.points {
		if sep {
			result += ","
		}
		result += "points: {show: true}"
	}
	if data.bars {
		if sep {
			result += ","
		}
		result += `bars: {show: true, barWidth: 0.8, align: "center"}`
	}
	if data.stack {
		result += ",stack: true"
	}
	result += `}, legend: { show: `
	if data.legend || data.legendpos != "" {
		result += "true"
	} else {
		result += "false"
	}
	if data.legendpos != "" {
		result += ", position: " + strconv.Quote(data.legendpos)
	}
	result += `} }); }); </script>`
	return
}
