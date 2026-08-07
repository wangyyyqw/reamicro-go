package main

// 书籍处理：扫描 .epub，自动解包到缓存目录，以解包后的 epub 结构同步。
//
// 安卓端同步导入时直接对书目录执行 Opf.obtain()，必须是解包后的 epub 结构
// （META-INF/container.xml、content.opf 等），而非单个 .epub 文件。

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const unpackDir = ".reamicro_pc/unpacked"

type BookFile struct {
	RelPath string // 相对书目录
	Size    int64
	Mtime   int64
}

type Book struct {
	UUID    string
	Title   string
	Root    string // 解包后的书目录
	Files   []BookFile
	OrigSize int64
}

// stableUUID 生成基于种子的稳定 UUID v3
func stableUUID(seed string) string {
	h := md5.Sum([]byte(seed))
	h[6] = (h[6] & 0x0f) | 0x30
	h[8] = (h[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(h[:])
	return raw[0:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:32]
}

// unpackCacheDir 返回解包缓存目录
func unpackCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", unpackDir)
	}
	return filepath.Join(home, unpackDir)
}

// unpackEpub 把 epub 解包到缓存目录，返回解包根目录
func unpackEpub(epubPath string) (string, error) {
	// 基于文件内容哈希做缓存目录名，避免重复解包
	h := md5.New()
	f, err := os.Open(epubPath)
	if err != nil {
		return "", err
	}
	io.Copy(h, f)
	f.Close()
	sum := hex.EncodeToString(h.Sum(nil))

	base := unpackCacheDir()
	out := filepath.Join(base, sum)
	container := filepath.Join(out, "META-INF", "container.xml")
	if _, err := os.Stat(container); err == nil {
		return out, nil // 已解包
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	zr, err := zip.OpenReader(epubPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		// 安全解包：拒绝路径穿越
		name := strings.TrimPrefix(filepath.ToSlash(zf.Name), "/")
		if name == "" || strings.Contains(name, "../") {
			continue
		}
		if zf.FileInfo().IsDir() {
			continue
		}
		target := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		w, err := os.Create(target)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(w, rc)
		w.Close()
		rc.Close()
	}
	// 删除 __MACOSX
	os.RemoveAll(filepath.Join(out, "__MACOSX"))
	if _, err := os.Stat(container); err != nil {
		return "", os.ErrNotExist
	}
	return out, nil
}

// listFiles 递归列出目录下所有文件（相对目录的路径）
func listFiles(root string) []BookFile {
	var files []BookFile
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == ".DS_Store" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, BookFile{
			RelPath: filepath.ToSlash(rel),
			Size:    info.Size(),
			Mtime:   info.ModTime().UnixMilli(),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files
}

// scanBooks 扫描目录下的 .epub，解包后返回书列表
func scanBooks(dir string) ([]Book, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var books []Book
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".epub") {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		root, err := unpackEpub(full)
		if err != nil {
			continue // 解包失败跳过
		}
		files := listFiles(root)
		if len(files) == 0 {
			continue
		}
		books = append(books, Book{
			UUID:     stableUUID("epub:" + dir + ":" + name),
			Title:    strings.TrimSuffix(name, filepath.Ext(name)),
			Root:     root,
			Files:    files,
			OrigSize: info.Size(),
		})
	}
	sort.Slice(books, func(i, j int) bool { return books[i].Title < books[j].Title })
	return books, nil
}

// opfItem / opfMeta 用于解析 content.opf 的 manifest 与 meta
type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	Properties string `xml:"properties,attr"`
	MediaType  string `xml:"media-type,attr"`
}
type opfMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}
type opfManifest struct {
	Items []opfItem `xml:"item"`
}
type opfMetadata struct {
	Metas []opfMeta `xml:"meta"`
}
type opfPackage struct {
	Metadata opfMetadata `xml:"metadata"`
	Manifest opfManifest `xml:"manifest"`
}
type opfRoot struct {
	Package opfPackage `xml:"package"`
}
type containerRootfile struct {
	FullPath string `xml:"full-path,attr"`
}
type containerRootfiles struct {
	Rootfile containerRootfile `xml:"rootfile"`
}
type containerRoot struct {
	Rootfiles containerRootfiles `xml:"rootfiles"`
}

// findOpf 从解包目录找到 content.opf 路径
func findOpf(root string) string {
	container := filepath.Join(root, "META-INF", "container.xml")
	data, err := os.ReadFile(container)
	if err == nil {
		var c containerRoot
		if xml.Unmarshal(data, &c) == nil && c.Rootfiles.Rootfile.FullPath != "" {
			p := filepath.Join(root, filepath.FromSlash(c.Rootfiles.Rootfile.FullPath))
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// 兜底：找任意 .opf
	var found string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".opf") && found == "" {
			found = path
		}
		return nil
	})
	return found
}

