package service

import (
	"errors"
	"testing"
)

func TestValidateACLRequestRejectsPathOutsideStorageRoots(t *testing.T) {
	request := ACLRequest{Path: "/etc", SubjectType: "user", Subject: "user001", Permission: "rw"}
	err := ValidateACLRequest(request, []string{"/data/home", "/data/share"})
	if !errors.Is(err, ErrPathOutsideRoots) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateACLRequestAcceptsConfiguredRoot(t *testing.T) {
	request := ACLRequest{Path: "/data/home/user001/share", SubjectType: "user", Subject: "user002", Permission: "r"}
	if err := ValidateACLRequest(request, []string{"/data/home"}); err != nil {
		t.Fatal(err)
	}
}
