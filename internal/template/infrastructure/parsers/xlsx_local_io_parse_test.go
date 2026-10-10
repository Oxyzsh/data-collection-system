package parsers

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseLocalIOTemplate(t *testing.T) {
	f := excelize.NewFile()
	sheet := "测点"
	index, err := f.NewSheet(sheet)
	if err != nil {
		t.Fatal(err)
	}
	f.SetActiveSheet(index)
	rows := [][]interface{}{
		{"ID", "类型", "中文名称", "英文名称", "读写属性", "通道名称", "最终数据类型", "是否启用", "备注"},
		{1, "POINT", "DI1", "DI1", "r", "DI1", "BOOL", 1, "数字输入1"},
		{2, "POINT", "DO1", "DO1", "rw", "DO1", "BOOL", 1, "数字输出1"},
	}
	for i, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	info := "模板信息"
	if _, err := f.NewSheet(info); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetRow(info, "A1", &[]interface{}{"模板名称", "本地IO测试"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "local_io.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseTemplateFileWithType(path, "local_io")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TemplateType != "local_io" || len(parsed.Points) != 2 {
		t.Fatalf("parsed = %#v", parsed)
	}
	if parsed.Points[0].LocalIO == nil || parsed.Points[0].LocalIO.Channel != "DI1" {
		t.Fatalf("first point = %#v", parsed.Points[0])
	}
	if parsed.Points[1].Access != "rw" || parsed.Points[1].DataType != "BOOL" {
		t.Fatalf("second point = %#v", parsed.Points[1])
	}
}
