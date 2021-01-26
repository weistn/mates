package main

import (
	"fmt"
)

// VarType is the type of a VarDef
type VarType int

const (
	// TextType means the value of the variable is a UTF string
	TextType VarType = iota
	// ImageType means the value of the variable the the name of a resource which is an image
	ImageType
)

// VarDef defines a variable
type VarDef struct {
	Name    string
	Type    VarType
	Values  []string
	Default string
}

func (v *VarDef) checkYAMLValue(val interface{}) error {
	str, ok := val.(string)
	if !ok {
		return fmt.Errorf("Value of variable %v must be a string", v.Name)
	}
	if str == v.Default {
		return nil
	}
	if len(v.Values) > 0 {
		for _, o := range v.Values {
			if o == str {
				return nil
			}
		}
		return fmt.Errorf("Value '%v' is illegal for variable %v", str, v.Name)
	}
	return nil
}

func yamlToVarDefs(n interface{}, filename string) (map[string]*VarDef, error) {
	y, ok := n.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("In %v: Vars must be a map", filename)
	}
	result := make(map[string]*VarDef)
	for k, yv := range y {
		switch v := yv.(type) {
		case map[string]interface{}:
			var err error
			result[k], err = yamlToVarDef(k, v, filename)
			if err != nil {
				return nil, err
			}
		case []interface{}:
			lst, err := yamlStrings(k, v, filename)
			if err != nil {
				return nil, err
			}
			var def string
			if len(lst) > 0 {
				def = lst[0]
			}
			result[k] = &VarDef{Name: k, Values: lst, Default: def}
		case string:
			t, err := yamlToVarType(v)
			if err != nil {
				return nil, fmt.Errorf("In %v: %v", filename, err)
			}
			result[k] = &VarDef{Name: k, Type: t}
		default:
			return nil, fmt.Errorf("In %v: A variable must be a defined as a scalar or an object or a list of values", filename)
		}
	}
	return result, nil
}

func yamlToVarType(t string) (VarType, error) {
	switch t {
	case "text", "":
		return TextType, nil
	case "img":
		return ImageType, nil
	}
	return TextType, fmt.Errorf("Unknown variable type: %v", t)
}

func yamlToVarDef(name string, m map[string]interface{}, filename string) (*VarDef, error) {
	vdef := &VarDef{Name: name}
	hasDefault := false
	for k, v := range m {
		switch k {
		case "type":
			t, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("In %v: variable type must be a string", filename)
			}
			var err error
			vdef.Type, err = yamlToVarType(t)
			if err != nil {
				return nil, err
			}
		case "default":
			d, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("In %v: Variable type must be a string", filename)
			}
			vdef.Default = string(d)
			hasDefault = true
		case "values":
			var err error
			vdef.Values, err = yamlStrings(k, v, filename)
			if err != nil {
				return nil, err
			}
			if !hasDefault {
				if len(vdef.Values) > 0 {
					vdef.Default = vdef.Values[0]
				}
			}
		}
	}
	return vdef, nil
}
