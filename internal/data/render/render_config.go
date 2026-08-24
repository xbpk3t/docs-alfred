package datarender

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/xbpk3t/docs-alfred/internal/gh/goods"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
	"github.com/xbpk3t/docs-alfred/pkg/parser"
	"github.com/xbpk3t/docs-alfred/pkg/render"
)

type fileType string

const (
	fileTypeJSON fileType = "json"
	fileTypeYAML fileType = "yml"
)

// DomainRenderConfig holds configuration for rendering a single domain.
type DomainRenderConfig struct {
	Domain string
	Src    string
	OutDir string
	Format string // "json", "yaml", "json,yaml"
}

// DomainRenderResult holds the result of a domain render.
type DomainRenderResult struct {
	OutputFiles []string
}

// RunDomainRender renders a single domain's data into the specified output formats.
func RunDomainRender(cfg DomainRenderConfig) (*DomainRenderResult, error) {
	src, err := filepath.Abs(cfg.Src)
	if err != nil {
		return nil, fmt.Errorf("get absolute path: %w", err)
	}

	fi, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("stat source path %s: %w", src, err)
	}

	isSourceDir := fi.IsDir()
	formats := strings.Split(cfg.Format, ",")

	renderer, err := createRendererForDomain(cfg.Domain)
	if err != nil {
		return nil, err
	}

	var outputFiles []string

	// goods/books/ntl split by file into the frontend's `type`→`topics`
	// catalog. Gather the entries once (a single tree walk + parse) and just
	// re-marshal per format, so a `json,yaml` render doesn't re-read the data.
	var catalogEntries []catalogEntry
	if prefix, ok := catalogDomainPrefix(cfg.Domain); ok && isSourceDir {
		entries, err := collectCatalogEntries(src, prefix)
		if err != nil {
			return nil, fmt.Errorf("process %s dir: %w", cfg.Domain, err)
		}
		catalogEntries = entries
	}

	for _, f := range formats {
		f = strings.TrimSpace(f)
		ft := normalizeFormat(f)
		if ft == "" {
			return nil, fmt.Errorf("unsupported format %q", f)
		}

		proc := newDocProcessor(ft)
		proc.Dst = cfg.OutDir
		outputName := proc.getOutputFilename(src)

		if err := renderFormat(cfg.Domain, isSourceDir, src, ft, renderer, proc, outputName, catalogEntries); err != nil {
			return nil, err
		}

		outputFiles = append(outputFiles, filepath.Join(cfg.OutDir, outputName))
	}

	return &DomainRenderResult{OutputFiles: outputFiles}, nil
}

// renderFormat renders a single format for the domain: gh dirs walk the tree,
// grouped catalog domains emit the pre-gathered catalog, everything else is the
// generic single-file path.
func renderFormat(domain string, isSourceDir bool, src string, ft fileType, renderer render.Renderer, proc *docProcessor, outputName string, catalogEntries []catalogEntry) error {
	switch {
	case domain == "gh" && isSourceDir:
		if err := processGithubDirDomain(src, ft, proc); err != nil {
			return fmt.Errorf("process gh dir: %w", err)
		}
		return nil
	case catalogEntries != nil:
		content, err := marshalCatalog(catalogEntries, ft)
		if err != nil {
			return err
		}
		if err := proc.writeOutput(content, outputName); err != nil {
			return fmt.Errorf("write %s: %w", outputName, err)
		}
		return nil
	default:
		if err := proc.processFile(src, renderer); err != nil {
			return fmt.Errorf("process %s: %w", ft, err)
		}
		return nil
	}
}

// createRendererForDomain returns the appropriate renderer for a domain.
func createRendererForDomain(domain string) (render.Renderer, error) {
	var renderer render.Renderer
	switch domain {
	case "gh":
		renderer = ghindex.NewGithubYAMLRender("")
	case "goods":
		renderer = goods.NewGoodsYAMLRender()
	default:
		renderer = render.NewYAMLRenderer(domain, true)
	}

	parseMode, exists := serviceParseModeMap()[domain]
	if !exists {
		parseMode = render.ParseSingle
	}

	type parseModeRenderer interface {
		WithParseMode(mode render.ParseMode)
	}

	r, ok := renderer.(parseModeRenderer)
	if !ok {
		return nil, errors.New("renderer does not support parse mode configuration")
	}
	r.WithParseMode(parseMode)

	return renderer, nil
}

