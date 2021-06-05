package util

import (
	"testing"
)

func TestLogMatcherPositive(t *testing.T) {
	line := "05-Jun-2021 07:24:47.780 queries: info: client @0xadfc0030 192.168.0.123#59542 (bitnebula.com): query: bitnebula.com IN A + (192.168.0.456)"
	matcher := NewLogMatcher()
	info := matcher.ExtractInfo(line)

	if !info.Matched {
		t.Fatalf("No match detected in known-good string with default pattern")
	}

	if info.QueryClient != "192.168.0.123" {
		t.Fatalf(`Expected client of 192.168.0.123 but got '%s'`, info.QueryClient)
	}
	if info.QueryName != "bitnebula.com" {
		t.Fatalf(`Expected target name of bitnebula.com but got '%s'`, info.QueryName)
	}
	if info.QueryType != "A" {
		t.Fatalf(`Expected query type of A but got '%s'`, info.QueryType)
	}
}
