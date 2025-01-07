// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

const (
	formatTable = "table"
	formatJSON  = "json"

	templateTable = "{{table .}}\n"
	templateJSON  = "{{json .}}\n"

	defaultSliceSeparator   = ", "
	defaultMapSeparator     = ", "
	defaultMapItemSeparator = ":"
)

var tableHeaderBlue = tablewriter.Colors{tablewriter.FgHiBlueColor}

// RenderOptions allows passing optional flags for altering rendered table
type RenderOptions struct {
	// OptionalColumns may contain column header names which can be hidden
	OptionalColumns map[string]bool

	// Border specified whether to render table border
	Border bool

	// Custom, per column alignment
	ColumnAlignment []int

	// Color will use coloring characters while printing table
	Color bool

	// PrintRawTime will print time as int64 unix nanos
	PrintRawTime bool
	// PrintDateTime will print both date & time
	PrintDateTime bool

	// DefaultTemplate (if specified) will be used to render data when not --format flag is given
	DefaultTemplate string
}

// Render is an entry point for presentation layer. It uses --format flag to determine output format.
func Render(c *cli.Context, data interface{}, opts RenderOptions) (err error) {
	defer func() {
		if err != nil {
			fmt.Errorf("failed to render: %w", err)
		}
	}()

	// For now always output to stdout
	w := getDeps(c).Output()

	template := opts.DefaultTemplate

	// Handle template shorthands
	switch format := c.String(FlagFormat); format {
	case formatJSON:
		template = templateJSON
	case formatTable:
		template = templateTable
	default:
		if len(format) > 0 {
			switch kind := reflect.ValueOf(data).Kind(); kind {
			case reflect.Slice:
				template = fmt.Sprintf("{{range .}}%s\n{{end}}", format)
			case reflect.Struct:
				template = fmt.Sprintf("%s\n", format)
			default:
				return fmt.Errorf("data must be a struct or a slice, provided: %s", kind)
			}
		}
	}

	return RenderTemplate(w, data, template, opts)
}

// RenderTemplate uses golang text/template format to render data with user provided template
func RenderTemplate(w io.Writer, data interface{}, tmpl string, opts RenderOptions) error {
	fns := map[string]interface{}{
		"table": func(data interface{}) (string, error) {
			sb := &strings.Builder{}
			if err := RenderTable(sb, data, opts); err != nil {
				return "", err
			}
			return sb.String(), nil
		},
		"json": func(data interface{}) (string, error) {
			encoded, err := json.MarshalIndent(data, "", "  ")
			return string(encoded), err
		},
	}

	t, err := template.New("").Funcs(fns).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("invalid template %q: %w", tmpl, err)
	}

	return t.Execute(w, data)
}

// RenderTable is generic function for rendering a slice of structs as a table
func RenderTable(w io.Writer, data interface{}, opts RenderOptions) error {
	value := reflect.ValueOf(data)

	if value.Kind() == reflect.Ptr {
		// Nil pointer - nothing to render
		if value.IsNil() {
			return nil
		}

		// Drop the pointer and start over
		return RenderTable(w, value.Elem().Interface(), opts)
	}

	if value.Kind() != reflect.Slice {
		// If data is not a slice - wrap it with a slice of one element
		value = reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(data)), 0, 1)
		value = reflect.Append(value, reflect.ValueOf(data))
	}

	// We have a slice at this point
	slice := value

	// No elements - nothing to render
	if slice.Len() == 0 {
		return nil
	}

	firstElem := slice.Index(0)
	if firstElem.Kind() != reflect.Struct {
		return fmt.Errorf("table slice element must be a struct, provided: %s", firstElem.Kind())
	}

	table := tablewriter.NewWriter(w)
	table.SetBorder(opts.Border)
	table.SetColumnSeparator("|")
	table.SetHeaderLine(opts.Border)
	if opts.ColumnAlignment != nil {
		table.SetColumnAlignment(opts.ColumnAlignment)
	}

	for r := 0; r < slice.Len(); r++ {
		var row []string
		var headers []string
		var colors []tablewriter.Colors

		elem := slice.Index(r)
		for f := 0; f < elem.NumField(); f++ {
			tag := elem.Type().Field(f).Tag

			header := columnHeader(tag, opts)
			if header == "" {
				continue
			}
			if r == 0 {
				headers = append(headers, header)
				colors = append(colors, tableHeaderBlue)
			}

			row = append(row, formatValue(elem.Field(f).Interface(), opts, tag))
		}
		if r == 0 {
			table.SetHeader(headers)
			if opts.Color {
				table.SetHeaderColor(colors...)
			}
		}

		table.Append(row)
	}

	table.Render()

	return nil
}

func columnHeader(tag reflect.StructTag, opts RenderOptions) string {
	header, ok := tag.Lookup("header")
	if !ok {
		// No header tag - do not display
		return ""
	}

	if opts.OptionalColumns == nil {
		// No optional columns defined - display
		return header
	}

	include, optional := opts.OptionalColumns[header]
	if !optional {
		// Display if it is non-optional
		return header
	}

	if include {
		// Display if it is optional but included
		return header
	}

	// Do not display optional and excluded
	return ""
}

func formatValue(value interface{}, opts RenderOptions, tag reflect.StructTag) string {
	switch v := value.(type) {
	case time.Time:
		return formatTime(v, opts)
	case string:
		return formatString(v, tag)
	}

	switch v := reflect.ValueOf(value); v.Kind() {
	case reflect.Slice:
		return formatSlice(value, opts)
	case reflect.Map:
		return formatMap(value, opts)
	}

	return fmt.Sprintf("%v", value)
}

func formatSlice(value interface{}, opts RenderOptions) string {
	v := reflect.ValueOf(value)
	values := []string{}
	for i := 0; i < v.Len(); i++ {
		values = append(values, formatValue(v.Index(i).Interface(), opts, ""))
	}
	return strings.Join(values, defaultSliceSeparator)
}

func formatMap(value interface{}, opts RenderOptions) string {
	v := reflect.ValueOf(value)
	values := []string{}
	for _, key := range v.MapKeys() {
		values = append(values, formatValue(key, opts, "")+defaultMapItemSeparator+formatValue(v.MapIndex(key), opts, ""))
	}
	sort.Strings(values)
	return strings.Join(values, defaultMapSeparator)
}

func formatTime(t time.Time, opts RenderOptions) string {
	if opts.PrintRawTime {
		return strconv.FormatInt(t.Unix(), 10)
	}
	if opts.PrintDateTime {
		return t.Format(defaultDateTimeFormat)
	}
	return t.Format(defaultTimeFormat)
}

func formatString(str string, tag reflect.StructTag) string {
	if maxLengthStr, ok := tag.Lookup("maxLength"); ok {
		maxLength, _ := strconv.ParseInt(maxLengthStr, 10, 64)
		str = trimString(str, int(maxLength))
	}

	return str
}

func trimString(str string, maxLength int) string {
	if len(str) < maxLength {
		return str
	}

	items := strings.Split(str, "/")
	lastItem := items[len(items)-1]
	if len(lastItem) < maxLength {
		return ".../" + lastItem
	}

	return "..." + lastItem[len(lastItem)-maxLength:]
}
