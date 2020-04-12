package apt

// This file implements a generic debian control file parser.
//
// Specifications of files are:
// https://wiki.debian.org/RepositoryFormat
// https://www.debian.org/doc/debian-policy/ch-controlfields.html
//
// According to Debian policy 5.1, folded fields are used in few
// fields such as Uploaders or Binary that we are not insterested in.
// This parser treats them just the same as multiline fields.

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

const (
	maxScanTokenSize = 1 * 1024 * 1024 // 1 MiB
	startBufSize     = 4096            // Default buffer allocation size in bufio
)

// Paragraph is a mapping between field names and values.
//
// Values are a list of strings.  For simple fields, the list has only
// one element.  Newlines are stripped from (multiline) strings.
// Folded fields are treated just the same as multiline fields.
type Paragraph map[string][]string

// Parser reads debian control file and return Paragraph one by one.
//
// PGP preambles and signatures are ignored if any.
type Parser struct {
	s         *bufio.Scanner
	lastField string
	err       error
	isPGP     bool
}

// NewParser creates a parser from a io.Reader.
func NewParser(r io.Reader) *Parser {
	p := &Parser{
		s:     bufio.NewScanner(r),
		isPGP: false,
	}
	b := make([]byte, startBufSize)
	p.s.Buffer(b, maxScanTokenSize)
	return p
}

// Read reads a paragraph.
//
// It returns io.EOF if no more paragraph can be read.
func (p *Parser) Read() (Paragraph, error) {
	if p.err != nil {
		return nil, p.err
	}

	ret := make(Paragraph)
L:
	for p.s.Scan() {
		switch l := p.s.Text(); {
		case len(l) == 0:
			break L
		case l[0] == '#':
			continue
		case l == "-----BEGIN PGP SIGNED MESSAGE-----":
			p.isPGP = true
			for p.s.Scan() {
				if l2 := p.s.Text(); len(l2) == 0 {
					break
				}
			}
			continue
		case p.isPGP && l == "-----BEGIN PGP SIGNATURE-----":
			// skip to EOF
			for p.s.Scan() {
			}
			break L
		case l[0] == ' ' || l[0] == '\t':
			// multiline
			if p.lastField == "" {
				p.err = errors.New("invalid line: " + l)
				return nil, p.err
			}
			ret[p.lastField] = append(ret[p.lastField], strings.Trim(l, " \t"))
		case strings.ContainsRune(l, ':'):
			t := strings.SplitN(l, ":", 2)
			k := t[0]
			v := strings.Trim(t[1], " \t")
			p.lastField = k
			if len(v) == 0 {
				// ignore empty value field
				continue
			}
			ret[k] = append(ret[k], v)
		default:
			p.err = errors.New("invalid line: " + l)
			return nil, p.err
		}
	}
	p.lastField = ""
	if err := p.s.Err(); err != nil {
		p.err = err
	} else if len(ret) == 0 {
		p.err = io.EOF
	}
	if p.err != nil {
		return nil, p.err
	}
	return ret, nil
}
