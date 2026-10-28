package core_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/0x6d61/guri/internal/core"
	"github.com/0x6d61/guri/internal/scheme"
)

func TestFilesystemOperations(t *testing.T) {
	root := t.TempDir()
	path := func(name string) string { return strconv.Quote(filepath.Join(root, name)) }
	for name, data := range map[string]string{"元.txt": "日本語\x00\xff", ".hidden": "hidden"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := fmt.Sprintf(`(import (scheme base) (scheme file) (guri filesystem))
 (define root %s) (define source %s) (define target %s) (define renamed %s) (define dir %s)
 (create-directory dir)
 (copy-file source target)
 (rename-file target renamed)
 (define listed (directory-files root))
 (define all (directory-files root #t))
 (define result
  (and (file-directory? dir) (not (file-regular? dir))
   (file-regular? source) (file-regular? source #f) (not (file-directory? source))
   (= (file-size source) 11) (= (file-size renamed) 11)
   (not (file-exists? target)) (not (file-directory? target)) (not (file-regular? target))
   (= (length listed) 3) (member "元.txt" listed) (not (member ".hidden" listed))
   (= (length all) 4) (member ".hidden" all) (not (member "." all)) (not (member ".." all))
   (null? (directory-files dir))))
 (delete-directory dir)
 (and result (not (file-exists? dir)))`, strconv.Quote(root), path("元.txt"), path("copy"), path("renamed"), path("dir"))
	if got := runProgram(t, core.NewRuntime(), source); got != scheme.True {
		t.Fatal(got)
	}
	data, err := os.ReadFile(filepath.Join(root, "renamed"))
	if err != nil || string(data) != "日本語\x00\xff" {
		t.Fatal(data, err)
	}
}

func TestFilesystemErrors(t *testing.T) {
	root := t.TempDir()
	regular := filepath.Join(root, "file")
	if err := os.WriteFile(regular, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "dir")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "child"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	q := strconv.Quote
	missing := q(filepath.Join(root, "missing"))
	badPath := `(string (integer->char 0))`
	cases := []struct {
		name, call string
		file       bool
	}{
		{"list missing", `(directory-files ` + missing + `)`, true},
		{"list file", `(directory-files ` + q(regular) + `)`, true},
		{"mkdir existing", `(create-directory ` + q(dir) + `)`, true},
		{"mkdir parent missing", `(create-directory ` + q(filepath.Join(root, "missing", "child")) + `)`, true},
		{"delete file", `(delete-directory ` + q(regular) + `)`, true},
		{"delete nonempty", `(delete-directory ` + q(dir) + `)`, true},
		{"delete missing", `(delete-directory ` + missing + `)`, true},
		{"rename missing", `(rename-file ` + missing + ` ` + q(filepath.Join(root, "target")) + `)`, true},
		{"rename into file", `(rename-file ` + q(dir) + ` ` + q(regular) + `)`, true},
		{"size directory", `(file-size ` + q(dir) + `)`, true},
		{"size missing", `(file-size ` + missing + `)`, true},
		{"copy existing", `(copy-file ` + q(regular) + ` ` + q(regular) + `)`, true},
		{"copy directory", `(copy-file ` + q(dir) + ` ` + missing + `)`, true},
		{"copy missing", `(copy-file ` + missing + ` ` + q(filepath.Join(root, "target")) + `)`, true},
		{"copy into directory", `(copy-file ` + q(regular) + ` ` + q(dir) + `)`, true},
		{"regular invalid path", `(file-regular? ` + badPath + `)`, true},
		{"directory invalid path", `(file-directory? ` + badPath + `)`, true},
	}
	for _, name := range []string{"directory-files", "create-directory", "delete-directory", "file-directory?", "file-regular?", "file-size"} {
		cases = append(cases, struct {
			name, call string
			file       bool
		}{name + " type", "(" + name + " 42)", false}, struct {
			name, call string
			file       bool
		}{name + " arity", "(" + name + ")", false})
	}
	for _, name := range []string{"rename-file", "copy-file"} {
		cases = append(cases, struct {
			name, call string
			file       bool
		}{name + " type", "(" + name + " " + q(regular) + " 42)", false}, struct {
			name, call string
			file       bool
		}{name + " arity", "(" + name + " " + q(regular) + ")", false})
	}
	for _, name := range []string{"directory-files", "file-directory?", "file-regular?"} {
		cases = append(cases, struct {
			name, call string
			file       bool
		}{name + " flag", "(" + name + " " + q(root) + " 1)", false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := `(and (error-object? e) (not (file-error? e)))`
			if tc.file {
				check = `(file-error? e)`
			}
			src := `(import (scheme base) (guri filesystem)) (guard (e (else ` + check + `)) ` + tc.call + ` #f)`
			if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
				t.Fatal(got)
			}
		})
	}
	if data, err := os.ReadFile(regular); err != nil || string(data) != "preserve" {
		t.Fatal(data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "child")); err != nil {
		t.Fatal(err)
	}
}

