package yaml

import (
	"bytes"
	"errors"
	"io/ioutil"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

func Marshal(v interface{}) ([]byte, error) {
	return NewEncoder().Encode(v)
}

func WriteFile(filename string, v interface{}) error {
	data, err := NewEncoder().Encode(v)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0777)
}

type Encoder struct {
	buf bytes.Buffer
}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) Reset() {
	e.buf.Reset()
}

func (e *Encoder) Encode(i interface{}) (data []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(err)
			}
			err = r.(error)
		}
	}()

	val := reflect.ValueOf(i)
	e.value(reflect.Indirect(val), 0, stateDefault)
	data = e.buf.Bytes()
	return
}

func (e *Encoder) error(info string) {
	panic(errors.New(info))
}

func (e *Encoder) indent(n int) {
	for i := 0; i < n; i++ {
		e.buf.WriteByte(' ')
	}
}

func (e *Encoder) value(val reflect.Value, indent, state int) {
	switch val.Kind() {
	case reflect.Int, reflect.Int64:
		e.buf.WriteString(strconv.FormatInt(val.Int(), 10))
		e.buf.WriteByte('\n')

	case reflect.Float64:
		e.buf.WriteString(strconv.FormatFloat(val.Float(), 'g', -1, 64))
		e.buf.WriteByte('\n')

	case reflect.String:
		e.string(val.String(), indent)
		e.buf.WriteByte('\n')

	case reflect.Bool:
		e.buf.WriteString(strconv.FormatBool(val.Bool()))
		e.buf.WriteByte('\n')

	case reflect.Slice:
		if state == stateObjectValue {
			e.buf.WriteByte('\n')
		}

		for i,n := 0, val.Len(); i<n; i++ {
			if i != 0 || state != stateListElem {
				e.indent(indent)
			}
			e.buf.WriteByte('-')
			e.buf.WriteByte(' ')
			e.value(val.Index(i), indent+2, stateListElem)
		}

	case reflect.Map:
		if state == stateObjectValue {
			e.buf.WriteByte('\n')
		}

		for i, key := range val.MapKeys() {
			if i != 0 || state != stateListElem {
				e.indent(indent)
			}
			e.key(key.String())
			e.buf.WriteByte(':')
			e.buf.WriteByte(' ')
			e.value(val.MapIndex(key), indent+2, stateObjectValue)
			e.buf.WriteByte('\n')
		}

	case reflect.Struct:
		if state == stateObjectValue {
			e.buf.WriteByte('\n')
		}

		t := val.Type()
		needIdent := state != stateListElem
		var name string
		for i, n := 0, t.NumField(); i < n; i++ {
			f := t.Field(i)
			if f.PkgPath == "" {
				name = f.Tag.Get("yaml")
				fv := val.Field(i)
				if name == "" {
					name = f.Name
				} else {
					if i := strings.Index(name, ","); i != -1 {
						if strings.Index(name, "omitempty") != -1 {
							switch f.Kind {
							case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
								if fv.Len() == 0 {
									continue
								}
							}
						}
						name = name[:i]
					}
				}

				if needIdent {
					e.indent(indent)
				} else {
					needIdent = true
				}
				e.key(name)
				e.buf.WriteByte(':')
				e.buf.WriteByte(' ')
				e.value(fv, indent+2, stateObjectValue)
				e.buf.WriteByte('\n')
			}
		}

	default:
		e.error("unsupported type "+val.Type().String())
	}
}

func (e *Encoder) key(key string) {
	if strings.IndexAny(key, "\n\r\t  #") != -1 {
		key = strconv.Quote(key)
	}
	e.buf.WriteString(key)
}

func (e *Encoder) string(str string, indent int) {
	if str == "" {
		return
	}

	i := strings.IndexByte(str, '\n')
	if i == -1 {
		if strings.IndexByte(str, '#') != -1 {
			e.buf.WriteByte('\n')
			e.indent(indent)
		}
		e.buf.WriteString(str)
		return
	}

	if str[len(str)-1] == '\n' {
		e.buf.WriteByte('>')
		str = str[:len(str)-1]
	}
	e.buf.WriteByte('\n')

	var line string
	for ; i != -1; i = strings.IndexByte(str, '\n') {
		line, str = str[:i+1], str[i+1:]
		if len(line) != 1 {
			e.indent(indent)
			e.buf.WriteString(line)
		} else {
			e.buf.WriteByte('\n')
		}
		e.buf.WriteByte('\n')
	}

	if len(str) != 0 {
		e.indent(indent)
		e.buf.WriteString(str)
	}
}
