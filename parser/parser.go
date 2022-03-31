package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
)

const (
	structComment     = "easyjson:json"
	structSkipComment = "easyjson:skip"
)

type Parser struct {
	PkgPath     string
	PkgName     string
	BuildTags   string
	StructNames []string
	AllStructs  bool
}

type visitor struct {
	*Parser

	name string
}

func (p *Parser) needType(comments *ast.CommentGroup) (skip, explicit bool) {
	if comments == nil {
		return
	}

	for _, v := range comments.List {
		comment := v.Text

		if len(comment) > 2 {
			switch comment[1] {
			case '/':
				// -style comment (no newline at the end)
				comment = comment[2:]
			case '*':
				/*-style comment */
				comment = comment[2 : len(comment)-2]
			}
		}

		for _, comment := range strings.Split(comment, "\n") {
			comment = strings.TrimSpace(comment)

			if strings.HasPrefix(comment, structSkipComment) {
				return true, false
			}
			if strings.HasPrefix(comment, structComment) {
				return false, true
			}
		}
	}

	return
}

func (v *visitor) Visit(n ast.Node) (w ast.Visitor) {
	switch n := n.(type) {
	case *ast.Package:
		return v
	case *ast.File:
		v.PkgName = n.Name.String()
		for _, comm := range n.Comments {
			if comm.Pos() > n.Package {
				break
			}
			if len(comm.List) == 0 {
				continue
			}
			for _, subComm := range comm.List {
				commText := subComm.Text
				if strings.HasPrefix(commText, "//go:build ") || strings.HasPrefix(commText, "// +build ") {
					v.BuildTags = strings.TrimPrefix(strings.TrimPrefix(commText, "//go:build "), "// +build ")
					return v
				}
			}
		}
		return v

	case *ast.GenDecl:
		skip, explicit := v.needType(n.Doc)

		if skip || explicit {
			for _, nc := range n.Specs {
				switch nct := nc.(type) {
				case *ast.TypeSpec:
					nct.Doc = n.Doc
				}
			}
		}

		return v
	case *ast.TypeSpec:
		skip, explicit := v.needType(n.Doc)
		if skip {
			return nil
		}
		if !explicit && !v.AllStructs {
			return nil
		}

		v.name = n.Name.String()

		// Allow to specify non-structs explicitly independent of '-all' flag.
		if explicit {
			v.StructNames = append(v.StructNames, v.name)
			return nil
		}

		return v
	case *ast.StructType:
		v.StructNames = append(v.StructNames, v.name)
		return nil
	}
	return nil
}

func (p *Parser) Parse(fname string, isDir bool) error {
	log.Println(fname)
	var err error
	if p.PkgPath, err = getPkgPath(fname, isDir); err != nil {
		return err
	}

	fset := token.NewFileSet()
	if isDir {
		packages, err := parser.ParseDir(fset, fname, filter, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, pckg := range packages {
			ast.Walk(&visitor{Parser: p}, pckg)
		}
	} else {
		f, err := parser.ParseFile(fset, fname, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		ast.Walk(&visitor{Parser: p}, f)
	}
	return nil
}

func excludeTestFiles(fi os.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_test.go")
}

func includeGoFiles(fi os.FileInfo) bool {
	return strings.HasSuffix(fi.Name(), ".go")
}

func excludeEasyjsonFiles(fi os.FileInfo) bool {
	return !strings.HasSuffix(fi.Name(), "_easyjson.go")
}

func filter(fi os.FileInfo) bool {
	return includeGoFiles(fi) && excludeTestFiles(fi) && excludeEasyjsonFiles(fi)
}
