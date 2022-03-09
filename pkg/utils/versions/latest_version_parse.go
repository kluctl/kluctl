package versions

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"text/scanner"
)

type tokenAndText struct {
	tok  rune
	text string
}

type preparsed struct {
	tokens  []tokenAndText
	nextPos int
}

func (p *preparsed) CurToken() rune {
	if p.nextPos > len(p.tokens) {
		return scanner.EOF
	}
	return p.tokens[p.nextPos-1].tok
}

func (p *preparsed) CurTokenText() string {
	if p.nextPos > len(p.tokens) {
		log.Panicf("CurTokenText at EOF")
	}
	return p.tokens[p.nextPos-1].text
}

func (p *preparsed) Next() rune {
	if p.nextPos > len(p.tokens) {
		return scanner.EOF
	}
	p.nextPos += 1
	return p.CurToken()
}

func (p *preparsed) Peek() rune {
	return p.PeekN(0)
}

func (p *preparsed) PeekN(n int) rune {
	if p.nextPos+n >= len(p.tokens) {
		return scanner.EOF
	}
	return p.tokens[p.nextPos+n].tok
}

func ParseLatestVersion(str string) (LatestVersionFilter, error) {
	var p preparsed
	var s scanner.Scanner
	s.Init(strings.NewReader(str))
	s.Mode |= scanner.ScanIdents | scanner.ScanStrings

	for true {
		tok := s.Scan()
		if tok == scanner.EOF {
			break
		}
		p.tokens = append(p.tokens, tokenAndText{
			tok:  tok,
			text: s.TokenText(),
		})
	}

	return parseFilter(&p)
}

func parseFilter(p *preparsed) (LatestVersionFilter, error) {
	tok := p.Next()

	if tok != scanner.Ident {
		return nil, fmt.Errorf("unexpected token %v, expected ident", p.CurToken())
	}
	name := p.CurTokenText()

	tok = p.Next()
	if tok != '(' {
		return nil, fmt.Errorf("unexpected token %v, expected (", tok)
	}

	var f LatestVersionFilter

	var err error
	switch name {
	case "regex":
		f, err = parseRegexFilter(p)
	case "semver":
		f, err = parseSemVerFilter(p)
	case "prefix":
		f, err = parsePrefixFilter(p)
	case "number":
		f, err = parseNumberFilter(p)
	}
	if err != nil {
		return nil, err
	}

	tok = p.Next()
	if tok != ')' {
		return nil, fmt.Errorf("unexpected token %v, expected (", tok)
	}

	return f, nil
}

func parseRegexFilter(p *preparsed) (LatestVersionFilter, error) {
	args := []*arg{
		{name: "pattern", tok: scanner.String, required: true},
	}
	err := parseArgs(p, args)
	if err != nil {
		return nil, err
	}

	if args[0].value == "" {
		return nil, fmt.Errorf("pattern can't be empty")
	}

	return NewRegexVersionFilter(args[0].value.(string))
}

func parseSemVerFilter(p *preparsed) (LatestVersionFilter, error) {
	args := []*arg{
		{name: "allow_no_nums", tok: scanner.Ident, value: false, isBool: true},
	}
	err := parseArgs(p, args)
	if err != nil {
		return nil, err
	}

	return NewLooseSemVerVersionFilter(args[0].value.(bool)), nil
}

func parsePrefixFilter(p *preparsed) (LatestVersionFilter, error) {
	args := []*arg{
		{name: "prefix", tok: scanner.String, required: true},
		{name: "suffix", tok: scanner.Ident, value: ""},
	}
	err := parseArgs(p, args)
	if err != nil {
		return nil, err
	}

	var suffix LatestVersionFilter
	if args[1].found {
		x, ok := args[1].value.(LatestVersionFilter)
		if !ok {
			return nil, fmt.Errorf("invalid suffix, must be a filter")
		}
		suffix = x
	}

	return NewPrefixVersionFilter(args[0].value.(string), suffix)
}

func parseNumberFilter(p *preparsed) (LatestVersionFilter, error) {
	return NewNumberVersionFilter()
}

type arg struct {
	name     string
	tok      rune
	value    interface{}
	required bool
	isBool   bool

	found bool
}

func parseArgs(p *preparsed, args []*arg) error {
	var parsedArgs []arg
	parsedKvArgs := make(map[string]arg)

	for true {
		var name string

		if p.Peek() == ')' {
			break
		}
		if len(parsedArgs)+len(parsedKvArgs) != 0 {
			if tok := p.Next(); tok != ',' {
				return fmt.Errorf("unexpected token %v, expected ')' or ','", tok)
			}
		}

		a, err := parseArg(p)
		if err != nil {
			return err
		}

		if a.name == "" {
			if len(parsedKvArgs) != 0 {
				return fmt.Errorf("can't have unnamed args after named args")
			}
			parsedArgs = append(parsedArgs, *a)
		} else {
			if _, ok := parsedKvArgs[name]; ok {
				return fmt.Errorf("duplicate named arg %s", name)
			}
			parsedKvArgs[a.name] = *a
		}
	}

	if len(parsedArgs) > len(args) {
		return fmt.Errorf("too many arguments")
	}
	for i, a := range parsedArgs {
		if a.tok != args[i].tok {
			return fmt.Errorf("unexpected argument type for %s, expected %v, got %v", a.name, args[i].tok, a.tok)
		}
		args[i].value = a.value
		args[i].found = true
	}

	for k, v := range parsedKvArgs {
		foundArg := false
		for _, a := range args {
			if k == a.name {
				foundArg = true
				if a.found {
					return fmt.Errorf("named arg %s has already been provided via unnamed args", k)
				}
				a.value = v.value
				a.found = true
			}
		}
		if !foundArg {
			return fmt.Errorf("unkown arg %s", k)
		}
	}

	for _, a := range args {
		if a.required && !a.found {
			return fmt.Errorf("required arg %s not found", a.name)
		}
		if a.found && a.isBool {
			if a.tok != scanner.Ident {
				return fmt.Errorf("invalid value for arg %s, must be a bool", a.name)
			}
			b, err := strconv.ParseBool(a.value.(string))
			if err != nil {
				return fmt.Errorf("invalid value for arg %s, must be a bool", a.name)
			}
			a.value = b
		}
	}

	return nil
}

func parseArg(p *preparsed) (*arg, error) {
	parseValue := func() (rune, interface{}, error) {
		if p.Peek() == scanner.Ident && p.PeekN(1) == '(' {
			f, err := parseFilter(p)
			return scanner.Ident, f, err
		}
		tok := p.Next()
		if tok != scanner.String && tok != scanner.Ident && tok != scanner.Int {
			return 'e', nil, fmt.Errorf("unexpected token %v, expected string, ident or int", tok)
		}
		value := p.CurTokenText()
		if tok == scanner.String {
			value = value[1 : len(value)-1]
		}
		return tok, value, nil
	}

	if p.Peek() == scanner.Ident && p.PeekN(1) == '=' {
		p.Next() // consume ident
		name := p.CurTokenText()
		p.Next() // consume '='

		valueTok, value, err := parseValue()
		if err != nil {
			return nil, err
		}
		a := arg{
			name:  name,
			tok:   valueTok,
			value: value,
		}
		return &a, nil
	}

	valueTok, value, err := parseValue()
	if err != nil {
		return nil, err
	}
	a := arg{
		tok:   valueTok,
		value: value,
	}
	return &a, nil
}
