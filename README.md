go-typeswitch-gen
=================

## INSTALLATION

    go get github.com/motemen/go-typeswitch-gen/cmd/tsgen

## USAGE

    tsgen [-w] [-main <pkg>] [-verbose] <mode> <file>

    Modes:
      expand: expand generic case clauses in type switch statements by its actual arguments
      sort:   sort case clauses in type switch statements

    Flags:
      -main="": entrypoint package
      -verbose=false: log verbose
      -w=false: write result to (source) file instead of stdout

## USING TEMPLATE VARIABLES

~~~go
// example.go
type T interface{} // treated as a type variable

func onGenericStringMap(m interface{}) {
    switch m := m.(type) {
    case map[string]T:
        var x T
        ...
    }
}
~~~

And in somewhere:

~~~go
// main.go
func main() {
    onGenericStringMap(map[string]bool{})
    onGenericStringMap(map[string]io.Reader{})
}
~~~

And run:

	tsgen example.go

Then you will get type switch clauses whose type variables are replaced with concrete types:

~~~go
func onGenericStringMap(m interface{}) []string {
    switch m := m.(type) {
    case map[string]bool:
        var x bool
        ...
    case map[string]io.Reader:
        var x io.Reader
        ...
    case map[string]T:
        var x T
        ...
    }
}
~~~

## DESCRIPTION

`tsgen` rewrites type switch statements which has template case clauses, which are case clauses with type variables in their case expression (e.g. `case map[string]T:` or `case chan S1:`). `tsgen` analyzes the source code and detects the actual argument types (e.g. `map[string]io.Reader` or `chan bool`), then generates new case clauses with concrete types based on the templates and adds them to the parent type switch statement.

Types with names of uppercase letters and numbers are considered as type variables.

## USAGE WITH `go generate`

Add lines below to expand type switches with `go generate`:

~~~go
//go:generate tsgen -w expand $GOFILE
//go:generate goimports -w $GOFILE
~~~

For a complete example, consult the `_example` directory.

## AUTHOR

motemen <motemen@gmail.com>
