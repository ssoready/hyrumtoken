package hyrumtoken_test

import (
	"crypto/rand"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/ssoready/hyrumtoken"
)

// testkey is a randomized key for testing. Do not use it in production.
var testkey = [32]byte{24, 12, 15, 90, 143, 133, 171, 28, 34, 75, 185, 194, 102, 93, 165, 183, 235, 96, 135, 135, 165, 1, 129, 91, 32, 7, 139, 135, 130, 2, 241, 168}

func TestEncoder(t *testing.T) {
	type data struct {
		Foo string
		Bar string
	}

	in := data{
		Foo: "foo",
		Bar: "bar",
	}

	encoded := hyrumtoken.Marshal(&testkey, in)

	var out data
	err := hyrumtoken.Unmarshal(&testkey, encoded, &out)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip failure")
	}
}

func TestEncoder_Unmarshal_empty(t *testing.T) {
	data := 123
	if err := hyrumtoken.Unmarshal(&testkey, "", &data); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if data != 123 {
		t.Fatalf("data unexpectedly modified: %d", data)
	}
}

func TestEncoder_Marshal(t *testing.T) {
	// test known produced values using fixed, zero rand and secret
	r := rand.Reader
	rand.Reader = zeroReader{}
	defer func() {
		rand.Reader = r
	}()

	token := hyrumtoken.Marshal(&testkey, 123)

	if token != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAULRUMRVA4GIqe5Y8N_z8B4J7hw==" {
		t.Fatalf("encoding regression, got: %q", token)
	}
}

func TestEncoder_Unmarshal(t *testing.T) {
	// inverse of TestEncoder_Marshal
	r := rand.Reader
	rand.Reader = zeroReader{}
	defer func() {
		rand.Reader = r
	}()

	var data int
	if err := hyrumtoken.Unmarshal(&testkey, "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAULRUMRVA4GIqe5Y8N_z8B4J7hw==", &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if data != 123 {
		t.Fatalf("unmarshal regression, got: %d", data)
	}
}

func TestEncoder_Unmarshal_incorrect_token_size(t *testing.T) {
	n := [23]byte{}
	var data int
	if err := hyrumtoken.Unmarshal(&testkey, base64.URLEncoding.EncodeToString(n[:]), &data); err == nil {
		t.Fatalf("expected get error")
	}
}

func TestEncoder_Unmarshal_incorrect_ecnrypted_payload(t *testing.T) {
	n := [100]byte{}
	var data int
	if err := hyrumtoken.Unmarshal(&testkey, base64.URLEncoding.EncodeToString(n[:]), &data); err == nil {
		t.Fatalf("expected get error")
	}
}

func TestEncoder_Unmarshal_incorrect_base64(t *testing.T) {
	var data int
	if err := hyrumtoken.Unmarshal(&testkey, "%=<>", &data); err == nil {
		t.Fatalf("expected get error")
	}
}

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := 0; i < len(p); i++ {
		p[i] = 0
	}
	return len(p), nil
}
