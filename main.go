package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	tmpRoot string
	dryRun  bool
)

func main() {
	flag.StringVar(&tmpRoot, "tmp-root", "/tmp", "root of your temp directory")
	flag.BoolVar(&dryRun, "dry-run", false, "don't delete anything")

	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	if tmpRoot == "" {
		return fmt.Errorf("missing required -tmp-root argument")
	}

	dirs, err := scanForToplevelStuff()
	if err != nil {
		return err
	}

	var totalBytes int64
	for _, dir := range dirs {
		sz, err := estimateTreeSize(dir)
		if err != nil {
			if strings.Contains(err.Error(), "Permission denied") {
				fmt.Printf("ERR: skipping %s for estimation: %v\n", dir, err)
			} else {
				return err
			}
		}

		totalBytes += sz
	}

	for _, dir := range dirs {
		if dryRun {
			fmt.Printf("DRY-RUN: deleting %s\n", dir)
		} else {
			fmt.Printf("deleting %s\n", dir)
			if err := os.RemoveAll(dir); err != nil {
				if strings.Contains(err.Error(), "Permission denied") {
					fmt.Printf("ERR: skipping %s: %v\n", dir, err)
				} else {
					return err
				}
			}
		}
	}

	fmt.Printf(
		"estimated savings ~%s from %d directories\n",
		humanizeBytes(totalBytes),
		len(dirs),
	)

	return nil
}

func humanizeBytes(v int64) string {
	unitF := func(v int64, s string) string {
		return strconv.FormatInt(v, 10) + "" + s
	}
	if v < 1024 {
		return unitF(v, "B")
	}

	v /= 1024
	if v < 1024 {
		return unitF(v, "K")
	}

	v /= 1024
	if v < 1024 {
		return unitF(v, "M")
	}

	v /= 1024
	if v < 1024 {
		return unitF(v, "G")
	}

	v /= 1024
	return unitF(v, "T")
}

func scanForToplevelStuff() ([]string, error) {
	var eligiblePrefixes = []string{
		"007-agent",
		"agent_smith",
		"go-build",
		"jones-agent",
		"Test",
		"test-agent",
		"test-consul-agent",
		"consul",
		"Agent1-agent",
		"Agent2-agent",
		"betty-agent",
		"bob-agent",
		"bonnie-agent",
		"dc1-agent",
		"dc2-agent",
		"gopls-",
		// "vim-go",
		"dc1-consul",
		"dc2-consul",
		"test-container",
	}
	var eligibleFilePrefixes = []string{
		"snapshot",
		"config-err-",
	}
	var eligibleFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^go\..*\.(sum|mod)$`),
		regexp.MustCompile(`^gopls\..*-heap.pb.gz$`),
		regexp.MustCompile(`^gopls\..*-goroutines.txt$`),
		regexp.MustCompile(`^gopls-.*.log$`),
		regexp.MustCompile(`^gopls\..*\.zip$`),
	}

	files, err := ioutil.ReadDir(tmpRoot)
	if err != nil {
		return nil, err
	}

	shouldDelete := func(dir bool, name string) bool {
		if dir {
			if name == "consul-test" { // definitely nuke the weird toplevel
				return true
			}
			for _, pfx := range eligiblePrefixes {
				if strings.HasPrefix(name, pfx) {
					return true
				}
			}
		} else {
			for _, pfx := range eligibleFilePrefixes {
				if strings.HasPrefix(name, pfx) {
					return true
				}
			}
			for _, patt := range eligibleFilePatterns {
				if patt.MatchString(name) {
					return true
				}
			}
		}
		return false
	}

	var roots []string
	for _, st := range files {
		if shouldDelete(st.IsDir(), st.Name()) {
			roots = append(roots, filepath.Join(tmpRoot, st.Name()))
		}
	}

	return roots, nil
}

var duRE = regexp.MustCompile("^([0-9]+)\\s+")

func estimateTreeSize(d string) (int64, error) {
	if d == "" {
		return 0, fmt.Errorf("missing directory name")
	}
	cmd := exec.Command("du", "-s", "--block-size=1", d)

	var stderr bytes.Buffer
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("%v: %s", err, stderr.String())
	}

	s := stdout.String()
	m := duRE.FindStringSubmatch(s)
	if m == nil || len(m) != 2 {
		return 0, fmt.Errorf("unrecognized du output: %s", s)
	}

	v, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unrecognized du output: %s", s)
	}

	return v, nil
}

func nukeTree(d string) error {
	if !filepath.IsAbs(d) {
		return fmt.Errorf("not an absolute path: %s", d)
	}
	st, err := os.Lstat(d)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", d)
	}
	log.Printf("NUKE: %s", d)
	return nil
	// return os.RemoveAll(d)
}
