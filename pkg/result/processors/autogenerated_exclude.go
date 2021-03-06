package processors

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/golangci/golangci-lint/pkg/lint/astcache"
	"github.com/golangci/golangci-lint/pkg/logutils"
	"github.com/golangci/golangci-lint/pkg/result"
)

var autogenDebugf = logutils.Debug("autogen_exclude")

type ageFileSummary struct {
	isGenerated bool
}

type ageFileSummaryCache map[string]*ageFileSummary

type AutogeneratedExclude struct {
	fileSummaryCache ageFileSummaryCache
	astCache         *astcache.Cache
}

func NewAutogeneratedExclude(astCache *astcache.Cache) *AutogeneratedExclude {
	return &AutogeneratedExclude{
		fileSummaryCache: ageFileSummaryCache{},
		astCache:         astCache,
	}
}

var _ Processor = &AutogeneratedExclude{}

func (p AutogeneratedExclude) Name() string {
	return "autogenerated_exclude"
}

func (p *AutogeneratedExclude) Process(issues []result.Issue) ([]result.Issue, error) {
	return filterIssuesErr(issues, p.shouldPassIssue)
}

func (p *AutogeneratedExclude) shouldPassIssue(i *result.Issue) (bool, error) {
	fs, err := p.getOrCreateFileSummary(i)
	if err != nil {
		return false, err
	}

	// don't report issues for autogenerated files
	return !fs.isGenerated, nil
}

// isGenerated reports whether the source file is generated code.
// Using a bit laxer rules than https://golang.org/s/generatedcode to
// match more generated code. See #48 and #72.
func isGeneratedFileByComment(doc string) bool {
	const (
		genCodeGenerated = "code generated"
		genDoNotEdit     = "do not edit"
		genAutoFile      = "autogenerated file" // easyjson
	)

	markers := []string{genCodeGenerated, genDoNotEdit, genAutoFile}
	doc = strings.ToLower(doc)
	for _, marker := range markers {
		if strings.Contains(doc, marker) {
			autogenDebugf("doc contains marker %q: file is generated", marker)
			return true
		}
	}

	autogenDebugf("doc of len %d doesn't contain any of markers: %s", len(doc), markers)
	return false
}

func (p *AutogeneratedExclude) getOrCreateFileSummary(i *result.Issue) (*ageFileSummary, error) {
	fs := p.fileSummaryCache[i.FilePath()]
	if fs != nil {
		return fs, nil
	}

	fs = &ageFileSummary{}
	p.fileSummaryCache[i.FilePath()] = fs

	if i.FilePath() == "" {
		return nil, fmt.Errorf("no file path for issue")
	}

	f := p.astCache.GetOrParse(i.FilePath(), nil)
	if f.Err != nil {
		return nil, fmt.Errorf("can't parse file %s: %s", i.FilePath(), f.Err)
	}

	autogenDebugf("file %q: astcache file is %+v", i.FilePath(), *f)

	doc := getDoc(f.F, f.Fset, i.FilePath())

	fs.isGenerated = isGeneratedFileByComment(doc)
	autogenDebugf("file %q is generated: %t", i.FilePath(), fs.isGenerated)
	return fs, nil
}

func getDoc(f *ast.File, fset *token.FileSet, filePath string) string {
	// don't use just f.Doc: e.g. mockgen leaves extra line between comment and package name

	var importPos token.Pos
	if len(f.Imports) != 0 {
		importPos = f.Imports[0].Pos()
		autogenDebugf("file %q: search comments until first import pos %d (%s)",
			filePath, importPos, fset.Position(importPos))
	} else {
		importPos = f.End()
		autogenDebugf("file %q: search comments until EOF pos %d (%s)",
			filePath, importPos, fset.Position(importPos))
	}

	var neededComments []string
	for _, g := range f.Comments {
		pos := g.Pos()
		filePos := fset.Position(pos)
		text := g.Text()

		// files using cgo have implicitly added comment "Created by cgo - DO NOT EDIT" for go <= 1.10
		// and "Code generated by cmd/cgo" for go >= 1.11
		isCgoGenerated := strings.Contains(text, "Created by cgo") || strings.Contains(text, "Code generated by cmd/cgo")

		isAllowed := pos < importPos && filePos.Column == 1 && !isCgoGenerated
		if isAllowed {
			autogenDebugf("file %q: pos=%d, filePos=%s: comment %q: it's allowed", filePath, pos, filePos, text)
			neededComments = append(neededComments, text)
		} else {
			autogenDebugf("file %q: pos=%d, filePos=%s: comment %q: it's NOT allowed", filePath, pos, filePos, text)
		}
	}

	autogenDebugf("file %q: got %d allowed comments", filePath, len(neededComments))

	if len(neededComments) == 0 {
		return ""
	}

	return strings.Join(neededComments, "\n")
}

func (p AutogeneratedExclude) Finish() {}
