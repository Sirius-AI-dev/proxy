package utils

func InArrayString(arr []string, str string) bool {
	for _, existsString := range arr {
		if existsString == str {
			return true
		}
	}
	return false
}
