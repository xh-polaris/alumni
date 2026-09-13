package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
)

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("生成测试图片失败: %v", err)
	}
	return buffer.Bytes()
}

func uploadServiceFixture(t *testing.T) (*UploadService, string) {
	t.Helper()
	dir := t.TempDir()
	return &UploadService{Config: &config.Config{
		Upload: config.Upload{Dir: dir, PublicBaseURL: "http://localhost:8888", MaxBytes: 1 << 20},
	}}, dir
}

func TestUploadSaveWritesFileAndReturnsURL(t *testing.T) {
	service, dir := uploadServiceFixture(t)
	data := pngBytes(t, 8, 8)

	result, err := service.Save(context.Background(), "avatar", data)
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if !strings.HasPrefix(result.URL, "http://localhost:8888/files/") {
		t.Fatalf("URL 前缀不正确: %s", result.URL)
	}
	if !strings.HasSuffix(result.Key, ".png") {
		t.Fatalf("扩展名应由内容嗅探决定，实际 %s", result.Key)
	}
	if !strings.Contains(result.Key, "/avatar/") {
		t.Fatalf("scope 应体现在存储路径上（月份/scope/uuid.ext），实际 %s", result.Key)
	}
	if result.Size != int64(len(data)) || result.ContentType != "image/png" {
		t.Fatalf("返回信息不正确: %+v", result)
	}

	written, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(result.Key)))
	if err != nil {
		t.Fatalf("文件未落盘: %v", err)
	}
	if !bytes.Equal(written, data) {
		t.Fatalf("落盘内容与上传内容不一致")
	}
}

func TestUploadResolveBlocksTraversal(t *testing.T) {
	service, dir := uploadServiceFixture(t)
	if err := os.MkdirAll(filepath.Join(dir, "202601"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "202601", "a.png")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, ok := service.Resolve("/202601/a.png")
	if !ok || resolved != target {
		t.Fatalf("正常路径应解析成功: %s %v", resolved, ok)
	}
	for _, evil := range []string{"../etc/config.yaml", "/../../etc/config.yaml", "a/../../../secret"} {
		if _, ok := service.Resolve(evil); ok {
			t.Fatalf("目录穿越应被拒绝: %s", evil)
		}
	}
	if _, ok := service.Resolve("/"); ok {
		t.Fatalf("根路径应被拒绝")
	}
}

func TestUploadRejectsNonImageAndOversize(t *testing.T) {
	service, _ := uploadServiceFixture(t)

	if _, err := service.Save(context.Background(), "", []byte{}); err == nil {
		t.Fatalf("空文件应被拒绝")
	}
	// 伪装成图片的文本：内容嗅探应识别为 text/plain 并拒绝
	if _, err := service.Save(context.Background(), "", []byte("hello, not an image")); err == nil {
		t.Fatalf("非图片应被拒绝")
	}

	service.Config.Upload.MaxBytes = 64
	if _, err := service.Save(context.Background(), "", pngBytes(t, 64, 64)); err == nil {
		t.Fatalf("超过上限应被拒绝")
	}
}

func TestUploadEscapesUserProvidedScope(t *testing.T) {
	service, dir := uploadServiceFixture(t)
	result, err := service.Save(context.Background(), "../../evil scope!", pngBytes(t, 4, 4))
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if strings.Contains(result.Key, "..") || strings.Contains(result.Key, " ") || strings.Contains(result.Key, "!") {
		t.Fatalf("scope 未被清洗: %s", result.Key)
	}
	// 只应产生「月份目录 / 清洗后的 scope 目录 / 文件」三层
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(result.Key))); err != nil {
		t.Fatalf("文件应落在存储根目录内: %v", err)
	}
}

func TestUploadFallsBackToDefaultDirAndRelativeURL(t *testing.T) {
	service := &UploadService{Config: &config.Config{}}
	// 默认目录是相对路径，测试里切换工作目录以免污染仓库
	dir := t.TempDir()
	previous, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(previous) }()

	result, err := service.Save(context.Background(), "chapter", pngBytes(t, 4, 4))
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	if !strings.HasPrefix(result.URL, "/files/") {
		t.Fatalf("未配置 PublicBaseURL 时应返回相对路径，实际 %s", result.URL)
	}
	if _, err := os.Stat(filepath.Join(dir, "output", "upload", filepath.FromSlash(result.Key))); err != nil {
		t.Fatalf("应落到默认目录 ./output/upload: %v", err)
	}
}
