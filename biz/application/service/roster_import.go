package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/roster"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util/log"
)

type RosterImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}
type RosterImportResult struct {
	Created    int                 `json:"created"`
	Updated    int                 `json:"updated"`
	Duplicates int                 `json:"duplicates"`
	Failed     int                 `json:"failed"`
	Errors     []RosterImportError `json:"errors"`
}

func (s *AdminService) ImportRoster(ctx context.Context, filename string, data []byte) (*RosterImportResult, error) {
	session, err := s.GetSession(ctx)
	if err != nil {
		return nil, err
	}
	if session.AdminRole != user.AdminSuper {
		return nil, ErrAdminForbidden
	}
	rows, err := parseRosterRows(filename, data)
	if err != nil {
		return nil, ErrAdminBadRequest
	}
	result := &RosterImportResult{Errors: make([]RosterImportError, 0)}
	seen := map[string]bool{}
	for index, row := range rows {
		rowNumber := index + 2
		if len(row) < 3 {
			result.Failed++
			result.Errors = append(result.Errors, RosterImportError{rowNumber, "缺少姓名、毕业年份或出生日期"})
			continue
		}
		name := strings.TrimSpace(row[0])
		year, yearErr := strconv.ParseInt(strings.TrimSpace(row[1]), 10, 64)
		birthDate := normalizeSpreadsheetDate(strings.TrimSpace(row[2]))
		if name == "" || yearErr != nil || year < 1900 {
			result.Failed++
			result.Errors = append(result.Errors, RosterImportError{rowNumber, "姓名或毕业年份格式不正确"})
			continue
		}
		if _, dateErr := parseBirthDate(birthDate); dateErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, RosterImportError{rowNumber, "出生日期必须为 YYYY-MM-DD"})
			continue
		}
		normalized := normalizeRosterName(name)
		key := fmt.Sprintf("%s|%d|%s", normalized, year, birthDate)
		if seen[key] {
			result.Duplicates++
			result.Errors = append(result.Errors, RosterImportError{rowNumber, "文件内存在重复记录"})
			continue
		}
		seen[key] = true
		created, saveErr := s.RosterMapper.Upsert(ctx, roster.Entry{Name: name, NormalizedName: normalized, GraduationYear: year, BirthDate: birthDate})
		if saveErr != nil {
			result.Failed++
			result.Errors = append(result.Errors, RosterImportError{rowNumber, "保存失败"})
		} else if created {
			result.Created++
		} else {
			result.Updated++
		}
	}
	// metric=roster_import 供后台监控名册导入失败数与重复数。
	log.CtxInfo(ctx, "metric=roster_import file=%s created=%d updated=%d duplicates=%d failed=%d",
		filename, result.Created, result.Updated, result.Duplicates, result.Failed)
	return result, nil
}

func parseRosterRows(filename string, data []byte) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})))
		reader.FieldsPerRecord = -1
		rows, err := reader.ReadAll()
		if err != nil || len(rows) == 0 {
			return nil, ErrAdminBadRequest
		}
		return validateRosterHeader(rows)
	case ".xlsx":
		rows, err := readSimpleXLSX(data)
		if err != nil || len(rows) == 0 {
			return nil, ErrAdminBadRequest
		}
		return validateRosterHeader(rows)
	default:
		return nil, ErrAdminBadRequest
	}
}

func validateRosterHeader(rows [][]string) ([][]string, error) {
	if len(rows[0]) < 3 || strings.TrimSpace(rows[0][0]) != "姓名" || strings.TrimSpace(rows[0][1]) != "毕业年份" || strings.TrimSpace(rows[0][2]) != "出生日期" {
		return nil, ErrAdminBadRequest
	}
	return rows[1:], nil
}

func normalizeSpreadsheetDate(value string) string {
	if number, err := strconv.ParseFloat(value, 64); err == nil && number > 1000 {
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(number*24) * time.Hour).Format("2006-01-02")
	}
	value = strings.ReplaceAll(value, "/", "-")
	return value
}

type sharedStringsXML struct {
	Items []struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}
type worksheetXML struct {
	Rows []struct {
		Cells []struct {
			Ref    string `xml:"r,attr"`
			Type   string `xml:"t,attr"`
			Value  string `xml:"v"`
			Inline string `xml:"is>t"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func readSimpleXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := map[string]*zip.File{}
	for _, file := range zr.File {
		files[file.Name] = file
	}
	shared := []string{}
	if file := files["xl/sharedStrings.xml"]; file != nil {
		content, readErr := readZipFile(file)
		if readErr != nil {
			return nil, readErr
		}
		var parsed sharedStringsXML
		if xml.Unmarshal(content, &parsed) != nil {
			return nil, ErrAdminBadRequest
		}
		for _, item := range parsed.Items {
			text := item.Text
			for _, run := range item.Runs {
				text += run.Text
			}
			shared = append(shared, text)
		}
	}
	file := files["xl/worksheets/sheet1.xml"]
	if file == nil {
		return nil, ErrAdminBadRequest
	}
	content, err := readZipFile(file)
	if err != nil {
		return nil, err
	}
	var sheet worksheetXML
	if err = xml.Unmarshal(content, &sheet); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := []string{"", "", ""}
		for _, cell := range row.Cells {
			if len(cell.Ref) == 0 {
				continue
			}
			column := int(cell.Ref[0] - 'A')
			if column < 0 || column > 2 {
				continue
			}
			value := cell.Value
			if cell.Type == "inlineStr" {
				value = cell.Inline
			} else if cell.Type == "s" {
				index, convErr := strconv.Atoi(value)
				if convErr == nil && index >= 0 && index < len(shared) {
					value = shared[index]
				}
			}
			values[column] = value
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
