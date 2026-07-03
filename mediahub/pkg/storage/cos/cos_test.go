package cos

import "testing"

func TestGetContentTypeUsesImageWhitelist(t *testing.T) {
	storage := &cosStorage{}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "jpg maps to jpeg mime",
			path: "/public/avatar.jpg",
			want: "image/jpeg",
		},
		{
			name: "uppercase extension still maps to jpeg mime",
			path: "/public/avatar.JPG",
			want: "image/jpeg",
		},
		{
			name: "webp keeps webp mime",
			path: "/public/banner.webp",
			want: "image/webp",
		},
		{
			name: "unknown extension falls back to octet stream",
			path: "/public/malicious.svg",
			want: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storage.getContentType(tt.path); got != tt.want {
				t.Fatalf("content type = %q, want %q", got, tt.want)
			}
		})
	}
}
