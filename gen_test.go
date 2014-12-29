package gen

import (
	"testing"

	"go/ast"
	"go/parser"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typeMatchTestCase struct {
	patternType string
	matches     map[string]string
}

func TestTypeParams(t *testing.T) {
	code := `
package E

import (
	"io"
)

type T interface{}
type S interface{}

type xxx struct{}

type in1 map[string][]io.Reader
type in2 map[int]bool
type in3 []chan<- *xxx
type in4 []struct{}
type in5 *xxx
type in6 func(int)
type in7 func(bool) (io.Reader, error)
type in8 struct { foo []byte }

func main() {
	Foo(map[string][]io.Reader{})
	Foo(map[int]bool{})
	Foo(make([]chan<- *xxx, 0))
	Foo([]struct{}{})
}

func Foo(x interface{}) {
	switch x := x.(type) {
	// in1
	case map[string]T:
		var r T // <-- T here
		for _, v := range x {
			r = v
		}
		_ = r

	// in2
	case map[T]bool:
		var keys []T = make([]T, 0)
		for k := range x {
			keys = append(keys, k)
		}
		_ = keys

	// in3
	case []chan<- T:
		var t1, t2 T
		for _, c := range x {
			c <- t1
			c <- t2
		}

	// in4
	case []T:
		var t T = x[0]
		_ = t

	// in5
	case *T:
		var t T = *x
		_ = t

	// in6
	case func(T):
		var t *T
		x(*t)

	// in7
	case func(T) (S, error):
		var t T
		var s S
		s, _ = x(t)
		_ = s

	// in8
	case struct { foo T }:
		var t T = x.foo
		_ = t
	}
}`

	conf := loader.Config{}
	conf.ParserMode = parser.ParseComments
	conf.SourceImports = true

	file, err := conf.ParseFile("test.go", code)
	require.NoError(t, err)

	conf.CreateFromFiles("", file)

	prog, err := conf.Load()
	require.NoError(t, err)

	typeDefs := map[string]types.Type{}

	for _, pkg := range prog.Created {
		for ident, obj := range pkg.Defs {
			if ty, ok := obj.(*types.TypeName); ok {
				typeDefs[ident.Name] = ty.Type().Underlying()
			}
		}
		require.Equal(t, "map[string][]io.Reader", typeDefs["in1"].String())

		cases := map[string]typeMatchTestCase{
			"in1": {
				"map[string]E.T",
				map[string]string{"T": "[]io.Reader"},
			},
			"in2": {
				"map[E.T]bool",
				map[string]string{"T": "int"},
			},
			"in3": {
				"[]chan<- E.T",
				map[string]string{"T": "*E.xxx"},
			},
			"in4": {
				"[]E.T",
				map[string]string{"T": "struct{}"},
			},
			"in5": {
				"*E.T",
				map[string]string{"T": "E.xxx"},
			},
			"in6": {
				"func(E.T)",
				map[string]string{"T": "int"},
			},
			"in7": {
				"func(E.T) (E.S, error)",
				map[string]string{"T": "bool", "S": "io.Reader"},
			},
			"in8": {
				"struct{foo E.T}",
				map[string]string{"T": "[]byte"},
			},
		}

		mode := ssa.SanityCheckFunctions
		ssaProg := ssa.Create(prog, mode)
		ssaPkg := ssaProg.Package(pkg.Pkg)
		ssaProg.BuildAll()
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if fd, ok := decl.(*ast.FuncDecl); ok {
					if fd.Name.Name != "Foo" {
						continue
					}

					_, path, _ := prog.PathEnclosingInterval(fd.Pos(), fd.End())
					f := ssa.EnclosingFunction(ssaPkg, path)
					conf := &pointer.Config{}
					conf.BuildCallGraph = true
					conf.Mains = []*ssa.Package{ssaPkg}
					res, err := pointer.Analyze(conf)
					require.NoError(t, err)

					in := res.CallGraph.CreateNode(f).In
					for _, edge := range in {
						for _, a := range edge.Site.Common().Args {
							t.Logf("%#v", a)
							if mi, ok := a.(*ssa.MakeInterface); ok {
								t.Log(mi.X.Type())
							}
						}
					}
					t.Log(in)
				}
			}
		}
		for node := range pkg.Scopes {
			sw, ok := node.(*ast.TypeSwitchStmt)
			if !ok {
				continue
			}

			stmt := NewTypeSwitchStmt(sw, pkg.Info)
			if stmt == nil {
				continue
			}

			for inTypeName, c := range cases {
				tmpl, m := stmt.FindMatchingTemplate(typeDefs[inTypeName])
				require.NotNil(t, tmpl, inTypeName)
				require.NotNil(t, m, inTypeName)
				assert.Equal(t, c.patternType, tmpl.TypePattern.String(), inTypeName)

				for typeVar, ty := range c.matches {
					assert.Equal(t, ty, m[typeVar].String(), inTypeName)
				}

				newBody := tmpl.Apply(m)
				t.Log(showNode(prog.Fset, newBody))
			}

			sw_ := stmt.Inflate([]types.Type{
				typeDefs["in1"],
				typeDefs["in2"],
				typeDefs["in3"],
				typeDefs["in4"],
				typeDefs["in5"],
				typeDefs["in6"],
				typeDefs["in7"],
				typeDefs["in8"],
			})

			t.Log(showNode(prog.Fset, sw_))
		}
	}
}
