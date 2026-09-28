package dfman

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"testing"
)

func TestReleaseRedirect(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{"https://github.com/kopiro/dfman/releases/tag/v1.2.3", true},
		{"https://example.com/kopiro/dfman/releases/tag/v1.2.3", false},
		{"https://github.com/kopiro/dfman/releases/latest", false},
		{"https://github.com/kopiro/dfman/releases/tag/v1.2.3/evil", false},
	} {
		u, _ := url.Parse(tc.raw)
		r, err := releaseFromURL(u)
		if (err == nil) != tc.valid {
			t.Fatal(tc, err)
		}
		if tc.valid && r.Tag != "v1.2.3" {
			t.Fatal(r)
		}
	}
}
func TestInstallerVerification(t *testing.T) {
	data := []byte("installer")
	name := "dfman_v1.2.3_darwin_arm64.pkg"
	manifest := []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(data), name))
	if err := verifyInstaller(data, manifest, name); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, append(manifest, manifest...), []byte("bad  " + name)} {
		if verifyInstaller(data, bad, name) == nil {
			t.Fatal("invalid manifest accepted")
		}
	}
	if verifyInstaller([]byte("tampered"), manifest, name) == nil {
		t.Fatal("tampering accepted")
	}
	for platform, suffix := range map[string]string{"darwin": ".pkg", "linux": ".deb", "windows": "_setup.exe"} {
		name, err := installerName("v1.2.3", platform, "arm64")
		if err != nil || name != "dfman_v1.2.3_"+platform+"_arm64"+suffix {
			t.Fatal(name, err)
		}
	}
}
