package config

import "testing"

// Note: config env var names were namespaced (AUTH_*, NOTE_FLOW_*), but APIPrefix behavior is unchanged.
// These tests intentionally cover only HTTPServer.APIPrefix() formatting logic.

func TestHTTPServer_APIPrefix_DefaultValues(t *testing.T) {
	h := HTTPServer{
		Prefix:     "gateway",
		AppVersion: "v1",
	}

	got := h.APIPrefix()
	want := "/gateway/api/v1"
	if got != want {
		t.Fatalf("APIPrefix() = %q, want %q", got, want)
	}
}

func TestHTTPServer_APIPrefix_CustomValues(t *testing.T) {
	h := HTTPServer{
		Prefix:     "my-service",
		AppVersion: "v2",
	}

	got := h.APIPrefix()
	want := "/my-service/api/v2"
	if got != want {
		t.Fatalf("APIPrefix() = %q, want %q", got, want)
	}
}

func TestHTTPServer_APIPrefix_EmptyParts(t *testing.T) {
	t.Run("empty prefix", func(t *testing.T) {
		h := HTTPServer{
			Prefix:     "",
			AppVersion: "v1",
		}
		got := h.APIPrefix()
		want := "//api/v1"
		if got != want {
			t.Fatalf("APIPrefix() = %q, want %q", got, want)
		}
	})

	t.Run("empty version", func(t *testing.T) {
		h := HTTPServer{
			Prefix:     "gateway",
			AppVersion: "",
		}
		got := h.APIPrefix()
		want := "/gateway/api/"
		if got != want {
			t.Fatalf("APIPrefix() = %q, want %q", got, want)
		}
	})
}
