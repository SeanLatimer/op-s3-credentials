package awsprocess

import (
	"bytes"
	"testing"
)

func TestWriteLongTerm(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := `{"Version":1,"AccessKeyId":"AKIAIOSFODNN7EXAMPLE","SecretAccessKey":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}` + "\n"
	if buf.String() != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", buf.String(), want)
	}
}

func TestWriteSessionToken(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, "AKIA1", "secret1", "token1")
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := `{"Version":1,"AccessKeyId":"AKIA1","SecretAccessKey":"secret1","SessionToken":"token1"}` + "\n"
	if buf.String() != want {
		t.Errorf("output mismatch\n got: %q\nwant: %q", buf.String(), want)
	}
}
