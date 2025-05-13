package main

import "strings"


type userOs struct {
	u  string
	os []string
}

var (
	amazon   = userOs{"ec2-user", []string{"amazon", "amzn"}}
	ubuntu   = userOs{"ubuntu", []string{"ubuntu"}}
	debian   = userOs{"admin", []string{"debian"}}
	centos   = userOs{"centos", []string{"centos"}}
	defaultU = userOs{"ec2-user", []string{}}
)

func getEC2user(imageDescription string) string {
	for _, uos := range []userOs{
		amazon,
		ubuntu,
		debian,
		centos,
	} {
		if containsAny(imageDescription, uos.os...) {
			return uos.u
		}
	}
	return defaultU.u
}

func containsAny(s string, substr ...string) bool {
	for _, p := range substr {
		if strings.Contains(strings.ToLower(s), strings.ToLower(p)) {
			return true
		}
	}
	return false
}