func TestFilesystemLibraryEnvironments(t *testing.T) {
	r := core.NewRuntime()
	registerLibrary(t, r, `(define-library (fs-client) (export check) (import (scheme base) (guri filesystem)) (begin (define (check p) (file-directory? p))))`)
	q := strconv.Quote(t.TempDir())
	for _, src := range []string{
		`(import (scheme base) (fs-client)) (check ` + q + `)`,
		`(import (scheme base) (scheme eval)) (eval '(file-directory? ` + q + `) (environment '(guri filesystem)))`,
		`(import (scheme base) (prefix (guri filesystem) fs:)) (and (fs:file-directory? ` + q + `) (cond-expand ((library (guri filesystem)) #t) (else #f)))`,
	} {
		if got := runProgram(t, r, src); got != scheme.True {
			t.Fatal(got)
		}
	}
	if _, err := r.Run(context.Background(), "private.scm", `(import (scheme base)) (directory-files ".")`); err == nil {
		t.Fatal("extension leaked into standard library")
	}
}

func TestFilesystemSymlinks(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "original")
	dir := filepath.Join(root, "directory")
	if err := os.WriteFile(file, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"file-link": file, "dir-link": dir, "dangling": filepath.Join(root, "missing")} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	q := func(name string) string { return strconv.Quote(filepath.Join(root, name)) }
	src := fmt.Sprintf(`(import (scheme base) (guri filesystem))
 (and (file-regular? %s) (not (file-regular? %s #f))
  (file-directory? %s) (not (file-directory? %s #f))
  (not (file-regular? %s)) (not (file-directory? %s)) (= (file-size %s) 4)
  (guard (e (else (file-error? e))) (delete-directory %s) #f)
  (guard (e (else (file-error? e))) (copy-file %s %s) #f)
  (guard (e (else (file-error? e))) (copy-file %s %s) #f))`,
		q("file-link"), q("file-link"), q("dir-link"), q("dir-link"), q("dangling"), q("dangling"), q("file-link"), q("dir-link"), q("original"), q("file-link"), q("original"), q("dangling"))
	if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
		t.Fatal(got)
	}
	src = fmt.Sprintf(`(import (scheme base) (guri filesystem)) (copy-file %s %s) (rename-file %s %s) (and (file-regular? %s #f) (file-regular? %s) (not (file-regular? %s #f)))`, q("file-link"), q("copy"), q("file-link"), q("moved-link"), q("copy"), q("moved-link"), q("moved-link"))
	if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
		t.Fatal(got)
	}
	if _, err := os.Lstat(filepath.Join(root, "file-link")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "data" {
		t.Fatal(data, err)
	}
}

func TestFilesystemRelativePathsAndFreshResults(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("元.txt", nil, 0600); err != nil {
		t.Fatal(err)
	}
	src := `(import (scheme base) (scheme file) (guri filesystem))
 (define a (directory-files "."))
 (string-set! (car a) 0 #\x)
 (set-car! a "changed")
 (create-directory "dir") (copy-file "元.txt" "dir/copy")
 (rename-file "dir/copy" "renamed")
 (define result (and (file-regular? "renamed") (= (file-size "renamed") 0)
  (member "元.txt" (directory-files ".")) (not (file-regular? "changed"))))
 (delete-directory "dir") (delete-file "renamed")
 (and result (not (file-directory? "dir")))`
	if got := runProgram(t, core.NewRuntime(), src); got != scheme.True {
		t.Fatal(got)
	}
}
