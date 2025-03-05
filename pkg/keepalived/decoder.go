package keepalived

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func getStructInfo(value reflect.Value) (map[int]reflect.Value, map[string]reflect.Value, map[string]string, error) {
	pos := make(map[int]reflect.Value)
	name := make(map[string]reflect.Value)
	defaultVal := make(map[string]string)

	typeInfo := value.Type()

	for i := 0; i < value.NumField(); i++ {
		fieldType := typeInfo.Field(i)
		field := value.Field(i)
		if !field.CanSet() {
			continue
		}

		nameTag := fieldType.Tag.Get("keepalive")
		posTag := fieldType.Tag.Get("keepalive_pos")
		nestedTag := fieldType.Tag.Get("keepalive_nested")
		tagsCount := 0
		if nameTag != "" {
			tagsCount++
		}

		if posTag != "" {
			tagsCount++
		}
		if nestedTag != "" {
			tagsCount++
		}
		if tagsCount > 1 {
			return nil, nil, nil, fmt.Errorf("only one keepalive tag allowed")
		}
		if nameTag != "" {
			// Tag can contain several names separated by a comma `,`
			nameTags := strings.Split(nameTag, ",")

			defaultValue := fieldType.Tag.Get("default")

			for _, nameTag := range nameTags {
				if defaultValue != "" {
					defaultVal[nameTag] = defaultValue
				}
				name[nameTag] = field
			}
		}
		if posTag != "" {
			tagValue, err := strconv.ParseUint(posTag, 10, 32)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("invalid positional tag: %s", posTag)
			}
			pos[int(tagValue)] = field
		}
		if nestedTag != "" {
			nestedPos, nestedName, nestedDefault, err := getStructInfo(field)
			if err != nil {
				return nil, nil, nil, err
			}
			for nPos := range nestedPos {
				pos[nPos] = nestedPos[nPos]
			}
			for nName := range nestedName {
				name[nName] = nestedName[nName]
			}
			for nDefault := range nestedDefault {
				defaultVal[nDefault] = nestedDefault[nDefault]
			}
		}
	}

	return pos, name, defaultVal, nil
}

func configIsSingle(cfg *confItem) bool {
	return len(cfg.Values) == 1 && len(cfg.SubItems) == 0
}

func configGetSingle(cfg *confItem) (string, error) {
	if !configIsSingle(cfg) {
		return "", fmt.Errorf("not a single value")
	}
	return cfg.Values[0], nil
}

func configAssignConf(value reflect.Value, cfg *confItem) error {
	// Check UnmarshalText is set and config value is singlular
	if configIsSingle(cfg) {
		// Use value pointer for the method lookup
		unmarshal, ok := value.Addr().Type().MethodByName("UnmarshalText")
		if ok {
			val, _ := configGetSingle(cfg)
			ret := unmarshal.Func.Call([]reflect.Value{value.Addr(), reflect.ValueOf([]byte(val))})
			if !ret[0].IsZero() {
				return ret[0].Interface().(error)
			}
			return nil
		}
	}

	switch value.Type().Kind() {
	// Scalar values
	case reflect.Bool:
		value.SetBool(true)
	case reflect.String:
		val, err := configGetSingle(cfg)
		if err != nil {
			return err
		}
		value.SetString(val)
	case reflect.Uint16:
		str, err := configGetSingle(cfg)
		if err != nil {
			return err
		}
		val, err := strconv.ParseUint(str, 10, 16)
		if err != nil {
			return err
		}
		value.SetUint(val)
	case reflect.Uint:
		str, err := configGetSingle(cfg)
		if err != nil {
			return err
		}
		val, err := strconv.ParseUint(str, 10, 64)
		if err != nil {
			return err
		}
		value.SetUint(val)
	case reflect.Int:
		str, err := configGetSingle(cfg)
		if err != nil {
			return err
		}
		val, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		value.SetInt(val)
	case reflect.Float64:
		str, err := configGetSingle(cfg)
		if err != nil {
			return err
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}
		value.SetFloat(val)

	// Compound values
	case reflect.Slice:
		item := value.Type().Elem()
		member := reflect.New(item).Elem()

		err := configAssignConf(member, cfg)
		if err != nil {
			return err
		}

		value.Set(reflect.Append(value, member))

	case reflect.Ptr:
		item := value.Type().Elem()
		member := reflect.New(item)

		err := configAssignConf(member.Elem(), cfg)
		if err != nil {
			return err
		}

		value.Set(member)

	case reflect.Struct:
		if defaulter, ok := value.Addr().Type().MethodByName("Default"); ok {
			defaulter.Func.Call([]reflect.Value{value.Addr()})
		}

		byPosInfo, byNameInfo, defaultInfo, err := getStructInfo(value)
		if err != nil {
			return err
		}
		// Positional values
		for id, val := range cfg.Values {
			value, ok := byPosInfo[id]
			if !ok {
				return fmt.Errorf("value %v with position id %d was not expected", val, id)
			}
			err := configAssignConf(value, &confItem{
				Name:   "",
				Values: []string{val},
			})
			if err != nil {
				return err
			}
		}

		// Named values
		for name, val := range defaultInfo {
			value := byNameInfo[name]
			item := confItem{
				Name:   name,
				Values: strings.Fields(val),
			}
			err := configAssignConf(value, &item)
			if err != nil {
				return err
			}
		}

		for _, item := range cfg.SubItems {
			value, ok := byNameInfo[item.Name]
			if !ok {
				continue
			}

			if val, ok := defaultInfo[item.Name]; len(item.Values) == 0 && ok {
				item.Values = strings.Fields(val)
			}

			delete(defaultInfo, item.Name)

			err := configAssignConf(value, &item)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown type")
	}
	return nil
}

func configDecode(object any, cfg *confItem) error {
	v := reflect.ValueOf(object)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("should be a pointer to struct")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("should be a pointer to struct")
	}

	return configAssignConf(v, cfg)
}
