yaml
====

Another Go YAML Parser for Simple YAML. 

Introduction
------------

A simplified YAML parser for configuration file.
It only implements a subset of YAML.

**Supported type:**

	Type :=
		string | int | int64 | float64
		| []Type
		| map[string]Type
		| struct (with fields having Type)

**Unsupported specification:**

- Document marker;
- Inline format (json pattern);
- Quoted scalar;
- Multi-line scalar doesn't recognize comment.


Example
------


	type Config struct {
		Name string   `yaml:"name"`
		Id int        `yaml:"id"`
    	Tasks []string  `yaml:"tasks"`
	}
  
	func main() {
		var config Config
		err := yaml.ReadFile("config.yaml", &config)
		if err != nil {
			println(err.Error())
			return
		}
    
		println(config.Name)
		// ...
	}
	
