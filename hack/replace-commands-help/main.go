package main

import (
    "bytes"
    "flag"
    "fmt"
    "github.com/alecthomas/kong"
    log "github.com/sirupsen/logrus"
    "io/ioutil"
    "reflect"
    "regexp"
    "strings"
    "syscall"

    "github.com/codablock/kluctl/cmd/kluctl/commands"
)

var commandsMDPath = flag.String("commands-md", "", "Path to commands.md")

type section struct {
    start int
    end int
    command string
    section string
    code bool
}

func main() {
    _ = syscall.Setenv("COLUMNS", "120")

    flag.Parse()
    text, err := ioutil.ReadFile(*commandsMDPath)
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
        err = ioutil.WriteFile(*commandsMDPath, []byte(strings.Join(newLines, "\n")), 0o600)
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
    log.Infof("Getting section '%s' from command '%s'", section, command)

    exitFunc := func(x int) {
        _ = command
    }

    helpBuf := bytes.NewBuffer(nil)
    parser, _, err := commands.ParseArgs([]string{command, "--help"}, kong.Exit(exitFunc), kong.Writers(helpBuf, helpBuf))
    parser.FatalIfErrorf(err)

    lines := strings.Split(helpBuf.String(), "\n")

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
    sectionIndent := countIndent(lines[sectionStart])
    for i := sectionStart + 1; i < len(lines); i++ {
        indent := countIndent(lines[i])
        if lines[i] != "" && indent <= sectionIndent {
            break
        }
        ret = append(ret, lines[i])
    }
    return ret
}

