package wpsev

import (
	"net/http"
	"testing"
)

// Arg hello/world
// Res ["","hello","world"]
func TestGetParseUrl(t *testing.T) {
	t1 := getParseUrl("")

	if t1[0] != "" || len(t1) != 2 {
		t.Errorf("testt faile t1 res:%v", t1)
		t.Fail()
	}

	t2 := getParseUrl("hello")

	if t2[1] != "hello" || len(t2) != 2 {
		t.Errorf("testt faile t2 res:%v", t2)
		t.Fail()
	}

	t3 := getParseUrl("hello/world")

	if t3[1] != "hello" || t3[2] != "world" || len(t3) != 3 {
		t.Errorf("testt faile t3 res:%v", t3)
		t.Fail()
	}
}

func TestSearchPattern(t *testing.T) {
	md := make(map[string]map[string][]http.Handler)

	md["/test/:id"] = nil
	md["/test/:id/:name"] = nil
	md["/test/:id/:name/:age"] = nil
	md["/file/*path"] = nil
	md["/file/upload/:id/*path"] = nil

	s := Server{
		server:   nil,
		patterns: md,
	}

	var res string

	res = s.searchPattern("/test/1234")
	if res != "/test/:id" {
		t.Errorf("test 1 faile\nexpectation: %v\nresult: %v", "/test/:id", res)
		t.Fail()
	}

	res = s.searchPattern("/test/1234/tom")
	if res != "/test/:id/:name" {
		t.Errorf("test 2 faile\nexpectation: %v\nresult: %v", "/test/:id/:name", res)
		t.Fail()
	}

	res = s.searchPattern("/test/1234/tom/33")
	if res != "/test/:id/:name/:age" {
		t.Errorf("test 3 faile\nexpectation: %v\nresult: %v", "/test/:id/:name/:age", res)
		t.Fail()
	}

	res = s.searchPattern("/file/vol0/766/1.jpeg")
	if res != "/file/*path" {
		t.Errorf("test 4 faile\nexpectation: %v\nresult: %v", "/file/*path", res)
		t.Fail()
	}

	res = s.searchPattern("/file/upload/789/766/1.jpeg")
	if res != "/file/upload/:id/*path" {
		t.Errorf("test 5 faile\nexpectation: %v\nresult: %v", "/file/upload/:id/*path", res)
		t.Fail()
	}
}

func TestCheckPattern(t *testing.T) {
	md := make(map[string][]http.HandlerFunc)

	md["/test/:id"] = nil
	md["/test/:id/:name"] = nil
	md["/test/:id/:name/:age"] = nil
	md["/file/*path"] = nil
	md["/file/upload/:id/*path"] = nil

	var res string

	res = parseDynamicPattern("/")
	if res != "/" {
		t.Errorf("test 1 faile\nexpectation: %v\nresult: %v", "/", res)
		t.Fail()
	}

	res = parseDynamicPattern("/test/:id")
	if res != "/test/0" {
		t.Errorf("test 2 faile\nexpectation: %v\nresult: %v", "/test/0", res)
		t.Fail()
	}

	res = parseDynamicPattern("/test/:id/:name")
	if res != "/test/0/0" {
		t.Errorf("test 3 faile\nexpectation: %v\nresult: %v", "/test/0/0", res)
		t.Fail()
	}

	res = parseDynamicPattern("/test/:id/:name/:age")
	if res != "/test/0/0/0" {
		t.Errorf("test 4 faile\nexpectation: %v\nresult: %v", "/test/0/0/0", res)
		t.Fail()
	}

	res = parseDynamicPattern("/file/*path")
	if res != "/file/1" {
		t.Errorf("test 5 faile\nexpectation: %v\nresult: %v", "/file/1", res)
		t.Fail()
	}

	res = parseDynamicPattern("/file/upload/:id/*path")
	if res != "/file/upload/0/1" {
		t.Errorf("test 6 faile\nexpectation: %v\nresult: %v", "/file/upload/0/1", res)
		t.Fail()
	}
}
