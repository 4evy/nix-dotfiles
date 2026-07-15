package process

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestPathScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"makeexec":          makeExecutableScript,
			"pathofwithpath":    pathOfWithPathScript,
			"rejectpathlikebin": rejectPathLikeBinScript,
		},
	})
}

func makeExecutableScript(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("makeexec does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: makeexec PATH")
	}
	if err := osWriteFileExecutable(scriptPath(ts, args[0]), []byte("#!/bin/sh\nexit 0\n")); err != nil {
		ts.Fatalf("%v", err)
	}
}

func pathOfWithPathScript(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("pathofwithpath does not support negation")
	}
	if len(args) != 3 {
		ts.Fatalf("usage: pathofwithpath BIN PATH WANT")
	}
	paths := scriptPathList(ts, args[1])
	want := scriptPath(ts, args[2])
	got, ok := PathOfWithPath(args[0], paths)
	if !ok || got != want {
		ts.Fatalf("PathOfWithPath = %q, %v; want %q, true", got, ok, want)
	}
	if _, err := fmt.Fprintln(ts.Stdout(), got); err != nil {
		ts.Fatalf("write stdout: %v", err)
	}
}

func rejectPathLikeBinScript(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("rejectpathlikebin does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: rejectpathlikebin PATH")
	}
	path := scriptPath(ts, args[0])
	if got, ok := PathOfWithPath(path, ""); ok {
		ts.Fatalf("PathOfWithPath = %q, true; want false", got)
	}
	if _, err := fmt.Fprintln(ts.Stdout(), filepath.Base(path)); err != nil {
		ts.Fatalf("write stdout: %v", err)
	}
}

func scriptPath(ts *testscript.TestScript, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return ts.MkAbs(path)
}

func scriptPathList(ts *testscript.TestScript, paths string) string {
	if paths == "" {
		return ""
	}
	var out []string
	for _, path := range filepath.SplitList(paths) {
		out = append(out, scriptPath(ts, path))
	}
	return strings.Join(out, string(filepath.ListSeparator))
}
