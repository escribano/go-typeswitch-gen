package gen

import (
	"sort"

	"go/ast"
	"go/token"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

// sortFileTypeSwitches is the main logic for "sort" mode.
// It sorts the case clauses in type switch statements in file by the popularity of
// the interfaces implemented by the case types.
// Cases with type which implements more popular interfaces are sorted first, for example:
//   case A: // implements I1
//   case B: // implements I2
//   case C: // implements I1, I2
//   case D: // implements I2
// Will be sorted as C, B, D, A, as I2 is more popular than I1.
func (g Gen) sortFileTypeSwitches(pkg *loader.PackageInfo, file *ast.File) error {
	ast.Inspect(file, func(n ast.Node) bool {
		if stmt, ok := n.(*ast.TypeSwitchStmt); ok {
			sort.Sort(g.byInterface(stmt.Body.List, &pkg.Info))
			// sort.Sort(byName{stmt.Body.List, g})

			// Remove empty lines between cases
			// as sorting cases will break the spacing.
			for _, st := range stmt.Body.List {
				if cc, ok := st.(*ast.CaseClause); ok {
					cc.Case = token.NoPos
					cc.Colon = token.NoPos
				}
			}

			return false
		}

		return true
	})

	return nil
}

type byTypeName struct {
	list []ast.Stmt
	gen  *Gen
}

func (s byTypeName) Len() int      { return len(s.list) }
func (s byTypeName) Swap(i, j int) { s.list[i], s.list[j] = s.list[j], s.list[i] }
func (s byTypeName) Less(i, j int) bool {
	cc1 := s.list[i].(*ast.CaseClause)
	cc2 := s.list[j].(*ast.CaseClause)

	if cc1.List == nil {
		return false
	}
	if cc2.List == nil {
		return true
	}

	type1 := s.gen.showNode(cc1.List[0])
	type2 := s.gen.showNode(cc2.List[0])

	return type1 < type2
}

// Sort case clauses by popularity (most polular to less)
func (g Gen) byInterface(list []ast.Stmt, info *types.Info) byInterfacePopularity {
	// First rank interfaces by their ocurrances
	caseTypes := map[types.Type]bool{}

	for _, st := range list {
		cc := st.(*ast.CaseClause)
		if cc.List == nil {
			continue
		}

		// We assume the case clause is inside a type switch statement
		// and the List has at most one element which is a type expression.
		caseTypes[info.TypeOf(cc.List[0])] = true
	}

	// Count all interfaces' implementation counts
	implCounts := map[types.Type]int{}
	for _, info := range g.program.AllPackages {
		for _, obj := range info.Defs {
			if tn, ok := obj.(*types.TypeName); ok {
				t := tn.Type()
				if _, ok := t.Underlying().(*types.Interface); ok {
					implCounts[t] = 0
				}
			}
		}
	}

	interfaceOrder := []types.Type{}
	for i := range implCounts {
		for t := range caseTypes {
			in := i.Underlying().(*types.Interface)
			if types.Implements(t, in) {
				implCounts[i] = implCounts[i] + 1
			}
		}
		if implCounts[i] > 0 {
			interfaceOrder = append(interfaceOrder, i)
		}
	}

	sort.Sort(byImplCount{interfaceOrder, implCounts})

	g.log(nil, nil, "%v", interfaceOrder)

	return byInterfacePopularity{
		list:       list,
		interfaces: interfaceOrder,
		gen:        &g,
		info:       info,
	}
}

type byImplCount struct {
	interfaces []types.Type
	count      map[types.Type]int
}

func (s byImplCount) Len() int { return len(s.interfaces) }
func (s byImplCount) Swap(i, j int) {
	s.interfaces[i], s.interfaces[j] = s.interfaces[j], s.interfaces[i]
}
func (s byImplCount) Less(i, j int) bool {
	i1, i2 := s.interfaces[i], s.interfaces[j]

	if s.count[i1] == s.count[i2] {
		return i1.String() < i2.String()
	}

	return s.count[i1] > s.count[i2]
}

type byInterfacePopularity struct {
	list       []ast.Stmt
	interfaces []types.Type
	gen        *Gen
	info       *types.Info
}

func (s byInterfacePopularity) Len() int { return len(s.list) }
func (s byInterfacePopularity) Swap(i, j int) {
	s.list[i], s.list[j] = s.list[j], s.list[i]
}
func (s byInterfacePopularity) Less(i, j int) bool {
	l1 := s.list[i].(*ast.CaseClause).List
	l2 := s.list[j].(*ast.CaseClause).List

	if l1 == nil {
		return false
	}
	if l2 == nil {
		return true
	}

	e1, e2 := l1[0], l2[0]
	t1, t2 := s.info.TypeOf(e1), s.info.TypeOf(e2)

	for _, in := range s.interfaces {
		impl1 := types.Implements(t1, in.Underlying().(*types.Interface))
		impl2 := types.Implements(t2, in.Underlying().(*types.Interface))

		if impl1 != impl2 {
			s.gen.log(nil, nil, "%s implements %s = %v", t1, in, impl1)
			s.gen.log(nil, nil, "%s implements %s = %v", t2, in, impl2)

			return impl1
		}
	}

	return s.gen.showNode(e1) < s.gen.showNode(e2)
}
