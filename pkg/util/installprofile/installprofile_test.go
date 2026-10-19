package installprofile

import "testing"

func TestSignVerify(t *testing.T) {
	body := []byte(`{"deferLonghornInstall":true}`)
	h := Sign("tok", "n1", body)
	if !Verify("tok", "n1", body, h) {
		t.Fatal("valid signature rejected")
	}
	if Verify("other", "n1", body, h) || Verify("tok", "n2", body, h) || Verify("tok", "n1", []byte("{}"), h) || Verify("tok", "n1", body, "") {
		t.Fatal("invalid signature accepted")
	}
}