// processGithubDirDomain handles the gh domain's special directory-based rendering.
func processGithubDirDomain(src string, ft fileType, proc *docProcessor) error {
	allRepos, err := ghindex.LoadConfigReposFromDir(src)
	if err != nil {
		return err
	}

	result, err := yaml.Marshal(allRepos)
	if err != nil {
		return fmt.Errorf("marshal gh repos: %w", err)
	}

	content := string(result)
	if ft == fileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			return fmt.Errorf("convert gh to json: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := proc.getOutputFilename(src)
	if err := proc.writeOutput(content, outputFilename); err != nil {
		return fmt.Errorf("write gh output: %w", err)
	}

	return nil
}

// catalogDomainPrefix returns the file-name prefix used to derive a catalog
// entry's type tag for the grouped domains, and whether the domain is grouped.
// goods/books/ntl split by file into the frontend's `type`→`topics` catalog.
func catalogDomainPrefix(domain string) (string, bool) {
	switch domain {
	case "goods":
		return "goods.", true
	case "books":
		return "books.", true
	case "ntl":
		return "", true
	default:
		return "", false
	}
}

// catalogEntry is one output `{type, topics}` group, matching the frontend
// CatalogType contract consumed by the goods/books/media pages.
type catalogEntry struct {
	Type   string `json:"type" yaml:"type"`
	Topics []any  `json:"topics" yaml:"topics"`
}

// collectCatalogEntries builds the grouped goods/books/ntl entries: one entry
// per source file, typed by its file-name stem (goods.EDC.yml → "EDC"). It
// keeps file boundaries so the `type` grouping survives, which the
// single-stream path flattens away.
func collectCatalogEntries(src, prefix string) ([]catalogEntry, error) {
	files, err := fileutil.ListYAMLFilesRecursive(src)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", src, err)
	}

	entries := make([]catalogEntry, 0, len(files))
	for _, yf := range files {
		data, err := os.ReadFile(yf)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", yf, err)
		}

		topics, err := parser.NewParser[any](data).ParseFlatten()
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", yf, err)
		}
		if len(topics) == 0 {
			continue
		}

		entries = append(entries, catalogEntry{
			Type:   fileutil.TypeFromFilename(filepath.Base(yf), prefix),
			Topics: topics,
		})
	}

	return entries, nil
}

// marshalCatalog serializes catalog entries in the requested format. The data
// is already decoded Go structs, so JSON is encoded directly rather than
// round-tripped through YAML.
func marshalCatalog(entries []catalogEntry, ft fileType) (string, error) {
	if ft == fileTypeJSON {
		data, err := json.Marshal(entries)
		if err != nil {
			return "", fmt.Errorf("marshal catalog json: %w", err)
		}
		return string(data), nil
	}

	data, err := yaml.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal catalog %s: %w", ft, err)
	}
	return string(data), nil
}

// normalizeFormat converts user-facing format names to internal fileType.
func normalizeFormat(f string) fileType {
	switch f {
	case "json":
		return fileTypeJSON
	case "yaml", "yml":
		return fileTypeYAML
	default:
		return ""
	}
}

// serviceParseModeMap returns the parse mode for each domain.
func serviceParseModeMap() map[string]render.ParseMode {
	return map[string]render.ParseMode{
		"goods": render.ParseFlatten,
		"task":  render.ParseMulti,
		"gh":    render.ParseFlatten,
		"books": render.ParseFlatten,
		"ntl":   render.ParseFlatten,
	}
}

// ---------------------------------------------------------------------------
// docProcessor — file I/O and output helpers
// ---------------------------------------------------------------------------

type docProcessor struct {
	Dst             string `yaml:"dst"`
	MergeOutputFile string `yaml:"mergeOutputFile"`
	currentFile     string
	fileType        fileType
}

func newDocProcessor(fileType fileType) *docProcessor {
	return &docProcessor{fileType: fileType}
}

func (p *docProcessor) setCurrentFile(filename string) {
	p.currentFile = filename
}

func (p *docProcessor) getOutputFilename(src string) string {
	if p.MergeOutputFile != "" {
		return p.MergeOutputFile
	}

	base := filepath.Base(src)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	return name + "." + string(p.fileType)
}

func (p *docProcessor) processFile(src string, renderer render.Renderer) error {
	data, err := p.readInput(src, isDir(src))
	if err != nil {
		slog.Error("read file error", "file", src, "error", err.Error())

		return fmt.Errorf("read file error: %w", err)
	}

	content, err := renderer.Render(data)
	if err != nil {
		slog.Error("render error", "file", src, "error", err.Error())

		return fmt.Errorf("render error: %w", err)
	}

	if p.fileType == fileTypeJSON {
		jsonData, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			slog.Error("convert to json error", "file", src, "error", err.Error())

			return fmt.Errorf("convert to json error: %w", err)
		}
		content = string(jsonData)
	}

	outputFilename := p.getOutputFilename(src)
	if err := p.writeOutput(content, outputFilename); err != nil {
		slog.Error("write file error", "file", outputFilename, "error", err.Error())

		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}

func (p *docProcessor) readInput(src string, isDir bool) ([]byte, error) {
	if isDir {
		return p.readAndMergeFiles(src)
	}

	return p.readSingleFile(src)
}

func (p *docProcessor) readSingleFile(src string) ([]byte, error) {
	if isDir(src) {
		return []byte(""), errors.New("stat path error")
	}

	return fileutil.ReadSingleFile(src, p.setCurrentFile)
}

func (p *docProcessor) readAndMergeFiles(src string) ([]byte, error) {
	if !isDir(src) {
		return []byte(""), errors.New("stat path error")
	}

	return fileutil.ReadAndMergeYAMLFilesRecursive(src, p.setCurrentFile)
}

func isDir(path string) bool {
	fi, err := os.Stat(path)

	return err == nil && fi.IsDir()
}

func (p *docProcessor) writeOutput(content, filename string) error {
	if err := fileutil.EnsureDir(p.Dst); err != nil {
		return fmt.Errorf("create dir error: %w", err)
	}

	outputPath := filepath.Join(p.Dst, filename)
	if err := fileutil.AtomicWriteFile(outputPath, []byte(content), fileutil.FilePermPrivate); err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}
