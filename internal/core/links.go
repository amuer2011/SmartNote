package core

import (
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

// linkReference stores byte offsets so rewriting leaves labels, titles, YAML,
// whitespace and unrelated Markdown untouched.
type linkReference struct {
	Start, End, FullStart, FullEnd int
	Target, Label                  string
	Wiki, Image                    bool
}

var definitionStart = regexp.MustCompile(`^ {0,3}\[[^\]\n]+\]:[ \t]*`)

func markdownLinks(text string) []linkReference {
	refs := []linkReference{}
	offset := 0
	fence := byte(0)
	fenceLen := 0
	front := strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n")
	for number, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if front {
			if number > 0 && trimmed == "---" {
				front = false
			}
			offset += len(line)
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimLeft(line, " ")
		if indent <= 3 && len(t) >= 3 && (t[0] == '`' || t[0] == '~') {
			count := 0
			for count < len(t) && t[count] == t[0] {
				count++
			}
			if count >= 3 {
				if fence == 0 {
					fence = t[0]
					fenceLen = count
				} else if fence == t[0] && count >= fenceLen && strings.TrimSpace(t[count:]) == "" {
					fence = 0
				}
				offset += len(line)
				continue
			}
		}
		if fence != 0 || indent >= 4 || strings.HasPrefix(line, "\t") {
			offset += len(line)
			continue
		}
		if loc := definitionStart.FindStringIndex(line); loc != nil {
			if start, end, ok := linkDestination(line, loc[1]); ok {
				refs = append(refs, linkReference{Start: offset + start, End: offset + end, Target: unescapeDestination(line[start:end])})
			}
			offset += len(line)
			continue
		}
		for i := 0; i < len(line); i++ {
			if line[i] == '\\' {
				i++
				continue
			}
			if strings.HasPrefix(line[i:], "<!--") {
				if end := strings.Index(line[i+4:], "-->"); end >= 0 {
					i += end + 6
					continue
				}
				break
			}
			if line[i] == '`' {
				run := 1
				for i+run < len(line) && line[i+run] == '`' {
					run++
				}
				if end := strings.Index(line[i+run:], strings.Repeat("`", run)); end >= 0 {
					i += run + end + run - 1
				}
				continue
			}
			if strings.HasPrefix(line[i:], "[[") {
				end := strings.Index(line[i+2:], "]]")
				if end < 0 {
					continue
				}
				end += i + 2
				parts := strings.SplitN(line[i+2:end], "|", 2)
				label := parts[0]
				if len(parts) == 2 {
					label = parts[1]
				}
				start := i
				image := i > 0 && line[i-1] == '!'
				if image {
					start--
				}
				refs = append(refs, linkReference{Start: offset + i + 2, End: offset + i + 2 + len(parts[0]), FullStart: offset + start, FullEnd: offset + end + 2, Target: parts[0], Label: label, Wiki: true, Image: image})
				i = end + 1
				continue
			}
			if line[i] == '[' {
				close := matchingBracket(line, i)
				if close < 0 || close+1 >= len(line) || line[close+1] != '(' {
					continue
				}
				start, end, ok := linkDestination(line, close+2)
				if !ok {
					continue
				}
				pos := end
				if start > close+2 && line[start-1] == '<' {
					pos++
				}
				for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
					pos++
				}
				if pos < len(line) && (line[pos] == '\'' || line[pos] == '"') {
					quote := line[pos]
					pos++
					for pos < len(line) {
						if line[pos] == '\\' {
							pos += 2
							continue
						}
						if line[pos] == quote {
							pos++
							break
						}
						pos++
					}
					for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
						pos++
					}
				}
				if pos >= len(line) || line[pos] != ')' {
					continue
				}
				full := i
				image := i > 0 && line[i-1] == '!'
				if image {
					full--
				}
				refs = append(refs, linkReference{Start: offset + start, End: offset + end, FullStart: offset + full, FullEnd: offset + pos + 1, Target: unescapeDestination(line[start:end]), Label: line[i+1 : close], Image: image})
				i = pos
			}
		}
		offset += len(line)
	}
	return refs
}
func matchingBracket(line string, start int) int {
	depth := 0
	for i := start; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '[' {
			depth++
		}
		if line[i] == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
func linkDestination(line string, pos int) (int, int, bool) {
	for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
		pos++
	}
	if pos >= len(line) {
		return 0, 0, false
	}
	if line[pos] == '<' {
		start := pos + 1
		for i := start; i < len(line); i++ {
			if line[i] == '\\' {
				i++
				continue
			}
			if line[i] == '>' {
				return start, i, true
			}
			if line[i] == '\n' || line[i] == '\r' {
				break
			}
		}
		return 0, 0, false
	}
	start := pos
	depth := 0
	for pos < len(line) {
		c := line[pos]
		if c == '\\' {
			pos += 2
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			if depth == 0 {
				break
			}
			depth--
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			break
		}
		pos++
	}
	return start, pos, pos > start && depth == 0
}
func unescapeDestination(s string) string {
	r := strings.NewReplacer(`\(`, "(", `\)`, ")", `\ `, " ", `\[`, "[", `\]`, "]", `\\`, `\`)
	return r.Replace(s)
}

type textEdit struct {
	Start, End int
	Value      string
}

func applyEdits(text string, edits []textEdit) string {
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
	for _, e := range edits {
		text = text[:e.Start] + e.Value + text[e.End:]
	}
	return text
}
func localTarget(from, target string) (string, string, bool) {
	u, err := url.Parse(strings.ReplaceAll(target, `\`, "/"))
	if err != nil || u.IsAbs() || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
		return "", "", false
	}
	resolved := path.Clean(path.Join(path.Dir(from), u.Path))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", "", false
	}
	suffix := ""
	if u.RawQuery != "" {
		suffix += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		suffix += "#" + u.EscapedFragment()
	}
	return resolved, suffix, true
}
func encodedPath(p string) string { return (&url.URL{Path: p}).EscapedPath() }
func resolveNoteLink(from Note, ref linkReference, notes []Note) *Note {
	target := ref.Target
	if ref.Wiki {
		target = strings.SplitN(target, "#", 2)[0]
		if target == "" {
			return nil
		}
		var found *Note
		for i := range notes {
			n := &notes[i]
			if strings.TrimSuffix(n.Path, ".md") == strings.TrimSuffix(target, ".md") || n.Path == target || n.Title == target || strings.TrimSuffix(path.Base(n.Path), ".md") == target {
				if found != nil && found.ID != n.ID {
					return nil
				}
				found = n
			}
		}
		if found != nil {
			return found
		}
	}
	resolved, _, ok := localTarget(from.Path, target)
	if !ok {
		return nil
	}
	for i := range notes {
		if notes[i].Path == resolved || (ref.Wiki && notes[i].Path == resolved+".md") {
			return &notes[i]
		}
	}
	return nil
}
