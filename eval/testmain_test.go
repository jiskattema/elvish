package eval

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/elves/elvish/util"
)

var filesToCreate = sorted(
	"a1", "a2", "a3", "a10", "b1", "b2", "b3",
	"foo", "bar", "lorem", "ipsum",
)

func sorted(a ...string) []string {
	sort.Strings(a)
	return a
}

var mods = map[string]string{
	"lorem":    "name = lorem; fn put-name { put $name }",
	"d":        "name = d",
	"a/b/c/d":  "name = a/b/c/d",
	"a/b/c/x":  "use ./d; d = $d:name; use ../../../lorem; lorem = $lorem:name",
	"has/init": "put has/init",
}

var dataDir string

func TestMain(m *testing.M) {
	var exitCode int
	util.InTempDir(func(tmpHome string) {
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", oldHome)

		for _, filename := range filesToCreate {
			file, err := os.Create(filename)
			if err != nil {
				panic(err)
			}
			file.Close()
		}

		util.WithTempDir(func(dir string) {
			dataDir = dir

			for mod, content := range mods {
				fname := filepath.Join(dataDir, "lib", mod+".elv")
				os.MkdirAll(filepath.Dir(fname), 0700)
				err := ioutil.WriteFile(fname, []byte(content), 0600)
				if err != nil {
					panic(err)
				}
			}

			exitCode = m.Run()
		})
	})
	os.Exit(exitCode)
}
