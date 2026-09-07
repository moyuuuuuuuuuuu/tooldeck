package platform

import "fmt"

var runtimeVersions = map[string][]string{"php": {"8.0", "8.1", "8.2", "8.3"}, "node": {"20", "21", "22", "23"}, "python": {"3.10", "3.11", "3.12"}, "go": {"1.22", "1.23", "1.24"}}

func runtimeName(v string) string {
	switch v {
	case "js":
		return "node"
	case "py":
		return "python"
	case "golang":
		return "go"
	}
	return v
}
func runtimeVersion(m Manifest) (string, error) {
	versions := runtimeVersions[runtimeName(m.Runtime)]
	v := m.RuntimeVersion
	if v == "" {
		switch runtimeName(m.Runtime) {
		case "node":
			v = "22"
		case "php":
			v = "8.3"
		case "python":
			v = "3.12"
		case "go":
			v = "1.24"
		}
	}
	for _, allowed := range versions {
		if v == allowed {
			return v, nil
		}
	}
	return "", fmt.Errorf("不支持的运行版本：%s %s", m.Runtime, v)
}
