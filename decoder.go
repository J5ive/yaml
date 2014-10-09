/*

A simplified YAML parser for configuration file.
It only implements a subset of YAML.

Supported type:
	Type :=
		string | int | int64 | float64
		| []Type
		| map[string]Type
		| struct (with fields having Type)

Unsupported specification:
	- Document marker ( --- );
	- Inline format (json pattern);
	- Quoted scalar;
	- Comment in Multi-line scalar. For example:

		OK: # this is comment
		  This is
		  a sentense.

		Incorrect:
		  This is # not comment.
		  This is a sentense.

*/
package yaml

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(data).Decode(v)
}

func ReadFile(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	return NewDecoder(data).Decode(v)
}

type Decoder struct {
	data []byte
	off  int
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{data, 0}
}

func (d *Decoder) Reset(data []byte) {
	d.data = data
	d.off = 0
}

func (d *Decoder) Decode(i interface{}) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(err)
			}
			err = r.(error)
		}
	}()

	val := reflect.ValueOf(i)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		d.error("", "expect ptr")
	}
	d.value("", val.Elem(), 0, stateDefault)
	return
}

func (d *Decoder) error(name, info string) {
	panic(fmt.Errorf("%s %s at %d", name, info, d.off))
}

// parse state
const (
	stateDefault = iota
	stateListElem		// Maybe there is no ident
	stateObjectValue	// The left of current line may be ignored.
)

func (d *Decoder) value(name string, val reflect.Value, indent, state int) {
	switch val.Kind() {
	case reflect.Int, reflect.Int64:
		i, err := strconv.ParseInt(d.string(indent), 10, val.Type().Bits())
		if err != nil {
			d.error(name, err.Error())
		}
		val.SetInt(i)

	case reflect.Float64:
		f, err := strconv.ParseFloat(d.string(indent), 64)
		if err != nil {
			d.error(name, err.Error())
		}
		val.SetFloat(f)

	case reflect.String:
		val.SetString(d.string(indent))

	case reflect.Bool:
		b, err := strconv.ParseBool(d.string(indent))
		if err != nil {
			d.error(name, err.Error())
		}
		val.SetBool(b)

	case reflect.Slice:
		if state == stateObjectValue {
			d.nextLine()
		}

		t := val.Type()
		elemType := t.Elem()
		if !val.IsNil() {
			val.SetLen(0)
		} /* else {
			val.Set(reflect.MakeSlice(t, 0, 0))
		}*/

		ok := d.sliceElem(name, val, elemType, indent, state)
		for ok {
			ok = d.sliceElem(name, val, elemType, indent, stateDefault)
		}

	case reflect.Map:
		if state == stateObjectValue {
			d.nextLine()
		}

		t := val.Type()
		elemType := t.Elem()
		if val.IsNil() {
			val.Set(reflect.MakeMap(t))
		}

		var elem reflect.Value
		key := d.key(name, indent, state)
		for key != "" {
			if !elem.IsValid() {
				elem = reflect.New(elemType).Elem()
			} else {
				elem.Set(reflect.Zero(elemType))
			}
			d.value(key, elem, indent+2, stateObjectValue)
			val.SetMapIndex(reflect.ValueOf(key), elem)
			key = d.key(name, indent, stateDefault)
		}

	case reflect.Struct:
		if state == stateObjectValue {
			d.nextLine()
		}

		fields := structFileds(val)
		key := d.key(name, indent, state)
		for key != "" {
			if f, ok := fields[key]; ok {
				d.value(key, f, indent+2, stateObjectValue)
			} else {
				d.error(name, "undefined field "+key)
			}
			key = d.key(name, indent, stateDefault)
		}

	default:
		d.error(name, "unsupported type "+val.Type().String())

	}
}

func (d *Decoder) key(name string, indent, state int) string {
	if !d.tryLine(indent, state) {
		return ""
	}

	if d.off < len(d.data) && d.data[d.off] == '"' {
		return d.quotedKey(name)
	}

	for i := d.off; i < len(d.data); i++ {
		c := d.data[i]
		if c == ':' {
			start := d.off
			d.off = i + 1
			return string(bytes.TrimSpace(d.data[start:i]))
		} else if c == '\n' {
			break
		}
	}

	d.error(name, "expect key")
	return ""
}

func (d *Decoder) quotedKey(name string) string {
LOOP:
	for i := d.off+1; i < len(d.data); i++ {
		switch c := d.data[i]; c {
		case '\n':
			break LOOP

		case '\\':
			i++
			if i>=len(d.data) {
				break LOOP
			}

		case '"':
			key, err := strconv.Unquote(string(d.data[d.off:i+1]))
			if err != nil {
				d.error(name, err.Error())
			}
			for i++; i<len(d.data); i++ {
				switch c = d.data[i]; c {
				case ' ', '\t':

				case ':':
					d.off = i+1; return key
				default:
					break LOOP
				}
			}
		}
	}

	d.error(name, "expect key")
	return ""
}