// extractCover 解析 opf，返回封面图片相对书根的路径（找不到返回空）
// 使用 xml.Decoder 忽略 namespace，只按 local name 匹配（epub opf 常带默认 namespace）
func extractCover(root string) string {
	opfPath := findOpf(root)
	if opfPath == "" {
		return ""
	}
	data, err := os.ReadFile(opfPath)
	if err != nil {
		return ""
	}
	opfDir := filepath.Dir(opfPath)

	var coverID string
	var coverItem *opfItem
	var opf opfRoot
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch el := tok.(type) {
		case xml.StartElement:
			local := el.Name.Local
			if local == "meta" {
				var name, content string
				for _, a := range el.Attr {
					switch a.Name.Local {
					case "name":
						name = a.Value
					case "content":
						content = a.Value
					}
				}
				if strings.EqualFold(name, "cover") {
					coverID = content
				}
			} else if local == "item" {
				it := opfItem{}
				for _, a := range el.Attr {
					switch a.Name.Local {
					case "id":
						it.ID = a.Value
					case "href":
						it.Href = a.Value
					case "properties":
						it.Properties = a.Value
					case "media-type":
						it.MediaType = a.Value
					}
				}
				opf.Package.Manifest.Items = append(opf.Package.Manifest.Items, it)
			}
		}
	}

	// 1) 优先 properties="cover-image"
	for i := range opf.Package.Manifest.Items {
		it := &opf.Package.Manifest.Items[i]
		if strings.Contains(strings.ToLower(it.Properties), "cover-image") {
			coverItem = it
			break
		}
	}
	// 2) meta cover content 作为 item id 或直接作为文件名
	if coverItem == nil && coverID != "" {
		for i := range opf.Package.Manifest.Items {
			it := &opf.Package.Manifest.Items[i]
			if it.ID == coverID {
				coverItem = it
				break
			}
		}
		// 某些 epub 的 cover content 直接是文件名（如 "cover.png"）
		if coverItem == nil {
			for i := range opf.Package.Manifest.Items {
				it := &opf.Package.Manifest.Items[i]
				if filepath.Base(filepath.ToSlash(it.Href)) == coverID {
					coverItem = it
					break
				}
			}
		}
	}
	// 3) 兜底：manifest 中第一个图片 item
	if coverItem == nil {
		imgExt := []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"}
		for i := range opf.Package.Manifest.Items {
			it := &opf.Package.Manifest.Items[i]
			lower := strings.ToLower(it.Href)
			for _, ext := range imgExt {
				if strings.HasSuffix(lower, ext) {
					coverItem = it
					break
				}
			}
			if coverItem != nil {
				break
			}
		}
	}
	if coverItem == nil || coverItem.Href == "" {
		return ""
	}
	// 解析相对 opf 目录的路径
	href := strings.TrimPrefix(filepath.ToSlash(coverItem.Href), "/")
	// 去掉查询/锚点
	if i := strings.IndexAny(href, "?#"); i >= 0 {
		href = href[:i]
	}
	full := filepath.Join(opfDir, filepath.FromSlash(href))
	rootNorm := filepath.Clean(root)
	fullNorm := filepath.Clean(full)
	if !strings.HasPrefix(fullNorm, rootNorm+string(os.PathSeparator)) {
		return ""
	}
	if _, err := os.Stat(fullNorm); err != nil {
		return ""
	}
	rel, err := filepath.Rel(rootNorm, fullNorm)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// buildManifest 构建同步清单（书籍为解包后的文件结构）
func buildManifest(dir, deviceID string, userID int64) (*SyncManifest, error) {
	books, err := scanBooks(dir)
	if err != nil {
		return nil, err
	}
	now := nowMs()
	var entries []BookSyncEntry
	for _, b := range books {
		var total int64
		for _, f := range b.Files {
			total += f.Size
		}
		snapshot := BookSnapshot{
			UUID:       b.UUID,
			Title:      b.Title,
			Cover:      extractCover(b.Root),
			Size:       total,
			Progress:   0.01,
			CfiVersion: 2,
			Created:    now,
			Updated:    now,
		}
		var files []FileSnapshot
		for _, f := range b.Files {
			files = append(files, FileSnapshot{
				RelativePath: f.RelPath,
				Size:         f.Size,
				Mtime:        f.Mtime,
			})
		}
		entries = append(entries, BookSyncEntry{
			UUID:              b.UUID,
			MetaUpdatedAt:     now,
			ProgressUpdatedAt: now,
			Book:              snapshot,
			Marks:             []MarkSnapshot{},
			Files:             files,
		})
	}
	return &SyncManifest{
		UserID:      userID,
		DeviceID:    deviceID,
		AppVersion:  4,
		GeneratedAt: now,
		Books:       entries,
	}, nil
}

// findBook 返回 uuid 对应的书
func findBook(dir, uuid string) (Book, bool) {
	books, err := scanBooks(dir)
	if err != nil {
		return Book{}, false
	}
	for _, b := range books {
		if b.UUID == uuid {
			return b, true
		}
	}
	return Book{}, false
}

// readBookFile 读取书中某文件内容（路径穿越防护）
func readBookFile(dir, uuid, relativePath string) ([]byte, error) {
	book, ok := findBook(dir, uuid)
	if !ok {
		return nil, os.ErrNotExist
	}
	rel := filepath.ToSlash(relativePath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.Contains(rel, "../") {
		return nil, os.ErrNotExist
	}
	full := filepath.Join(book.Root, filepath.FromSlash(rel))
	rootNorm := filepath.Clean(book.Root)
	fullNorm := filepath.Clean(full)
	if !strings.HasPrefix(fullNorm, rootNorm+string(os.PathSeparator)) {
		return nil, os.ErrNotExist
	}
	return os.ReadFile(fullNorm)
}

// cleanupUnpackCache 删除解包缓存目录（同步结束后调用，下次会自动重新解包）
func cleanupUnpackCache() {
	dir := unpackCacheDir()
	if err := os.RemoveAll(dir); err == nil {
		emitLog("已清理解包缓存")
	} else {
		emitLog("清理解包缓存失败: " + err.Error())
	}
}
