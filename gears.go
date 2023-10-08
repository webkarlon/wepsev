package wpsev

import (
	"fmt"
	"strings"
)

func getParseUrl(str string) []string {
	if str == "" {
		str = "/"
	}

	if str[0] != '/' {
		str = "/" + str
	}

	return strings.Split(str, "/")
}

func (s *Server) searchPattern(url string, mtls bool) string {

	hitPatterns := make([]string, 0)
	parseUrl := getParseUrl(url)

	patterns := s.patterns
	if mtls {
		patterns = s.patternsMTLS
	}

	for pattern, _ := range patterns {

		parsePat := getParseUrl(pattern)
		pf := strings.Contains(pattern, "/*")

		if len(parseUrl) < len(parsePat) {
			continue
		}

		if !pf && len(parseUrl) != len(parsePat) {
			continue
		}

		//if len(parseUrl) > 1 {
		//	matched, _ := regexp.MatchString("^*\\.[A-Za-z0-9]{1,6}$", parseUrl[len(parseUrl)-1])
		//
		//	if matched && !pf {
		//		continue
		//	}
		//
		//	if !matched && pf {
		//		continue
		//	}
		//}

		var lenDynamicUrl int

		for i, v := range parsePat {

			if v == parseUrl[i] {
				lenDynamicUrl++
				continue
			}

			if len(v) == 0 {
				continue
			}

			if v[0] == ':' {
				lenDynamicUrl++
				continue
			}

			if v[0] == '*' {
				lenDynamicUrl++
				break
			}
		}

		if lenDynamicUrl == len(parsePat) {
			hitPatterns = append(hitPatterns, pattern)
		}
	}

	if len(hitPatterns) == 0 {
		return ""
	}

	if len(hitPatterns) == 1 {
		return hitPatterns[0]
	}

	m := make(map[string]int)

	for _, hit := range hitPatterns {
		m[hit] = 0
		for i, v := range getParseUrl(hit) {
			if v == parseUrl[i] {
				m[hit]++
			}
		}
	}

	pattern := hitPatterns[0]
	hits := m[pattern]
	for k, v := range m {
		if v > hits {
			pattern = k
			hits = v
		}
	}

	return pattern

}

func (s *Server) checkPattern(method, pattern string, mtls bool) error {

	patterns := s.patterns
	if mtls {
		patterns = s.patternsMTLS
	}

	if _, ok := patterns[pattern][method]; ok {
		return fmt.Errorf("pattern %s already exists", pattern)
	}

	if strings.Contains(pattern, "/*") || strings.Contains(pattern, "/:") {
		for k, v := range patterns {
			if parseDynamicPattern(pattern) == parseDynamicPattern(k) {
				for met, _ := range v {
					if met != method {
						continue
					}
					return fmt.Errorf("mutually exclusive patterns:\n%s\n%s", pattern, k)
				}
			}
		}
	}

	return nil
}

func parseDynamicPattern(pattern string) string {
	if pattern == "/" {
		return pattern
	}

	parse := getParseUrl(pattern)
	var res string
	for _, v := range parse {
		if v == "" {
			continue
		}

		if v[0] == ':' {
			res = res + "/0"
			continue
		}

		if v[0] == '*' {
			res = res + "/1"
			break
		}

		res = res + "/" + v
	}

	return res
}
