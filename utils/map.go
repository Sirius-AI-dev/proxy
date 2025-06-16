package utils

import "dario.cat/mergo"

func MapMerge(dst *map[string]interface{}, src interface{}) error {
	return mergo.Merge(dst, src)
}
