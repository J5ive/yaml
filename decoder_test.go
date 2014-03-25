package yaml

import (
	"reflect"
	"testing"
)

func assertEqual(t *testing.T, x, y interface{}) {
	if !reflect.DeepEqual(x, y) {
		t.Errorf("Assert fail! \nExpect: %v\nObtain: %v\n", x, y)
	}
}

func  TestDecodeSimple(t *testing.T) {
	data := []byte(`
a: 1

b : abc

# comment
c: >
  abc
  def

D :

E :
  # comment
  - 1
  - 2
  - 3 #comment


`)

	var s struct {
		A int 		`yaml:"a"`
		B string 	`yaml:"b"`
		C string 	`yaml:"c"`
		D string
		E []int
	}
	
	err := Unmarshal(data, &s)
	assertEqual(err, nil)
	assertEqual(t, s.A, 1)
	assertEqual(t, s.B, "abc")
	assertEqual(t, s.C, "abc def\n")
	assertEqual(t, s.D, "")
	assertEqual(t, s.E, []int{1,2,3})
}