func (d *Decoder) tryLine(indent, state int) bool {
	var line []byte
	var pos int

	if state == stateListElem {
		line, pos = d.peekLine()
		if len(bytes.TrimSpace(line)) != 0 {
			return true
		}
		d.off = pos
	}

	for {
		line, pos = d.peekLine()
		if d.off == pos { // at Eof
			return false
		}
		if len(bytes.TrimSpace(line)) != 0 {
			break
		}
		d.off = pos
	}

	if hasIndent(line, indent) {
		d.off += indent
		return true
	}
	return false
}

func (d *Decoder) peekLine() ([]byte, int) {
	end := len(d.data)
	for i := d.off; i < len(d.data); i++ {
		c := d.data[i]
		if c == '#' {
			end = i
		} else if c == '\n' {
			if i < end {
				end = i
			}
			return d.data[d.off:end], i + 1
		}
	}
	return d.data[d.off:end], len(d.data)
}

func (d *Decoder) nextLine() {
	for ; d.off < len(d.data); d.off++ {
		if d.data[d.off] == '\n' {
			d.off++
			break
		}
	}
}

func hasIndent(line []byte, indent int) bool {
	if len(line) <= indent {
		return false
	}
	for i := 0; i < indent; i++ {
		if line[i] != ' ' {
			return false
		}
	}
	return true
}

func (d *Decoder) sliceElem(name string, slice reflect.Value, elemType reflect.Type, indent, state int) (ok bool) {
	if d.tryLine(indent, state) && d.data[d.off] == '-' {
		d.off++
		if d.off < len(d.data) && d.data[d.off] == ' ' {
			d.off++
		}
		slice.Set(reflect.Append(slice, reflect.Zero(elemType)))
		d.value(name, slice.Index(slice.Len()-1), indent+2, stateListElem)
		ok = true
	}
	return
}


// multi-line string mode
const (
	strDefault = iota
	strFolded
	strPreserved
)

func (d *Decoder) string(indent int) string {
	line, pos := d.peekLine()
	line = bytes.TrimSpace(line)
	d.off = pos

	if len(line) == 0 {
		return d.strMultiLine(indent, strDefault)
	}
	if len(line) == 1 {
		switch line[0] {
		case '>':
			return d.strMultiLine(indent, strFolded)
		case '|':
			return d.strMultiLine(indent, strPreserved)
		}
	}

	// Thinking:
	// return string(line) + d.strMultiLine(indent, strDefault)
	return string(line)
}

func (d *Decoder) strMultiLine(indent, mode int) string {
	var buf bytes.Buffer
	needSpace, ln := false, 0

	for line := d.getStrLine(indent); line != nil; line = d.getStrLine(indent) {
		if len(line) == 0 {
			ln++
		} else {
			for i := 0; i < ln; i++ {
				buf.WriteByte('\n')
				needSpace = false
			}
			ln = 0

			if mode == strDefault {
				line = bytes.TrimSpace(line)
			}
			if needSpace {
				buf.WriteByte(' ')
			}
			buf.Write(line)
			if mode == strPreserved {
				buf.WriteByte('\n')
			} else {
				needSpace = true
			}
		}
	}
	if mode == strFolded && buf.Len() != 0 {
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (d *Decoder) getStrLine(indent int) []byte {
	line, pos := d.peekStringLine()

	if d.off == pos {
		return nil
	}

	ind := indent
	if len(line) < indent {
		ind = len(line)
	}
	for i := 0; i < ind; i++ {
		if line[i] != ' ' {
			return nil
		}
	}

	d.off = pos
	return line[ind:]
}

func (d *Decoder) peekStringLine() ([]byte, int) {
	for i := d.off; i < len(d.data); i++ {
		if d.data[i] == '\n' {
			return d.data[d.off:i], i + 1
		}
	}
	return d.data[d.off:], len(d.data)
}

func structFileds(val reflect.Value) map[string]reflect.Value {
	m := make(map[string]reflect.Value)
	t := val.Type()
	var name string
	for i, n := 0, t.NumField(); i < n; i++ {
		f := t.Field(i)
		if f.PkgPath == "" {
			name = f.Tag.Get("yaml")
			if name == "" {
				name = f.Name
			} else {
				if i := strings.Index(name, ","); i != -1 {
					name = name[:i]
				}
			}
			m[name] = val.Field(i)
		}
	}
	return m
}
