package main

import (
	"flag"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/commands"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
)

var docsDir = flag.String("docs-dir", "", "Path to documentation")

type section struct {
	start   int
	end     int
	command string
	section string
	code    bool
}

func main() {
	_ = syscall.Setenv("COLUMNS", "120")

	if os.Getenv("CALL_KLUCTL") == "true" {
		kluctlMain()
		return
	}

	flag.Parse()

	filepath.WalkDir(*docsDir, func(path string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		processFile(path)
		return nil
	})
}

func kluctlMain() {
	commands.Main()
}

func processFile(path string) {
	text, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(text), "\n")

	var newLines []string
	pos := 0
	for true {
		s := findNextSection(lines, pos)
		if s == nil {
			newLines = append(newLines, lines[pos:]...)
			break
		}

		newLines = append(newLines, lines[pos:s.start+1]...)

		s2 := getHelpSection(s.command, s.section)

		if s.code {
			newLines = append(newLines, "```")
		}
		newLines = append(newLines, s2...)
		if s.code {
			newLines = append(newLines, "```")
		}

		newLines = append(newLines, lines[s.end])
		pos = s.end + 1
	}

	if !reflect.DeepEqual(lines, newLines) {
		err = ioutil.WriteFile(path, []byte(strings.Join(newLines, "\n")), 0o600)
		if err != nil {
			log.Fatal(err)
		}
	}
}

var beginPattern = regexp.MustCompile(`<!-- BEGIN SECTION "([^"]*)" "([^"]*)" (true|false) -->`)
var endPattern = regexp.MustCompile(`<!-- END SECTION -->`)

func findNextSection(lines []string, start int) *section {

	for i := start; i < len(lines); i++ {
		m := beginPattern.FindSubmatch([]byte(lines[i]))
		if m == nil {
			continue
		}

		var s section
		s.start = i
		s.command = string(m[1])
		s.section = string(m[2])
		s.code = string(m[3]) == "true"

		for j := i + 1; j < len(lines); j++ {
			m = endPattern.FindSubmatch([]byte(lines[j]))
			if m == nil {
				continue
			}
			s.end = j
			return &s
		}
	}
	return nil
}

func countIndent(str string) int {
	for i := 0; i < len(str); i++ {
		if str[i] != ' ' {
			return i
		}
	}
	return 0
}

func getHelpSection(command string, section string) []string {
	log.Printf("Getting section '%s' from command '%s'", section, command)

	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	args := strings.Split(command, " ")
	args = append(args, "--help")

	helpCmd := exec.Command(exe, args...)
	helpCmd.Env = os.Environ()
	helpCmd.Env = append(helpCmd.Env, "CALL_KLUCTL=true")

	out, err := helpCmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(string(out), "\n")

	sectionStart := -1
	for i := 0; i < len(lines); i++ {
		indent := countIndent(lines[i])
		if strings.HasPrefix(lines[i][indent:], fmt.Sprintf("%s:", section)) {
			sectionStart = i
			break
		}
	}
	if sectionStart == -1 {
		log.Fatalf("Section %s not found in command %s", section, command)
	}

	var ret []string
	ret = append(ret, lines[sectionStart])
	for i := sectionStart + 1; i < len(lines); i++ {
		indent := countIndent(lines[i])
		if len(lines[i]) != 0 && indent == 0 && lines[i][len(lines[i])-1] == ':' {
			// new section has started
			break
		}
		ret = append(ret, lines[i])
	}
	return ret
}
