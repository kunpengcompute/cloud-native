package basecache

import "testing"

func TestBaseCacheImagePolicyRequiresDigestByDefault(t *testing.T) {
	digest := "docker.io/library/debian@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	eligible, err := ImageEligible(digest, false)
	if err != nil {
		t.Fatal(err)
	}
	if !eligible {
		t.Fatalf("ImageEligible(%q) = false, want true", digest)
	}

	eligible, err = ImageEligible("docker.io/library/debian:bookworm-slim", false)
	if err != nil {
		t.Fatal(err)
	}
	if eligible {
		t.Fatal("mutable tag is eligible without explicit opt-in")
	}
}

func TestBaseCacheImagePolicyAllowsMutableTagWhenExplicit(t *testing.T) {
	eligible, err := ImageEligible("docker.io/library/debian:bookworm-slim", true)
	if err != nil {
		t.Fatal(err)
	}
	if !eligible {
		t.Fatal("mutable tag is not eligible after explicit opt-in")
	}
}

func TestBaseCacheImagePolicyRejectsInvalidReference(t *testing.T) {
	if _, err := ImageEligible("not a valid image@@", true); err == nil {
		t.Fatal("ImageEligible() error = nil, want invalid reference error")
	}
}
