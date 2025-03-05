package keepalived

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type confItem struct {
	Name     string
	Values   []string
	SubItems []confItem
	Path     string
	File     string
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isSpecial(r rune) bool {
	return r == '#' || r == '{' || r == '}'
}

func splitLine(line string) ([]string, error) {
	var res []string

	data := []rune(line)

	var prev, curr int
	length := len(data)
	openedQuote := false

	// Skip leading spaces
	for curr = 0; curr < length && unicode.IsSpace(data[curr]); curr++ {
	}

	for ; curr < length; curr++ {
		prev = curr
		r := data[curr]

		if isSpecial(r) {
			res = append(res, string(data[prev:curr+1]))
		} else if r == '"' {
			// Quoted string - scan until next unescaped quote
			// Do not include quotes into returning token
			openedQuote = true
			curr++
			prev = curr
			escaped := false
			for ; curr < length; curr++ {
				r = data[curr]
				if r == '"' && !escaped {
					openedQuote = false
					res = append(res, string(data[prev:curr]))
					break
				}

				if r == '\\' && !escaped {
					escaped = true
				} else {
					escaped = false
				}
			}

			if length == curr {
				res = append(res, string(data[prev:curr]))
			}
		} else {
			// Quoted string - scan until next space
			for ; curr < length; curr++ {
				r = data[curr]
				if isSpace(r) || isSpecial(r) {
					if curr-prev != 0 {
						res = append(res, string(data[prev:curr]))
					}
					break
				}
			}

			if length == curr {
				res = append(res, string(data[prev:curr]))
			}
		}
	}

	if openedQuote {
		return nil, fmt.Errorf("unmatched quotation mark here %s", line)
	}

	return res, nil
}

func parseConfig(scanner *bufio.Scanner, path string, name string, unbalancedBraces *int) ([]confItem, error) {
	res := make([]confItem, 0)

	re := regexp.MustCompile("^[a-zA-Z0-9_-]+$")

	for scanner.Scan() {
		item := confItem{
			File: name,
			Path: path,
		}
		tokens, err := splitLine(scanner.Text())
		if len(tokens) == 0 {
			if err != nil {
				return nil, err
			}
			continue
		}

		// comments can start with both '#' and '!' characters according to keepalived.conf man page
		if strings.HasPrefix(tokens[0], "#") || strings.HasPrefix(tokens[0], "!") {
			continue
		}

		if tokens[0] == "}" {
			// Enclosing bracket - return
			*unbalancedBraces--
			return res, nil
		}

		if tokens[0] == "include" {
			if len(tokens) != 2 {
				return nil, fmt.Errorf("invalid INCLUDE directive")
			}
			glob := filepath.Join(path, tokens[1])
			files, err := filepath.Glob(glob)
			if err != nil {
				return nil, fmt.Errorf("could not get file list: %v", err)
			}
			for _, file := range files {
				include, err := parseFile(file)
				if err != nil {
					return nil, fmt.Errorf("could not include file %s: %v", file, err)
				}
				res = append(res, include.SubItems...)
			}
			continue
		}

		if strings.ContainsRune(tokens[0], '{') {
			return nil, fmt.Errorf("format error: parameter name cannot be a complex object (a list of subitems starting with opening brace)")
		}

		if !re.MatchString(tokens[0]) {
			return nil, fmt.Errorf("format error: unexpected character in parameter name %s: expected only numbers, letters or underscore", tokens[0])
		}

		item.Name = tokens[0]
		tokens = tokens[1:]

		if len(tokens) > 0 && tokens[len(tokens)-1] == "{" {
			tokens = tokens[0 : len(tokens)-1]
			*unbalancedBraces++
			item.SubItems, err = parseConfig(scanner, path, name, unbalancedBraces)
			if err != nil {
				return nil, err
			}
		}

		if len(tokens) > 0 {
			item.Values = tokens
		}

		res = append(res, item)
	}

	return res, nil
}

func parseFile(path string) (*confItem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	root := confItem{
		Path: filepath.Dir(path),
		File: filepath.Base(path),
	}

	unbalancedBraces := 0
	root.SubItems, err = parseConfig(scanner, filepath.Dir(path), filepath.Base(path), &unbalancedBraces)
	if err != nil {
		return nil, err
	}
	if unbalancedBraces != 0 {
		return nil, fmt.Errorf("unbalanced brace in config file")
	}

	return &root, nil
}
